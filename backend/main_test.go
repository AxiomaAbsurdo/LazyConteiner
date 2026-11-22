package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/client"
	"github.com/jesseduffield/lazydocker/pkg/commands"
	"github.com/sirupsen/logrus"
)

func call(b *backend, method, path, token, origin, host string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:17878"+path, nil)
	r.Header.Set("X-LazyContainer-Token", token)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if host != "" {
		r.Host = host
	}
	w := httptest.NewRecorder()
	b.handler().ServeHTTP(w, r)
	return w
}

func TestActionBoundary(t *testing.T) {
	b := &backend{demo: true, token: "test-secret", rows: demoRows()}
	for _, tt := range []struct {
		name, path, token, origin, host string
		want                            int
	}{
		{"missing token", "/api/containers/demo-api/start", "", "", "", 401},
		{"wrong token", "/api/containers/demo-api/start", "bad", "", "", 401},
		{"cross site", "/api/containers/demo-api/start", "test-secret", "https://evil.example", "", 403},
		{"null origin", "/api/containers/demo-api/start", "test-secret", "null", "", 403},
		{"rebinding", "/api/containers/demo-api/start", "test-secret", "", "evil.example:17878", 403},
		{"unsupported delete", "/api/containers/demo-api/remove", "test-secret", "", "", 400},
		{"unknown container", "/api/containers/unknown/start", "test-secret", "", "", 404},
		{"valid action", "/api/containers/demo-api/start", "test-secret", "", "", 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := call(b, http.MethodPost, tt.path, tt.token, tt.origin, tt.host)
			if w.Code != tt.want {
				t.Fatalf("got %d: %s; want %d", w.Code, w.Body, tt.want)
			}
		})
	}
	if b.rows[0].State != "running" {
		t.Fatal("valid action did not update isolated demo inventory")
	}
	b.token = ""
	if got := call(b, http.MethodPost, "/api/containers/demo-api/stop", "", "", "").Code; got != 401 {
		t.Fatalf("empty configuration must disable actions: %d", got)
	}
}

func TestLocalDockerClientRejectsRemoteEndpoints(t *testing.T) {
	for _, host := range []string{"tcp://127.0.0.1:2375", "tcp://production.example:2376", "ssh://production.example", "http://production.example"} {
		t.Run(host, func(t *testing.T) {
			t.Setenv("DOCKER_HOST", host)
			cli, err := localDockerClient()
			if err == nil || cli != nil {
				t.Fatal("non-Unix Docker endpoint must be rejected before any connection")
			}
			if strings.Contains(err.Error(), host) {
				t.Fatal("error must not disclose the configured endpoint")
			}
		})
	}
	for _, host := range []string{"", "unix:///tmp/lazyconteiner-rootless-test.sock"} {
		t.Run("local "+host, func(t *testing.T) {
			t.Setenv("DOCKER_HOST", host)
			t.Setenv("DOCKER_TLS_VERIFY", "")
			t.Setenv("DOCKER_CERT_PATH", "")
			cli, err := localDockerClient()
			if err != nil {
				t.Fatal(err)
			}
			defer cli.Close()
			if !strings.HasPrefix(cli.DaemonHost(), "unix://") {
				t.Fatal("local configuration must resolve to a Unix socket")
			}
		})
	}
}

func TestDashboardAggregationAndDemoIsolation(t *testing.T) {
	b := &backend{demo: true, rows: demoRows()}
	w := call(b, http.MethodGet, "/api/dashboard", "", "", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var result struct {
		Mode   string           `json:"mode"`
		Stacks []stack          `json:"stacks"`
		Totals map[string]any   `json:"totals"`
		Host   []map[string]any `json:"host"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Mode != "demo" || len(result.Stacks) != 3 || result.Totals["running"] != float64(2) || result.Totals["failed"] != float64(3) || result.Totals["cleanExit"] != float64(1) {
		t.Fatalf("incorrect dashboard: %s", w.Body.String())
	}
	if result.Host == nil || len(result.Host) != 0 {
		t.Fatal("uncollected metrics must be an empty array, never fabricated")
	}
	// Paused and created containers are not clean exits or failed exits.
	d := summarize([]containerRow{{ID: "x", State: "paused", ExitCode: 0}, {ID: "y", State: "created", ExitCode: 0}}, "live")
	if d["totals"].(object)["cleanExit"] != 0 || d["totals"].(object)["failed"] != 0 {
		t.Fatal("paused/created containers classified as exits")
	}
}

func TestLazydockerCoreAgainstDockerAPI(t *testing.T) {
	started := false
	usageUnavailable := false
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch strings.TrimPrefix(r.URL.Path, "/v1.48") {
		case "/containers/json":
			if r.URL.Query().Get("all") != "1" {
				t.Error("Lazydocker must include stopped containers")
			}
			_, _ = w.Write([]byte(`[{"Id":"abc123","Names":["/sample-api"],"State":"exited","Labels":{"com.docker.compose.project":"sample"},"Ports":[]}]`))
		case "/containers/abc123/json":
			_, _ = w.Write([]byte(`{"Id":"abc123","Name":"/sample-api","State":{"Status":"exited","ExitCode":7}}`))
		case "/containers/abc123/start":
			if r.Method != "POST" {
				t.Errorf("wrong mutation method: %s", r.Method)
			}
			started = true
			w.WriteHeader(http.StatusNoContent)
		case "/system/df":
			if usageUnavailable {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"message":"disk usage temporarily unavailable"}`))
				return
			}
			_, _ = w.Write([]byte(`{"Images":[],"Volumes":[]}`))
		case "/networks":
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Errorf("unexpected Docker request: %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer daemon.Close()
	cli, err := client.NewClientWithOpts(client.WithHost(daemon.URL), client.WithVersion("1.48"))
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	b := &backend{token: "test-secret", docker: &commands.DockerCommand{Client: cli, Log: logrus.NewEntry(logrus.New())}}
	w := call(b, http.MethodGet, "/api/dashboard", "", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"exitCode":7`) || !strings.Contains(w.Body.String(), `"name":"sample-api"`) {
		t.Fatalf("Lazydocker inventory mapping failed: %d %s", w.Code, w.Body)
	}
	w = call(b, http.MethodPost, "/api/containers/abc123/start", "test-secret", "", "")
	if w.Code != 200 || !started {
		t.Fatalf("Lazydocker lifecycle request failed: %d %s", w.Code, w.Body)
	}
	usageUnavailable = true
	w = call(b, http.MethodGet, "/api/dashboard", "", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `Docker disk usage unavailable`) || !strings.Contains(w.Body.String(), `"imagesSize":"Unavailable"`) {
		t.Fatalf("partial inventory failure must retain unavailable state and warning: %d %s", w.Code, w.Body)
	}
	daemon.Close()
	w = call(b, http.MethodGet, "/api/dashboard", "", "", "")
	if w.Code != 503 || !strings.Contains(w.Body.String(), "Docker unavailable") {
		t.Fatalf("unavailable Docker must not fall back to demo: %d %s", w.Code, w.Body)
	}
}
