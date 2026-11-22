package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
)

type projectConfig struct {
	ID    string   `json:"id"`
	Path  string   `json:"path"`
	Files []string `json:"files"`
}
type customCommand struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Scope        string   `json:"scope"`
	Argv         []string `json:"argv"`
	Cwd          string   `json:"cwd"`
	Terminal     bool     `json:"terminal"`
	ServiceNames []string `json:"serviceNames,omitempty"`
}
type appConfig struct {
	Projects         []projectConfig     `json:"projects"`
	Commands         []customCommand     `json:"commands"`
	Terminal         []string            `json:"terminal"`
	Stats            []statGraph         `json:"stats"`
	StatsMaxDuration string              `json:"statsMaxDuration"`
	Logs             logConfig           `json:"logs"`
	CommandTemplates map[string][]string `json:"commandTemplates"`
}
type logConfig struct {
	Since      string `json:"since"`
	Tail       int    `json:"tail"`
	Timestamps bool   `json:"timestamps"`
}
type statGraph struct {
	Label    string  `json:"label"`
	StatPath string  `json:"statPath"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	AutoMin  bool    `json:"autoMin,omitempty"`
	AutoMax  bool    `json:"autoMax,omitempty"`
}

func (g *statGraph) UnmarshalJSON(data []byte) error {
	type plain statGraph
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	var bounds struct {
		Min *float64 `json:"min"`
		Max *float64 `json:"max"`
	}
	if err := json.Unmarshal(data, &bounds); err != nil {
		return err
	}
	*g = statGraph(p)
	if bounds.Min == nil {
		g.AutoMin = true
	}
	if bounds.Max == nil {
		g.AutoMax = true
	}
	return nil
}

type commandRunner func(context.Context, string, []string, bool) (string, error)

var configID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func configDigest(v any) string {
	data, _ := json.Marshal(v)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func emptyConfig() appConfig {
	return appConfig{Projects: []projectConfig{}, Commands: []customCommand{}, Terminal: []string{"x-terminal-emulator", "-e"}, Stats: []statGraph{{Label: "CPU", StatPath: "DerivedStats.CPUPercentage", Min: 0, Max: 100}, {Label: "Memory", StatPath: "DerivedStats.MemoryPercentage", Min: 0, Max: 100}}, StatsMaxDuration: "2m", Logs: logConfig{Since: "60m", Tail: 300, Timestamps: false}, CommandTemplates: map[string][]string{}}
}
func (b *backend) loadConfig() error {
	b.config = emptyConfig()
	b.configPath = os.Getenv("LAZYCONTAINER_CONFIG")
	if b.configPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		b.configPath = filepath.Join(dir, "lazyconteiner", "config.json")
	}
	data, err := os.ReadFile(b.configPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	config, err := parseConfig(data)
	if err != nil {
		return fmt.Errorf("config %s: %w", b.configPath, err)
	}
	b.config = config
	return nil
}
func parseConfig(data []byte) (appConfig, error) {
	result := emptyConfig()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return result, fmt.Errorf("config must contain one JSON object")
	}
	if len(result.Terminal) == 0 || len(result.Terminal) > 32 {
		return result, fmt.Errorf("terminal must be an argv array")
	}
	seen := map[string]bool{}
	duration, err := time.ParseDuration(result.StatsMaxDuration)
	if err != nil || duration <= 0 || duration > 24*time.Hour {
		return result, fmt.Errorf("statsMaxDuration must be between 1ns and 24h")
	}
	if result.Logs.Tail < 1 || result.Logs.Tail > 5000 {
		return result, fmt.Errorf("logs.tail must be between 1 and 5000")
	}
	if result.Logs.Since != "" {
		if _, err := time.ParseDuration(result.Logs.Since); err != nil {
			if _, err = time.Parse(time.RFC3339, result.Logs.Since); err != nil {
				return result, fmt.Errorf("logs.since must be a duration or RFC3339 timestamp")
			}
		}
	}
	for key, args := range result.CommandTemplates {
		parts := strings.SplitN(key, ".", 2)
		valid := false
		if len(parts) == 2 {
			for _, id := range actionIDs[parts[0]] {
				if id == parts[1] {
					valid = true
				}
			}
		}
		if !valid || len(args) == 0 || len(args) > 128 {
			return result, fmt.Errorf("commandTemplates keys must identify an action, e.g. services.restart")
		}
		for _, arg := range args {
			if strings.ContainsRune(arg, 0) || len(arg) > 16384 {
				return result, fmt.Errorf("invalid command template argv")
			}
		}
	}
	for _, g := range result.Stats {
		if g.Label == "" || g.StatPath == "" || (!g.AutoMin && !g.AutoMax && g.Max <= g.Min) {
			return result, fmt.Errorf("stats require label, statPath, and max greater than min")
		}
	}
	for _, p := range result.Projects {
		if !configID.MatchString(p.ID) || seen["p:"+p.ID] || !filepath.IsAbs(p.Path) {
			return result, fmt.Errorf("projects require unique IDs and absolute paths")
		}
		seen["p:"+p.ID] = true
		for _, f := range p.Files {
			if !filepath.IsAbs(f) {
				return result, fmt.Errorf("Compose files must use absolute paths")
			}
		}
	}
	for _, c := range result.Commands {
		if !configID.MatchString(c.ID) || seen["c:"+c.ID] || c.Label == "" || len(c.Argv) == 0 || len(c.Argv) > 128 {
			return result, fmt.Errorf("custom commands require unique IDs, labels and argv")
		}
		seen["c:"+c.ID] = true
		if _, ok := actionIDs[c.Scope]; !ok {
			return result, fmt.Errorf("unknown command scope %q", c.Scope)
		}
		if c.Cwd != "" && !filepath.IsAbs(c.Cwd) {
			return result, fmt.Errorf("custom command cwd must be absolute")
		}
		for _, name := range c.ServiceNames {
			if !configID.MatchString(name) {
				return result, fmt.Errorf("invalid custom command service filter")
			}
		}
		for _, arg := range c.Argv {
			if strings.ContainsRune(arg, 0) || len(arg) > 16384 {
				return result, fmt.Errorf("invalid command argument")
			}
		}
	}
	return result, nil
}
func (b *backend) saveConfig(data []byte) error {
	c, err := parseConfig(data)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.demo {
		if b.configPath == "" {
			return fmt.Errorf("configuration path not initialized")
		}
		if err = os.MkdirAll(filepath.Dir(b.configPath), 0700); err != nil {
			return err
		}
		f, e := os.CreateTemp(filepath.Dir(b.configPath), ".config-*")
		if e != nil {
			return e
		}
		name := f.Name()
		defer os.Remove(name)
		_, err = f.Write(data)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if err = os.Rename(name, b.configPath); err != nil {
			return err
		}
	}
	b.config = c
	b.plans = nil
	b.cache = nil
	b.cacheGeneration++
	return nil
}

func (b *backend) project(ctx context.Context, id string) (projectConfig, error) {
	b.mu.Lock()
	for _, p := range b.config.Projects {
		if p.ID == id {
			b.mu.Unlock()
			return p, nil
		}
	}
	b.mu.Unlock()
	if b.demo {
		return projectConfig{ID: id, Path: "/demo/" + id, Files: []string{"/demo/" + id + "/compose.yaml"}}, nil
	}
	cs, err := b.containers(ctx)
	if err != nil {
		return projectConfig{}, err
	}
	for _, c := range cs {
		if c.Labels["com.docker.compose.project"] != id {
			continue
		}
		p := projectConfig{ID: id, Path: c.Labels["com.docker.compose.project.working_dir"]}
		files := c.Labels["com.docker.compose.project.config_files"]
		if files != "" {
			p.Files = strings.Split(files, ",")
		}
		if !filepath.IsAbs(p.Path) {
			break
		}
		for _, f := range p.Files {
			if !filepath.IsAbs(f) {
				return projectConfig{}, fmt.Errorf("Compose project %s has a non-absolute config path; configure it in Settings", id)
			}
		}
		return p, nil
	}
	return projectConfig{}, fmt.Errorf("Compose project %s needs an absolute project path in Settings", id)
}
func composeArgv(p projectConfig, op, service string) ([]string, error) {
	if !configID.MatchString(p.ID) {
		return nil, fmt.Errorf("invalid Compose project name")
	}
	args := []string{"docker", "compose", "--project-name", p.ID, "--project-directory", p.Path}
	for _, file := range p.Files {
		args = append(args, "-f", file)
	}
	commands := map[string][]string{
		"up": {"up", "-d"}, "up-attached": {"up"}, "down": {"down"}, "down-volumes": {"down", "--volumes"}, "down-images": {"down", "--rmi", "all"}, "down-volumes-images": {"down", "--volumes", "--rmi", "all"},
		"start": {"start"}, "stop": {"stop"}, "restart": {"restart"}, "pause": {"pause"}, "unpause": {"unpause"}, "pull": {"pull"}, "build": {"build", "--parallel", "--force-rm"},
		"rebuild": {"up", "-d", "--build"}, "recreate": {"up", "-d", "--force-recreate"}, "remove": {"rm", "--stop", "--force"}, "remove-volumes": {"rm", "--stop", "--force", "--volumes"},
		"logs": {"logs", "--no-color", "--tail", "300"}, "logs-follow": {"logs", "--tail", "300", "--follow"}, "config": {"config"}, "top": {"top"},
	}
	command, ok := commands[op]
	if !ok {
		return nil, fmt.Errorf("unsupported Compose operation")
	}
	args = append(args, command...)
	if service != "" {
		if !configID.MatchString(service) {
			return nil, fmt.Errorf("invalid Compose service name")
		}
		args = append(args, "--", service)
	}
	return args, nil
}

type limitedOutput struct {
	bytes.Buffer
	truncated bool
}

func (b *backend) logSettings() logConfig {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.config.Logs
	if c.Tail <= 0 {
		c = emptyConfig().Logs
	}
	return c
}
func (b *backend) logFlags() []string {
	c := b.logSettings()
	args := []string{"--tail", fmt.Sprint(c.Tail)}
	if c.Since != "" {
		args = append(args, "--since", c.Since)
	}
	if c.Timestamps {
		args = append(args, "--timestamps")
	}
	return args
}
func (b *backend) composeArgs(p projectConfig, op, service string) ([]string, error) {
	if op != "logs" && op != "logs-follow" {
		return composeArgv(p, op, service)
	}
	base, err := composeArgv(p, "config", "")
	if err != nil {
		return nil, err
	}
	args := append(base[:len(base)-1], "logs", "--no-color")
	args = append(args, b.logFlags()...)
	if op == "logs-follow" {
		args = append(args, "--follow")
	}
	if service != "" {
		if !configID.MatchString(service) {
			return nil, fmt.Errorf("invalid service")
		}
		args = append(args, "--", service)
	}
	return args, nil
}

func (w *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 256*1024 - w.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = w.Buffer.Write(p)
	}
	if n > remaining {
		w.truncated = true
	}
	return n, nil
}
func (b *backend) run(ctx context.Context, cwd string, argv []string, terminal bool) (string, error) {
	b.mu.Lock()
	prefix := append([]string{}, b.config.Terminal...)
	b.mu.Unlock()
	return b.runResolved(ctx, cwd, argv, terminal, prefix)
}
func (b *backend) runResolved(ctx context.Context, cwd string, argv []string, terminal bool, prefix []string) (string, error) {
	if b.runner != nil {
		return b.runner(ctx, cwd, argv, terminal)
	}
	if len(argv) == 0 {
		return "", fmt.Errorf("empty command")
	}
	if b.demo {
		return "Demo command (no process launched): " + strings.Join(argv, " "), nil
	}
	if terminal {
		if len(prefix) == 0 {
			prefix = []string{"x-terminal-emulator", "-e"}
		}
		argv = append(prefix, argv...)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.WaitDelay = 2 * time.Second
	for _, e := range os.Environ() {
		key := strings.SplitN(e, "=", 2)[0]
		if key != "LAZYCONTAINER_TOKEN" && key != "DOCKER_HOST" && key != "DOCKER_CONTEXT" && key != "DOCKER_TLS_VERIFY" && key != "DOCKER_CERT_PATH" {
			cmd.Env = append(cmd.Env, e)
		}
	}
	if b.docker != nil {
		cmd.Env = append(cmd.Env, "DOCKER_HOST="+b.docker.Client.DaemonHost())
	}
	output := &limitedOutput{}
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	text := output.String()
	if output.truncated {
		text += "\n[output truncated at 256 KiB]"
	}
	if err != nil {
		return text, fmt.Errorf("%s: %w", argv[0], err)
	}
	return text, nil
}

func (b *backend) selectedContainers(ctx context.Context, i item) ([]container.Summary, error) {
	cs, err := b.containers(ctx)
	if err != nil {
		return nil, err
	}
	result := []container.Summary{}
	for _, c := range cs {
		if c.ID == i.ID || (i.Project != "" && c.Labels["com.docker.compose.project"] == i.Project && (i.Service == "" || c.Labels["com.docker.compose.service"] == i.Service)) {
			result = append(result, c)
		}
	}
	return result, nil
}
