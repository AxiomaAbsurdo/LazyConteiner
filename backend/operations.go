package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/jesseduffield/lazydocker/pkg/commands"
)

func (b *backend) projectProof(p projectConfig) (string, error) {
	if b.demo {
		return configDigest(p), nil
	}
	info, err := os.Stat(p.Path)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("Compose project directory unavailable: %s", p.Path)
	}
	files := p.Files
	if len(files) == 0 {
		for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"} {
			path := filepath.Join(p.Path, name)
			if _, err := os.Stat(path); err == nil {
				files = append(files, path)
				break
			}
		}
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no Compose file found in %s; configure files in Settings", p.Path)
	}
	proof := []string{configDigest(p)}
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 5*1024*1024 {
			return "", fmt.Errorf("Compose file unavailable or larger than 5 MiB: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		proof = append(proof, path, configDigest(data))
	}
	// Compose substitutes .env at execution; include it in the confirmation proof.
	if data, err := os.ReadFile(filepath.Join(p.Path, ".env")); err == nil {
		proof = append(proof, configDigest(data))
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return configDigest(proof), nil
}

type resolvedCommand struct {
	Cwd          string
	Argv         []string
	Terminal     bool
	TerminalArgv []string
}

func (b *backend) resolveCommand(ctx context.Context, r actionRequest, i item) (*resolvedCommand, error) {
	command := &resolvedCommand{}
	b.mu.Lock()
	command.TerminalArgv = append([]string{}, b.config.Terminal...)
	b.mu.Unlock()
	if len(command.TerminalArgv) == 0 {
		command.TerminalArgv = []string{"x-terminal-emulator", "-e"}
	}
	b.mu.Lock()
	override := append([]string{}, b.config.CommandTemplates[r.Kind+"."+r.Op]...)
	b.mu.Unlock()
	if len(override) > 0 {
		c := customCommand{Argv: override, Terminal: isTerminal(r.Op)}
		if i.Project != "" {
			p, e := b.project(ctx, i.Project)
			if e != nil {
				return nil, e
			}
			c.Cwd = p.Path
		}
		c, e := b.renderCustom(c, r, i)
		if e != nil {
			return nil, e
		}
		command.Cwd, command.Argv, command.Terminal = c.Cwd, c.Argv, c.Terminal
		return command, nil
	}
	if r.Kind == "commands" || strings.HasPrefix(r.Op, "custom:") {
		c, e := b.customFor(r, i)
		if e != nil {
			return nil, e
		}
		command.Cwd, command.Argv, command.Terminal = c.Cwd, c.Argv, c.Terminal
		return command, nil
	}
	if r.Kind == "projects" || r.Kind == "services" {
		if r.Kind == "services" && (r.Op == "exec" || r.Op == "attach" || r.Op == "open-browser") {
			cs, e := b.selectedContainers(ctx, i)
			if e != nil {
				return nil, e
			}
			if len(cs) == 0 {
				return nil, fmt.Errorf("service has no container")
			}
			i.ID = cs[0].ID
			r.Kind = "containers"
		} else {
			p, e := b.project(ctx, i.Project)
			if e != nil {
				return nil, e
			}
			argv, e := b.composeArgs(p, r.Op, i.Service)
			if e != nil {
				return nil, e
			}
			command.Cwd, command.Argv, command.Terminal = p.Path, argv, isTerminal(r.Op)
			return command, nil
		}
	}
	if r.Kind != "containers" || !isTerminal(r.Op) {
		return nil, nil
	}
	command.Terminal = true
	switch r.Op {
	case "exec":
		command.Argv = append([]string{"docker", "exec", "-it", i.ID}, r.Args...)
	case "attach":
		command.Argv = []string{"docker", "attach", "--sig-proxy=false", i.ID}
	case "logs-follow":
		command.Argv = append([]string{"docker", "logs", "--follow"}, b.logFlags()...)
		command.Argv = append(command.Argv, i.ID)
	case "open-browser":
		command.Terminal = false
		port := uint16(0)
		if b.demo {
			port = 8000
		} else {
			cs, e := b.containers(ctx)
			if e != nil {
				return nil, e
			}
			for _, c := range cs {
				if c.ID == i.ID {
					for _, p := range c.Ports {
						if p.PublicPort > 0 && p.Type == "tcp" {
							port = p.PublicPort
							break
						}
					}
				}
			}
		}
		if port == 0 {
			return nil, fmt.Errorf("container has no published TCP port")
		}
		command.Argv = []string{"xdg-open", fmt.Sprintf("http://127.0.0.1:%d", port)}
	}
	return command, nil
}

// Docker's default image remove recursively prunes unused parents. Resolve that
// closure before confirmation, then issue only exact-ID no-prune removals.
func (b *backend) imageParentTargets(ctx context.Context, selected []item) ([]item, error) {
	if b.demo {
		return selected, nil
	}
	images, err := b.docker.Client.ImageList(ctx, image.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	cs, err := b.containers(ctx)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, c := range cs {
		used[c.ImageID] = true
	}
	byID := map[string]image.Summary{}
	children := map[string][]string{}
	included := map[string]bool{}
	for _, i := range images {
		byID[i.ID] = i
		if i.ParentID != "" {
			children[i.ParentID] = append(children[i.ParentID], i.ID)
		}
	}
	for _, i := range selected {
		included[i.ID] = true
	}
	result := append([]item{}, selected...)
	for n := 0; n < len(result); n++ {
		parent := byID[result[n].ID].ParentID
		if parent == "" || included[parent] || used[parent] {
			continue
		}
		eligible := true
		for _, child := range children[parent] {
			if !included[child] {
				eligible = false
			}
		}
		if !eligible {
			continue
		}
		p, ok := byID[parent]
		if !ok {
			continue
		}
		if len(p.RepoTags) > 0 || len(p.RepoDigests) > 0 {
			continue
		}
		included[parent] = true
		name := strings.Join(p.RepoTags, ", ")
		if name == "" {
			name = "Unused parent image"
		}
		result = append(result, item{ID: p.ID, Name: name, Version: fmt.Sprint(p.Created)})
	}
	depth := func(id string) int {
		n := 0
		seen := map[string]bool{}
		for id != "" && !seen[id] {
			seen[id] = true
			n++
			id = byID[id].ParentID
		}
		return n
	}
	sort.SliceStable(result, func(i, j int) bool { return depth(result[i].ID) > depth(result[j].ID) })
	return result, nil
}
func (b *backend) customFor(r actionRequest, i item) (customCommand, error) {
	id := strings.TrimPrefix(r.Op, "custom:")
	if r.Kind == "commands" {
		id = i.ID
	}
	b.mu.Lock()
	configured := append([]customCommand{}, b.config.Commands...)
	b.mu.Unlock()
	for _, c := range configured {
		if c.ID == id {
			if r.Kind != "commands" && c.Scope != r.Kind {
				return c, fmt.Errorf("custom command scope mismatch")
			}
			return b.renderCustom(c, r, i)
		}
	}
	return customCommand{}, fmt.Errorf("custom command no longer exists")
}
func (b *backend) renderCustom(c customCommand, r actionRequest, i item) (customCommand, error) {
	if len(c.ServiceNames) > 0 && r.Kind == "services" {
		allowed := false
		for _, name := range c.ServiceNames {
			if name == i.Service {
				allowed = true
			}
		}
		if !allowed {
			return c, fmt.Errorf("custom command is not enabled for service %s", i.Service)
		}
	}
	args := []string{}
	replacer := strings.NewReplacer("{id}", i.ID, "{name}", i.Name, "{project}", i.Project, "{service}", i.Service)
	data := object{"Container": object{"ID": i.ID, "Name": i.Name, "ProjectName": i.Project, "ServiceName": i.Service}, "Service": object{"ID": i.ID, "Name": i.Service, "ProjectName": i.Project}, "Image": object{"ID": i.ID, "Name": i.Name}, "Volume": object{"Name": i.Name}, "Network": object{"Name": i.Name}, "Project": object{"Name": i.Project}, "DockerCompose": "{compose}"}
	for _, arg := range c.Argv {
		if arg == "{compose}" || strings.TrimSpace(arg) == "{{ .DockerCompose }}" {
			p, err := b.project(context.Background(), i.Project)
			if err != nil {
				return c, err
			}
			base, err := composeArgv(p, "config", "")
			if err != nil {
				return c, err
			}
			args = append(args, base[:len(base)-1]...)
			continue
		}
		parsed, err := template.New("argument").Option("missingkey=error").Parse(arg)
		if err != nil {
			return c, err
		}
		var out bytes.Buffer
		if err = parsed.Execute(&out, data); err != nil {
			return c, err
		}
		if strings.Contains(out.String(), "{compose}") {
			return c, fmt.Errorf("DockerCompose must occupy one complete argv entry")
		}
		args = append(args, replacer.Replace(out.String()))
	}
	c.Argv = args
	return c, nil
}
func (b *backend) execute(ctx context.Context, p actionPlan) (int, string, error) {
	defer b.invalidate()
	count := 0
	text := []string{}
	for _, i := range p.Targets {
		var message string
		var err error
		if b.demo && p.Request.Kind != "commands" && !strings.HasPrefix(p.Request.Op, "custom:") {
			message, err = b.demoOperation(p.Request, i)
		} else if command, ok := p.Commands[i.ID]; ok {
			message, err = b.runResolved(ctx, command.Cwd, command.Argv, command.Terminal, command.TerminalArgv)
		} else {
			message, err = b.executeOne(ctx, p.Request, i)
		}
		if err != nil {
			return count, strings.Join(text, "\n"), fmt.Errorf("%s: %w (%d of %d completed)", i.Name, err, count, len(p.Targets))
		}
		count++
		if message == "" {
			message = p.Request.Op + " " + i.Name + ": completed"
		}
		text = append(text, message)
	}
	return count, strings.Join(text, "\n"), nil
}
func (b *backend) executeOne(ctx context.Context, r actionRequest, i item) (string, error) {
	if r.Kind == "commands" || strings.HasPrefix(r.Op, "custom:") {
		c, err := b.customFor(r, i)
		if err != nil {
			return "", err
		}
		return b.run(ctx, c.Cwd, c.Argv, c.Terminal)
	}
	if b.demo {
		return b.demoOperation(r, i)
	}
	op := r.Op
	if strings.HasPrefix(op, "prune") {
		op = "remove"
	}
	if r.Kind == "projects" || r.Kind == "services" {
		if r.Kind == "services" && (op == "exec" || op == "attach" || op == "open-browser") {
			cs, err := b.selectedContainers(ctx, i)
			if err != nil {
				return "", err
			}
			if len(cs) == 0 {
				return "", fmt.Errorf("service has no existing container")
			}
			i.ID = cs[0].ID
			r.Kind = "containers"
		} else {
			p, err := b.project(ctx, i.Project)
			if err != nil {
				return "", err
			}
			argv, err := b.composeArgs(p, op, i.Service)
			if err != nil {
				return "", err
			}
			return b.run(ctx, p.Path, argv, isTerminal(op))
		}
	}
	switch r.Kind {
	case "containers":
		c := &commands.Container{ID: i.ID, Name: i.Name, Client: b.docker.Client, Log: b.docker.Log}
		switch op {
		case "start":
			return "", c.Start()
		case "stop":
			return "", c.Stop()
		case "restart":
			return "", c.Restart()
		case "pause":
			return "", c.Pause()
		case "unpause":
			return "", c.Unpause()
		case "kill":
			return "", b.docker.Client.ContainerKill(ctx, i.ID, "SIGKILL")
		case "remove", "remove-volumes", "remove-force", "remove-force-volumes":
			return "", c.Remove(container.RemoveOptions{Force: strings.Contains(op, "force"), RemoveVolumes: strings.Contains(op, "volumes")})
		case "exec":
			return b.run(ctx, "", append([]string{"docker", "exec", "-it", i.ID}, r.Args...), true)
		case "attach":
			return b.run(ctx, "", []string{"docker", "attach", "--sig-proxy=false", i.ID}, true)
		case "logs-follow":
			return b.run(ctx, "", []string{"docker", "logs", "--follow", "--tail", "300", i.ID}, true)
		case "open-browser":
			cs, err := b.containers(ctx)
			if err != nil {
				return "", err
			}
			for _, c := range cs {
				if c.ID != i.ID {
					continue
				}
				for _, p := range c.Ports {
					if p.PublicPort != 0 && p.Type == "tcp" {
						return b.run(ctx, "", []string{"xdg-open", fmt.Sprintf("http://127.0.0.1:%d", p.PublicPort)}, false)
					}
				}
			}
			return "", fmt.Errorf("container has no published TCP port")
		}
	case "images":
		c := &commands.Image{ID: i.ID, Client: b.docker.Client}
		// Prune freezes exact image IDs. Never recursively delete unpreviewed parents.
		return "", c.Remove(image.RemoveOptions{Force: strings.Contains(op, "force"), PruneChildren: false})
	case "volumes":
		return "", (&commands.Volume{Name: i.ID, Client: b.docker.Client}).Remove(op == "remove-force")
	case "networks":
		return "", (&commands.Network{Name: i.ID, Client: b.docker.Client}).Remove()
	}
	return "", fmt.Errorf("unsupported operation")
}

func (b *backend) demoOperation(r actionRequest, i item) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.demoRemoved == nil {
		b.demoRemoved = map[string]bool{}
	}
	if isTerminal(r.Op) {
		return "Demo: native terminal action simulated; no terminal or process launched.", nil
	}
	if r.Kind == "containers" || r.Kind == "services" || r.Kind == "projects" {
		rows := []containerRow{}
		matched := false
		for _, row := range b.rows {
			match := row.ID == i.ID || (i.Project != "" && row.Stack == i.Project && (r.Kind == "projects" || strings.TrimPrefix(row.Name, row.Stack+"_") == i.Service))
			if match {
				matched = true
				switch r.Op {
				case "remove", "remove-volumes", "remove-force", "remove-force-volumes", "prune", "down", "down-volumes", "down-images", "down-volumes-images":
					continue
				case "start", "up", "restart", "unpause", "recreate", "rebuild":
					row.State, row.ExitCode = "running", -1
				case "stop":
					row.State, row.ExitCode = "exited", 0
				case "kill":
					row.State, row.ExitCode = "exited", 137
				case "pause":
					row.State = "paused"
				}
			}
			rows = append(rows, row)
		}
		if !matched && r.Kind == "projects" && (r.Op == "up" || r.Op == "rebuild" || r.Op == "recreate") {
			rows = append(rows, containerRow{ID: "demo-" + i.ID, Name: i.ID + "_app", Stack: i.ID, State: "running", ExitCode: -1, CPU: "—", Ports: "—"})
		}
		b.rows = rows
	} else if strings.HasPrefix(r.Op, "remove") || strings.HasPrefix(r.Op, "prune") {
		b.demoRemoved[r.Kind+":"+i.ID] = true
	}
	return "Demo: " + r.Op + " " + i.Name + " completed in isolated memory.", nil
}
