// LazyConteiner exposes Lazydocker's Docker core to the local Exact UI.
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/jesseduffield/lazydocker/pkg/commands"
	"github.com/sirupsen/logrus"
)

type object = map[string]any

type containerRow struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Stack    string `json:"stack"`
	State    string `json:"state"`
	ExitCode int    `json:"exitCode"`
	Ports    string `json:"ports"`
	CPU      string `json:"cpu"`
	Selected bool   `json:"selected"`
}

type stack struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	IsCompose  bool           `json:"isCompose"`
	Running    int            `json:"running"`
	Total      int            `json:"total"`
	Status     string         `json:"status"`
	Containers []containerRow `json:"containers"`
}

type backend struct {
	docker *commands.DockerCommand
	demo   bool
	token  string
	mu     sync.Mutex
	rows   []containerRow
}

func main() {
	demo := flag.Bool("demo", false, "use isolated sample data; never contact Docker")
	addr := flag.String("listen", "127.0.0.1:17878", "loopback listen address")
	flag.Parse()
	host, _, err := net.SplitHostPort(*addr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("--listen must use a loopback IP address")
	}
	b := &backend{demo: *demo, token: os.Getenv("LAZYCONTAINER_TOKEN"), rows: demoRows()}
	if !*demo {
		// Inject only the Docker client. NewDockerCommand also starts the TUI's
		// config/Compose/SSH setup and can terminate the process on errors.
		cli, err := localDockerClient()
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()
		b.docker = &commands.DockerCommand{Client: cli, Log: logrus.NewEntry(logrus.New())}
	}
	if b.token == "" {
		log.Print("LAZYCONTAINER_TOKEN absent: lifecycle actions disabled")
	}
	server := &http.Server{Addr: *addr, Handler: b.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second}
	log.Printf("LazyConteiner backend listening on http://%s (demo=%t)", *addr, *demo)
	log.Fatal(server.ListenAndServe())
}

func localDockerClient() (*client.Client, error) {
	// The UI identifies this daemon as local. Never inherit a remote endpoint
	// and accidentally apply a confirmed local operation to another machine.
	if host := os.Getenv("DOCKER_HOST"); host != "" && !strings.HasPrefix(host, "unix://") {
		return nil, fmt.Errorf("this iteration requires a local Unix Docker socket; unset DOCKER_HOST or set it to unix:///path/to/docker.sock")
	}
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation(), client.WithTimeout(10*time.Second))
}

func (b *backend) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, object{"ok": true, "mode": b.mode(), "core": "lazydocker", "actionsEnabled": b.token != ""})
	})
	mux.HandleFunc("GET /api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		dash, err := b.dashboard(r.Context())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, object{"error": err.Error(), "mode": b.mode()})
			return
		}
		writeJSON(w, http.StatusOK, dash)
	})
	mux.HandleFunc("POST /api/containers/{id}/{op}", b.containerAction)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// The Exact data process makes native HTTP calls. Browser callers are
		// denied, including localhost pages, to prevent cross-site Docker access.
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() || r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" {
			writeJSON(w, http.StatusForbidden, object{"error": "native loopback client required"})
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (b *backend) mode() string {
	if b.demo {
		return "demo"
	}
	return "live"
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (b *backend) containerAction(w http.ResponseWriter, r *http.Request) {
	if b.token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-LazyContainer-Token")), []byte(b.token)) != 1 {
		writeJSON(w, http.StatusUnauthorized, object{"error": "lifecycle action requires session token"})
		return
	}
	op := r.PathValue("op")
	if op != "start" && op != "stop" && op != "restart" {
		writeJSON(w, http.StatusBadRequest, object{"error": "supported actions: start, stop, restart"})
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	id := r.PathValue("id")
	if b.demo {
		for i := range b.rows {
			if b.rows[i].ID == id {
				b.rows[i].State, b.rows[i].ExitCode = "running", -1
				if op == "stop" {
					b.rows[i].State, b.rows[i].ExitCode = "exited", 0
				}
				writeJSON(w, http.StatusOK, object{"count": 1, "mode": "demo"})
				return
			}
		}
	} else {
		containers, err := b.docker.GetContainers(nil)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, object{"error": err.Error()})
			return
		}
		for _, ctr := range containers {
			// Resolve exact IDs from current inventory, never names or shell input.
			if ctr.ID != id {
				continue
			}
			switch op {
			case "start":
				err = ctr.Start()
			case "stop":
				err = ctr.Stop()
			case "restart":
				err = ctr.Restart()
			}
			if err != nil {
				writeJSON(w, http.StatusBadGateway, object{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, object{"count": 1, "mode": "live"})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, object{"error": "container no longer exists"})
}

func (b *backend) dashboard(ctx context.Context) (object, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	rows := b.rows
	if !b.demo {
		containers, err := b.docker.GetContainers(nil)
		if err != nil {
			return nil, fmt.Errorf("Docker unavailable: %w", err)
		}
		rows = make([]containerRow, 0, len(containers))
		for _, ctr := range containers {
			exitCode := -1
			if ctr.Details.ContainerJSONBase != nil && ctr.Details.State != nil && ctr.Container.State != "running" {
				exitCode = ctr.Details.State.ExitCode
			}
			ports := []string{}
			for _, port := range ctr.Container.Ports {
				if port.PublicPort != 0 {
					ports = append(ports, fmt.Sprintf("%s:%d → %d/%s", port.IP, port.PublicPort, port.PrivatePort, port.Type))
				}
			}
			portText := strings.Join(ports, ", ")
			if portText == "" {
				portText = "—"
			}
			rows = append(rows, containerRow{ID: ctr.ID, Name: ctr.Name, Stack: ctr.ProjectName, State: ctr.Container.State, ExitCode: exitCode, Ports: portText, CPU: "—"})
		}
	}
	dash := summarize(rows, b.mode())
	if b.demo {
		return dash, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	warnings := []string{"CPU, host metrics and events are not collected in this iteration."}
	totals := dash["totals"].(object)
	usage, err := b.docker.Client.DiskUsage(ctx, types.DiskUsageOptions{})
	if err != nil {
		warnings = append(warnings, "Docker disk usage unavailable: "+err.Error())
	} else {
		totals["images"], totals["volumes"] = len(usage.Images), len(usage.Volumes)
		var size int64
		unusedImages, unusedVolumes := 0, 0
		for _, image := range usage.Images {
			size += image.Size
			if image.Containers == 0 {
				unusedImages++
			}
		}
		for _, volume := range usage.Volumes {
			if volume.UsageData != nil && volume.UsageData.RefCount == 0 {
				unusedVolumes++
			}
		}
		totals["imagesSize"], totals["imagesUnused"], totals["volumesUnused"] = humanBytes(size), unusedImages, unusedVolumes
	}
	networks, err := b.docker.Client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		warnings = append(warnings, "Docker network inventory unavailable: "+err.Error())
	} else {
		totals["networks"] = len(networks)
		totals["networksDetail"] = "Docker networks"
	}
	dash["warnings"] = warnings
	return dash, nil
}

func summarize(rows []containerRow, mode string) object {
	groups := map[string]*stack{}
	running, failed, clean := 0, 0, 0
	alertItems := []object{}
	for _, row := range rows {
		id, name := row.Stack, row.Stack
		if id == "" {
			id, name = "__standalone", "Standalone containers"
		}
		if groups[id] == nil {
			groups[id] = &stack{ID: id, Name: name, IsCompose: row.Stack != "", Containers: []containerRow{}}
		}
		group := groups[id]
		group.Containers = append(group.Containers, row)
		group.Total++
		if row.State == "running" {
			group.Running++
			running++
		} else if row.State == "exited" || row.State == "dead" {
			if row.ExitCode > 0 {
				failed++
				alertItems = append(alertItems, object{"id": row.ID, "name": row.Name, "exitCode": row.ExitCode})
			} else if row.ExitCode == 0 {
				clean++
			}
		}
	}
	stacks := []*stack{}
	for _, group := range groups {
		group.Status = "Stopped"
		if group.Running == group.Total {
			group.Status = "Running"
		} else if group.Running > 0 {
			group.Status = "Partial"
		}
		sort.Slice(group.Containers, func(i, j int) bool { return group.Containers[i].Name < group.Containers[j].Name })
		stacks = append(stacks, group)
	}
	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Name < stacks[j].Name })
	alerts := []object{}
	if len(alertItems) > 0 {
		alerts = append(alerts, object{"id": "failed", "title": fmt.Sprintf("%d containers exited with errors", failed), "hint": "Review logs before restarting. Exit 137 alone does not prove an OOM kill.", "cta": fmt.Sprintf("Restart all %d", failed), "ctaAction": "restart", "tone": "warn", "items": alertItems})
	}
	label, detail := "local · Docker", "connected · Docker Engine"
	if mode == "demo" {
		label, detail = "demo · sample fleet", "isolated sample data · no Docker connection"
	}
	env := object{"id": "local", "label": label, "connected": true, "detail": detail}
	return object{
		"mode": mode, "error": "", "updatedAt": time.Now().UTC().Format(time.RFC3339), "env": env, "envs": []object{env},
		"totals": object{"containers": len(rows), "running": running, "failed": failed, "cleanExit": clean, "images": 0, "imagesSize": "Unavailable", "imagesUnused": 0, "volumes": 0, "volumesUnused": 0, "networks": 0, "networksDetail": "Unavailable"},
		"alerts": alerts, "stacks": stacks, "selectedCount": 0, "host": []object{}, "serviceActive": mode == "live", "consumers": []object{}, "events": []object{}, "cleanup": []object{
			{"id": "containers", "label": "Stopped containers", "size": "Preview required", "checked": true, "dangerous": false},
			{"id": "images", "label": "Dangling images", "size": "Preview required", "checked": true, "dangerous": false},
			{"id": "volumes", "label": "Unused volumes", "size": "Preview required", "checked": false, "dangerous": true},
			{"id": "cache", "label": "Build cache", "size": "Preview required", "checked": false, "dangerous": false},
		}, "unread": len(alerts), "attention": failed, "warnings": []string{},
	}
}

func humanBytes(n int64) string { return fmt.Sprintf("%.1f GB", float64(n)/1e9) }

func demoRows() []containerRow {
	return []containerRow{
		{ID: "demo-api", Name: "kodus_api", Stack: "kodus", State: "exited", ExitCode: 137, Ports: "—", CPU: "—"},
		{ID: "demo-web", Name: "kodus-web-prod", Stack: "kodus", State: "exited", ExitCode: 0, Ports: "—", CPU: "—"},
		{ID: "demo-worker", Name: "kodus-worker-prod", Stack: "kodus", State: "exited", ExitCode: 137, Ports: "—", CPU: "—"},
		{ID: "demo-agent", Name: "openhands-agent-canvas", Stack: "openhands", State: "running", ExitCode: -1, Ports: "127.0.0.1:8000 → 8000/tcp", CPU: "—"},
		{ID: "demo-tts", Name: "ccs-tts", State: "running", ExitCode: -1, Ports: "—", CPU: "—"},
		{ID: "demo-stopped", Name: "cool_hodgkin", State: "exited", ExitCode: 1, Ports: "—", CPU: "—"},
	}
}
