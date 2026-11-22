package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/jesseduffield/lazydocker/pkg/commands"
)

func pretty(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}
func redacted(v any) any {
	data, _ := json.Marshal(v)
	var value any
	_ = json.Unmarshal(data, &value)
	var walk func(any) any
	walk = func(v any) any {
		switch o := v.(type) {
		case map[string]any:
			for k, x := range o {
				key := strings.ToLower(k)
				if key == "env" || key == "cmd" || key == "args" || key == "entrypoint" || key == "labels" || key == "healthcheck" || key == "options" || key == "driveropts" || key == "driver_opts" || strings.Contains(key, "password") || strings.Contains(key, "secret") || strings.Contains(key, "token") || strings.Contains(key, "auth") {
					o[k] = "[hidden; use Reveal]"
				} else {
					o[k] = walk(x)
				}
			}
		case []any:
			for i, x := range o {
				o[i] = walk(x)
			}
		}
		return v
	}
	return walk(value)
}
func (b *backend) detail(ctx context.Context, kind, id, tab string, reveal bool) (string, error) {
	valid := false
	for _, v := range viewIDs[kind] {
		if v == tab {
			valid = true
		}
	}
	if !valid {
		return "", fmt.Errorf("unsupported detail tab")
	}
	items, err := b.inventory(ctx, kind)
	if err != nil {
		return "", err
	}
	var selected item
	found := false
	for _, i := range items {
		if i.ID == id {
			selected = i
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("resource no longer exists")
	}
	if b.demo {
		return b.demoDetail(kind, selected, tab, reveal), nil
	}
	format := func(v any) string {
		if !reveal {
			v = redacted(v)
		}
		return pretty(v)
	}
	if kind == "services" {
		if tab == "logs" || tab == "top" {
			p, e := b.project(ctx, selected.Project)
			if e != nil {
				return "", e
			}
			argv, e := b.composeArgs(p, tab, selected.Service)
			if e != nil {
				return "", e
			}
			return b.run(ctx, p.Path, argv, false)
		}
		cs, e := b.selectedContainers(ctx, selected)
		if e != nil {
			return "", e
		}
		if len(cs) == 0 {
			return "", fmt.Errorf("service has no existing container")
		}
		kind, id = "containers", cs[0].ID
	}
	switch kind {
	case "containers":
		c := &commands.Container{ID: id, Name: selected.Name, Client: b.docker.Client, Log: b.docker.Log}
		switch tab {
		case "inspect":
			v, e := c.Inspect()
			return format(v), e
		case "top":
			return c.RenderTop(ctx)
		case "env":
			v, e := c.Inspect()
			if e != nil {
				return "", e
			}
			if v.Config == nil {
				return "", fmt.Errorf("container config unavailable")
			}
			lines := []string{}
			for _, entry := range v.Config.Env {
				if !reveal {
					entry = strings.SplitN(entry, "=", 2)[0] + "=[hidden]"
				}
				lines = append(lines, entry)
			}
			return strings.Join(lines, "\n"), nil
		case "stats":
			response, e := b.docker.Client.ContainerStats(ctx, id, false)
			if e != nil {
				return "", e
			}
			defer response.Body.Close()
			var s commands.ContainerStats
			if e = json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(&s); e != nil {
				return "", e
			}
			cpu, memory := s.CalculateContainerCPUPercentage(), s.CalculateContainerMemoryUsage()
			if math.IsNaN(cpu) || math.IsInf(cpu, 0) {
				cpu = 0
			}
			if math.IsNaN(memory) || math.IsInf(memory, 0) {
				memory = 0
			}
			return b.statsText(id, object{"ClientStats": s, "DerivedStats": object{"CPUPercentage": cpu, "MemoryPercentage": memory, "MemoryBytes": s.MemoryStats.Usage}}, cpu), nil
		case "logs":
			v, e := c.Inspect()
			if e != nil {
				return "", e
			}
			stream, e := b.docker.Client.ContainerLogs(ctx, id, container.LogsOptions{ShowStdout: true, ShowStderr: true, Tail: strconv.Itoa(b.logSettings().Tail), Timestamps: b.logSettings().Timestamps, Since: b.logSettings().Since})
			if e != nil {
				return "", e
			}
			defer stream.Close()
			raw, e := io.ReadAll(io.LimitReader(stream, 256*1024))
			if e != nil {
				return "", e
			}
			if v.Config != nil && v.Config.Tty {
				return string(raw), nil
			}
			var out bytes.Buffer
			_, e = stdcopy.StdCopy(&out, &out, bytes.NewReader(raw))
			if e != nil && !strings.Contains(e.Error(), "EOF") {
				return "", e
			}
			if len(raw) == 256*1024 {
				out.WriteString("\n[log snapshot truncated at 256 KiB]")
			}
			return out.String(), nil
		}
	case "projects":
		if tab == "config" && !reveal {
			return "Compose configuration can contain interpolated secrets. Select Reveal to load its full content.", nil
		}
		p, e := b.project(ctx, selected.Project)
		if e != nil {
			return "", e
		}
		argv, e := b.composeArgs(p, tab, "")
		if e != nil {
			return "", e
		}
		return b.run(ctx, p.Path, argv, false)
	case "images":
		if tab == "history" {
			return (&commands.Image{ID: id, Client: b.docker.Client}).RenderHistory()
		}
		v, e := b.docker.Client.ImageInspect(ctx, id)
		return format(v), e
	case "volumes":
		v, e := b.docker.Client.VolumeInspect(ctx, id)
		return format(v), e
	case "networks":
		v, e := b.docker.Client.NetworkInspect(ctx, id, network.InspectOptions{})
		return format(v), e
	case "commands":
		b.mu.Lock()
		defer b.mu.Unlock()
		for _, c := range b.config.Commands {
			if c.ID == id {
				if !reveal {
					return "Command argv may contain secrets. Select Reveal to inspect its definition.", nil
				}
				return pretty(c), nil
			}
		}
	case "host":
		switch tab {
		case "info":
			v, e := b.docker.Client.Info(ctx)
			return format(v), e
		case "disk":
			v, e := b.docker.Client.DiskUsage(ctx, types.DiskUsageOptions{})
			return format(v), e
		case "events":
			v, e := b.events(ctx)
			return format(v), e
		case "config":
			b.mu.Lock()
			defer b.mu.Unlock()
			if !reveal {
				return "Select Reveal to inspect local application configuration. Edit it in Settings.", nil
			}
			return pretty(b.config), nil
		}
	}
	return "", fmt.Errorf("detail not available")
}

func (b *backend) events(ctx context.Context) ([]events.Message, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	now := time.Now()
	messages, errs := b.docker.Client.Events(ctx, events.ListOptions{Since: strconv.FormatInt(now.Add(-time.Hour).Unix(), 10), Until: strconv.FormatInt(now.Unix(), 10), Filters: filters.NewArgs(filters.Arg("type", "container"))})
	result := []events.Message{}
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				return result, nil
			}
			result = append(result, message)
			if len(result) > 100 {
				result = result[len(result)-100:]
			}
		case err := <-errs:
			if err == nil || err == io.EOF {
				return result, nil
			}
			return result, err
		case <-ctx.Done():
			return result, ctx.Err()
		}
	}
}

// Expensive per-container sampling runs independently of UI polling. A stalled
// daemon sample cannot hold the dashboard HTTP response or start N inspections.
func (b *backend) sampleLiveMetrics() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		cs, err := b.containers(ctx)
		if err == nil {
			type measured struct {
				id, name string
				cpu      float64
			}
			values := []measured{}
			var mu sync.Mutex
			var wg sync.WaitGroup
			slots := make(chan struct{}, 4)
			for _, c := range cs {
				if c.State != "running" {
					continue
				}
				wg.Add(1)
				go func(c container.Summary) {
					defer wg.Done()
					select {
					case slots <- struct{}{}:
					case <-ctx.Done():
						return
					}
					defer func() { <-slots }()
					response, e := b.docker.Client.ContainerStats(ctx, c.ID, false)
					if e != nil {
						return
					}
					defer response.Body.Close()
					var stats commands.ContainerStats
					if json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(&stats) != nil {
						return
					}
					cpu := stats.CalculateContainerCPUPercentage()
					if math.IsNaN(cpu) || math.IsInf(cpu, 0) {
						return
					}
					mu.Lock()
					values = append(values, measured{c.ID, containerName(c), cpu})
					mu.Unlock()
				}(c)
			}
			wg.Wait()
			sort.Slice(values, func(i, j int) bool { return values[i].cpu > values[j].cpu })
			consumers := []object{}
			cpus := map[string]string{}
			for _, v := range values {
				text := fmt.Sprintf("%.2f%%", v.cpu)
				cpus[v.id] = text
				if len(consumers) < 5 {
					consumers = append(consumers, object{"id": v.id, "name": v.name, "cpu": text})
				}
			}
			b.mu.Lock()
			b.liveConsumers, b.liveCPU = consumers, cpus
			b.mu.Unlock()
		}
		eventList, e := b.events(ctx)
		if e == nil {
			rows := []object{}
			if len(eventList) > 6 {
				eventList = eventList[len(eventList)-6:]
			}
			for _, event := range eventList {
				tone := "neutral"
				if event.Action == "start" {
					tone = "ok"
				}
				if event.Action == "die" || event.Action == "oom" || event.Action == "kill" {
					tone = "warn"
				}
				rows = append(rows, object{"id": fmt.Sprintf("%s-%d", event.Actor.ID, event.TimeNano), "name": event.Actor.Attributes["name"], "what": string(event.Action), "tone": tone, "at": time.Unix(event.Time, 0).Format("15:04")})
			}
			b.mu.Lock()
			b.liveEvents = rows
			b.mu.Unlock()
		}
		cancel()
		time.Sleep(10 * time.Second)
	}
}
func (b *backend) demoDetail(kind string, i item, tab string, reveal bool) string {
	switch tab {
	case "logs":
		return "[DEMO] 12:00:00 service ready\n[DEMO] 12:00:01 health check passed\nNo live Docker logs are used."
	case "stats":
		return b.statsText(i.ID, object{"mode": "demo", "DerivedStats": object{"CPUPercentage": 0.5, "MemoryBytes": 67108864, "MemoryPercentage": 12.5}}, 0.5)
	case "env":
		if reveal {
			return "DEMO_MODE=true\nDEMO_SECRET=sample-not-a-credential"
		}
		return "DEMO_MODE=[hidden]\nDEMO_SECRET=[hidden]"
	case "top":
		return "PID  USER  COMMAND\n1    demo  sample-server"
	case "history":
		return "IMAGE              SIZE      COMMAND\ndemo-layer         8 MB      COPY sample /app\ndemo-base          56 MB     FROM sample"
	case "events":
		return "[DEMO] container sample started at 12:00"
	case "config":
		if !reveal {
			return "Demo configuration hidden. Select Reveal to view."
		}
		return pretty(object{"mode": "demo", "resource": i.Name})
	case "info":
		return "Demo Docker Engine · isolated in-memory simulation"
	case "disk":
		return "Demo disk usage: 72 MB images, one sample volume"
	default:
		v := object{"Id": i.ID, "Name": i.Name, "State": i.State, "Config": object{"Env": []string{"DEMO_SECRET=sample-not-a-credential"}}, "mode": "demo"}
		if !reveal {
			return pretty(redacted(v))
		}
		return pretty(v)
	}
}

var meterMu sync.Mutex

func (b *backend) statsText(id string, data object, cpu float64) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.statsHistory == nil {
		b.statsHistory = map[string][]float64{}
		b.statsTimes = map[string][]time.Time{}
	}
	if b.statsTimes == nil {
		b.statsTimes = map[string][]time.Time{}
	}
	duration, err := time.ParseDuration(b.config.StatsMaxDuration)
	if err != nil || duration <= 0 {
		duration = 2 * time.Minute
	}
	now := time.Now()
	appendValue := func(key string, value float64) []float64 {
		values := append(b.statsHistory[key], value)
		times := append(b.statsTimes[key], now)
		first := 0
		for first < len(times)-1 && times[first].Before(now.Add(-duration)) {
			first++
		}
		if len(values)-first > 2048 {
			first = len(values) - 2048
		}
		values, times = values[first:], times[first:]
		b.statsHistory[key], b.statsTimes[key] = values, times
		return values
	}
	history := appendValue(id, cpu)
	graphs := b.config.Stats
	if len(graphs) == 0 {
		graphs = emptyConfig().Stats
	}
	metrics := []object{}
	for _, g := range graphs {
		number, ok := statValue(data, g.StatPath)
		if !ok {
			metrics = append(metrics, object{"label": g.Label, "value": "Unavailable: " + g.StatPath, "percent": 0, "history": []float64{}})
			continue
		}
		raw := appendValue(id+":"+configDigest(g), number)
		min, max := g.Min, g.Max
		if g.AutoMin {
			min = raw[0]
			for _, v := range raw {
				min = math.Min(min, v)
			}
		}
		if g.AutoMax {
			max = raw[0]
			for _, v := range raw {
				max = math.Max(max, v)
			}
		}
		if max <= min {
			max = min + 1
		}
		points := make([]float64, len(raw))
		for n, v := range raw {
			points[n] = math.Max(0, math.Min(100, 100*(v-min)/(max-min)))
		}
		metrics = append(metrics, object{"label": g.Label, "value": fmt.Sprintf("%.2f", number), "percent": points[len(points)-1], "history": points})
	}
	data["metrics"], data["points"] = metrics, history
	return pretty(data)
}

func statValue(root any, path string) (float64, bool) {
	v := reflect.ValueOf(root)
	for _, part := range strings.Split(path, ".") {
		for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
			if v.IsNil() {
				return 0, false
			}
			v = v.Elem()
		}
		if !v.IsValid() {
			return 0, false
		}
		switch v.Kind() {
		case reflect.Map:
			if v.Type().Key().Kind() != reflect.String {
				return 0, false
			}
			v = v.MapIndex(reflect.ValueOf(part))
		case reflect.Struct:
			v = v.FieldByName(part)
		default:
			return 0, false
		}
	}
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return 0, false
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return 0, false
	}
	var number float64
	switch v.Kind() {
	case reflect.Float64, reflect.Float32:
		number = v.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		number = float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		number = float64(v.Uint())
	default:
		return 0, false
	}
	return number, !math.IsNaN(number) && !math.IsInf(number, 0)
}

var lastCPUIdle, lastCPUTotal uint64

func hostMeters() []object {
	result := []object{}
	data, err := os.ReadFile("/proc/meminfo")
	if err == nil {
		var total, available uint64
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				_, _ = fmt.Sscanf(line, "MemTotal: %d kB", &total)
			}
			if strings.HasPrefix(line, "MemAvailable:") {
				_, _ = fmt.Sscanf(line, "MemAvailable: %d kB", &available)
			}
		}
		if total > 0 {
			used := total - available
			pct := 100 * float64(used) / float64(total)
			result = append(result, object{"label": "Memory", "valueText": fmt.Sprintf("%.1f GB / %.1f GB", float64(used)/1e6, float64(total)/1e6), "percent": pct, "warn": pct > 80})
		}
	}
	if data, e := os.ReadFile("/proc/stat"); e == nil {
		fields := strings.Fields(strings.SplitN(string(data), "\n", 2)[0])
		var total, idle uint64
		for i, f := range fields[1:] {
			if i >= 8 {
				break
			}
			n, _ := strconv.ParseUint(f, 10, 64)
			total += n
			if i == 3 || i == 4 {
				idle += n
			}
		}
		meterMu.Lock()
		if lastCPUTotal > 0 && total > lastCPUTotal {
			pct := 100 * (1 - float64(idle-lastCPUIdle)/float64(total-lastCPUTotal))
			result = append(result, object{"label": "CPU", "valueText": fmt.Sprintf("%.1f%%", pct), "percent": pct, "warn": pct > 80})
		}
		lastCPUIdle, lastCPUTotal = idle, total
		meterMu.Unlock()
	}
	var stat syscall.Statfs_t
	if syscall.Statfs("/var/lib/docker", &stat) == nil && stat.Blocks > 0 {
		pct := 100 * (1 - float64(stat.Bavail)/float64(stat.Blocks))
		result = append(result, object{"label": "/var/lib/docker", "valueText": fmt.Sprintf("%.1f%% used", pct), "percent": pct, "warn": pct > 80})
	}
	return result
}
