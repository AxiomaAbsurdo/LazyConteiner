// LazyConteiner exposes Lazydocker's Docker core to the local Exact UI.
package main

import (
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
	docker          *commands.DockerCommand
	demo            bool
	token           string
	mu              sync.Mutex
	rows            []containerRow
	cache           map[string]cachedValue
	cacheGeneration uint64
	plans           map[string]actionPlan
	jobs            map[string]*job
	config          appConfig
	configPath      string
	runner          commandRunner
	demoRemoved     map[string]bool
	statsHistory    map[string][]float64
	statsTimes      map[string][]time.Time
	liveConsumers   []object
	liveCPU         map[string]string
	liveEvents      []object
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
	if err := b.loadConfig(); err != nil {
		log.Fatal(err)
	}
	if !*demo {
		// Inject only the Docker client. NewDockerCommand also starts the TUI's
		// config/Compose/SSH setup and can terminate the process on errors.
		cli, err := localDockerClient()
		if err != nil {
			log.Fatal(err)
		}
		defer cli.Close()
		b.docker = &commands.DockerCommand{Client: cli, Log: logrus.NewEntry(logrus.New())}
		go b.sampleLiveMetrics()
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
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation(), client.WithTimeout(45*time.Second))
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
	mux.HandleFunc("POST /api/containers/{id}/{op}", func(w http.ResponseWriter, r *http.Request) {
		failure(w, http.StatusGone, fmt.Errorf("use /api/action/preview then /api/action; direct mutations are disabled"))
	})
	b.registerAPI(mux)
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
