package main

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
)

type cachedValue struct {
	value any
	until time.Time
}
type item struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	State   string `json:"state"`
	Detail  string `json:"detail"`
	Project string `json:"project"`
	Service string `json:"service"`
	Version string `json:"version"`
}
type capability struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Destructive bool   `json:"destructive"`
	Input       string `json:"input"`
}

var actionIDs = map[string][]string{
	"containers": {"start", "stop", "restart", "pause", "unpause", "kill", "remove", "remove-volumes", "remove-force", "remove-force-volumes", "exec", "attach", "logs-follow", "open-browser", "prune"},
	"services":   {"start", "stop", "restart", "pause", "unpause", "up", "recreate", "rebuild", "remove", "remove-volumes", "exec", "attach", "logs-follow", "open-browser"},
	"projects":   {"up", "up-attached", "down", "down-volumes", "down-images", "down-volumes-images", "start", "stop", "restart", "pause", "unpause", "pull", "build", "rebuild", "recreate", "logs-follow"},
	"images":     {"remove", "remove-no-prune", "remove-force", "remove-force-no-prune", "prune", "prune-all"},
	"volumes":    {"remove", "remove-force", "prune"},
	"networks":   {"remove", "prune"},
	"commands":   {"run"},
	"host":       {},
}
var viewIDs = map[string][]string{
	"containers": {"logs", "stats", "env", "inspect", "top"}, "services": {"logs", "stats", "env", "inspect", "top"},
	"projects": {"logs", "config"}, "images": {"inspect", "history"}, "volumes": {"inspect"}, "networks": {"inspect"},
	"commands": {"config"}, "host": {"info", "events", "disk", "config"},
}

func (b *backend) capabilities(kind string) []capability {
	result := []capability{}
	for _, id := range actionIDs[kind] {
		input := ""
		if id == "exec" {
			input = "argv"
		}
		result = append(result, capability{ID: id, Label: strings.ReplaceAll(id, "-", " "), Destructive: strings.Contains(id, "remove") || strings.HasPrefix(id, "prune") || strings.HasPrefix(id, "down") || id == "kill", Input: input})
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, command := range b.config.Commands {
		if command.Scope == kind {
			result = append(result, capability{ID: "custom:" + command.ID, Label: command.Label, Destructive: true})
		}
	}
	return result
}

func (b *backend) cached(key string, ttl time.Duration, fetch func() (any, error)) (any, error) {
	b.mu.Lock()
	entry, ok := b.cache[key]
	generation := b.cacheGeneration
	b.mu.Unlock()
	if ok && time.Now().Before(entry.until) {
		return entry.value, nil
	}
	value, err := fetch()
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	if b.cacheGeneration != generation {
		b.mu.Unlock()
		return value, nil
	}
	if b.cache == nil {
		b.cache = map[string]cachedValue{}
	}
	b.cache[key] = cachedValue{value, time.Now().Add(ttl)}
	b.mu.Unlock()
	return value, nil
}
func (b *backend) invalidate() { b.mu.Lock(); b.cache = nil; b.cacheGeneration++; b.mu.Unlock() }

func (b *backend) containers(ctx context.Context) ([]container.Summary, error) {
	if b.demo {
		b.mu.Lock()
		defer b.mu.Unlock()
		result := []container.Summary{}
		for _, r := range b.rows {
			labels := map[string]string{}
			if r.Stack != "" {
				labels["com.docker.compose.project"] = r.Stack
				labels["com.docker.compose.service"] = strings.TrimPrefix(r.Name, r.Stack+"_")
			}
			status := r.State
			if r.State == "exited" {
				status = fmt.Sprintf("Exited (%d) 1 minute ago", r.ExitCode)
			}
			result = append(result, container.Summary{ID: r.ID, Names: []string{"/" + r.Name}, State: r.State, Status: status, Labels: labels, Image: "sample:latest", ImageID: "sha256:demo-image", Created: 1700000000})
		}
		return result, nil
	}
	value, err := b.cached("containers", 2*time.Second, func() (any, error) { return b.docker.Client.ContainerList(ctx, container.ListOptions{All: true}) })
	if err != nil {
		return nil, err
	}
	return value.([]container.Summary), nil
}

func containerName(c container.Summary) string {
	if len(c.Names) > 0 {
		return strings.TrimPrefix(c.Names[0], "/")
	}
	return c.ID
}

var exitPattern = regexp.MustCompile(`^Exited \((-?\d+)\)`)

func rowFromSummary(c container.Summary) containerRow {
	exit := -1
	if m := exitPattern.FindStringSubmatch(c.Status); len(m) > 1 {
		_, _ = fmt.Sscan(m[1], &exit)
	}
	ports := []string{}
	for _, p := range c.Ports {
		if p.PublicPort > 0 {
			ports = append(ports, fmt.Sprintf("%s:%d → %d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type))
		}
	}
	text := strings.Join(ports, ", ")
	if text == "" {
		text = "—"
	}
	return containerRow{ID: c.ID, Name: containerName(c), Stack: c.Labels["com.docker.compose.project"], State: c.State, ExitCode: exit, Ports: text, CPU: "—"}
}

func (b *backend) inventory(ctx context.Context, kind string) ([]item, error) {
	if _, ok := actionIDs[kind]; !ok {
		return nil, fmt.Errorf("unknown inventory kind %q", kind)
	}
	result := []item{}
	if kind == "containers" || kind == "services" || kind == "projects" {
		cs, err := b.containers(ctx)
		if err != nil {
			return nil, err
		}
		groups := map[string]item{}
		for _, c := range cs {
			p, s := c.Labels["com.docker.compose.project"], c.Labels["com.docker.compose.service"]
			i := item{ID: c.ID, Name: containerName(c), State: c.State, Detail: c.Image + " · " + c.Status, Project: p, Service: s, Version: fmt.Sprint(c.Created)}
			if kind == "containers" {
				result = append(result, i)
				continue
			}
			if p == "" {
				continue
			}
			i.ID, i.Name = p, p
			if kind == "services" {
				if s == "" {
					continue
				}
				i.ID, i.Name = p+"/"+s, s
			}
			old := groups[i.ID]
			i.Version = old.Version + c.ID + ","
			i.Detail = p
			groups[i.ID] = i
		}
		if kind == "projects" {
			b.mu.Lock()
			for _, p := range b.config.Projects {
				if _, ok := groups[p.ID]; !ok {
					groups[p.ID] = item{ID: p.ID, Name: p.ID, State: "configured", Detail: p.Path, Project: p.ID, Version: "configured"}
				}
			}
			b.mu.Unlock()
		}
		for _, i := range groups {
			parts := strings.Split(i.Version, ",")
			sort.Strings(parts)
			i.Version = strings.Join(parts, ",")
			result = append(result, i)
		}
	} else if kind == "commands" {
		b.mu.Lock()
		for _, c := range b.config.Commands {
			if c.Scope != "commands" {
				continue
			}
			result = append(result, item{ID: c.ID, Name: c.Label, State: c.Scope, Detail: "Configured command · reveal in details", Version: configDigest(c)})
		}
		b.mu.Unlock()
	} else if kind == "host" {
		result = append(result, item{ID: "local", Name: "Local Docker Engine", State: b.mode(), Detail: "Unix socket only", Version: "local"})
	} else if b.demo {
		b.mu.Lock()
		defer b.mu.Unlock()
		fixtures := map[string][]item{
			"images":   {{ID: "sha256:demo-image", Name: "sample:latest", State: "tagged", Detail: "64.0 MB", Version: "demo"}, {ID: "sha256:demo-dangling", Name: "<none>", State: "dangling", Detail: "8.0 MB", Version: "demo"}},
			"volumes":  {{ID: "demo-data", Name: "demo-data", State: "local", Detail: "sample volume", Version: "demo"}},
			"networks": {{ID: "demo-network", Name: "demo-network", State: "bridge", Detail: "sample network", Version: "demo"}},
		}
		for _, i := range fixtures[kind] {
			if !b.demoRemoved[kind+":"+i.ID] {
				result = append(result, i)
			}
		}
	} else {
		switch kind {
		case "images":
			images, err := b.docker.Client.ImageList(ctx, image.ListOptions{All: true})
			if err != nil {
				return nil, err
			}
			for _, i := range images {
				name := strings.Join(i.RepoTags, ", ")
				state := "tagged"
				if name == "" {
					name = "<none>"
					state = "dangling"
				}
				result = append(result, item{ID: i.ID, Name: name, State: state, Detail: humanBytes(i.Size), Version: fmt.Sprint(i.Created)})
			}
		case "volumes":
			volumes, err := b.docker.Client.VolumeList(ctx, volume.ListOptions{})
			if err != nil {
				return nil, err
			}
			for _, v := range volumes.Volumes {
				result = append(result, item{ID: v.Name, Name: v.Name, State: v.Driver, Detail: v.Scope, Version: v.CreatedAt})
			}
		case "networks":
			networks, err := b.docker.Client.NetworkList(ctx, network.ListOptions{})
			if err != nil {
				return nil, err
			}
			for _, n := range networks {
				result = append(result, item{ID: n.ID, Name: n.Name, State: n.Driver, Detail: n.Scope, Version: n.Created.String()})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (b *backend) pruneCandidates(ctx context.Context, kind, op string) ([]item, error) {
	items, err := b.inventory(ctx, kind)
	if err != nil {
		return nil, err
	}
	result := []item{}
	used := map[string]bool{}
	cs, err := b.containers(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range cs {
		used[c.ImageID] = true
		for _, m := range c.Mounts {
			if m.Type == "volume" {
				used[m.Name] = true
			}
		}
	}
	for _, i := range items {
		switch kind {
		case "containers":
			if i.State == "exited" || i.State == "dead" {
				result = append(result, i)
			}
		case "images":
			if !used[i.ID] && (op == "prune-all" || i.State == "dangling") {
				result = append(result, i)
			}
		case "volumes":
			if !used[i.ID] { // Docker's volume prune defaults to anonymous volumes in recent APIs.
				if b.demo {
					result = append(result, i)
				} else {
					v, e := b.docker.Client.VolumeInspect(ctx, i.ID)
					if e != nil {
						return nil, e
					}
					if _, anonymous := v.Labels["com.docker.volume.anonymous"]; anonymous {
						result = append(result, i)
					}
				}
			}
		case "networks":
			if i.Name == "bridge" || i.Name == "host" || i.Name == "none" {
				continue
			}
			if b.demo {
				result = append(result, i)
			} else {
				n, e := b.docker.Client.NetworkInspect(ctx, i.ID, network.InspectOptions{})
				if e != nil {
					return nil, e
				}
				if len(n.Containers) == 0 {
					result = append(result, i)
				}
			}
		}
	}
	return result, nil
}

func (b *backend) dashboard(ctx context.Context) (object, error) {
	cs, err := b.containers(ctx)
	if err != nil {
		return nil, fmt.Errorf("Docker unavailable: %w", err)
	}
	rows := make([]containerRow, 0, len(cs))
	b.mu.Lock()
	cpus := b.liveCPU
	consumers := b.liveConsumers
	eventRows := b.liveEvents
	b.mu.Unlock()
	for _, c := range cs {
		row := rowFromSummary(c)
		if cpu, ok := cpus[c.ID]; ok {
			row.CPU = cpu
		}
		rows = append(rows, row)
	}
	dash := summarize(rows, b.mode())
	if consumers != nil {
		dash["consumers"] = consumers
	}
	if eventRows != nil {
		dash["events"] = eventRows
	}
	if b.demo {
		dash["events"] = []object{{"id": "demo-event", "name": "sample", "what": "demo inventory ready", "tone": "ok", "at": "12:00"}}
		dash["host"] = []object{{"label": "Memory (sample)", "valueText": "2.0 GB / 8.0 GB", "percent": 25, "warn": false}}
		return dash, nil
	}
	warnings := []string{}
	totals := dash["totals"].(object)
	value, e := b.cached("usage", 30*time.Second, func() (any, error) { return b.docker.Client.DiskUsage(ctx, types.DiskUsageOptions{}) })
	if e != nil {
		warnings = append(warnings, "Docker disk usage unavailable: "+e.Error())
	} else {
		u := value.(types.DiskUsage)
		totals["images"], totals["volumes"] = len(u.Images), len(u.Volumes)
		var size int64
		ui, uv := 0, 0
		for _, i := range u.Images {
			size += i.Size
			if i.Containers == 0 {
				ui++
			}
		}
		for _, v := range u.Volumes {
			if v.UsageData != nil && v.UsageData.RefCount == 0 {
				uv++
			}
		}
		totals["imagesSize"], totals["imagesUnused"], totals["volumesUnused"] = humanBytes(size), ui, uv
	}
	nets, e := b.cached("networks", 30*time.Second, func() (any, error) { return b.docker.Client.NetworkList(ctx, network.ListOptions{}) })
	if e == nil {
		totals["networks"], totals["networksDetail"] = len(nets.([]network.Summary)), "Docker networks"
	} else {
		warnings = append(warnings, e.Error())
	}
	dash["host"] = hostMeters()
	dash["warnings"] = warnings
	return dash, nil
}
