package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/jesseduffield/lazydocker/pkg/commands"
	"github.com/sirupsen/logrus"
)

func apiCall(b *backend, method, path string, body any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "http://127.0.0.1:17878"+path, strings.NewReader(string(data)))
	r.Header.Set("X-LazyContainer-Token", b.token)
	w := httptest.NewRecorder()
	b.handler().ServeHTTP(w, r)
	return w
}
func testBackend() *backend {
	c := emptyConfig()
	c.Commands = []customCommand{{ID: "hello", Label: "Hello", Scope: "commands", Argv: []string{"printf", "hello"}}, {ID: "container-echo", Label: "Echo container", Scope: "containers", Argv: []string{"printf", "%s", "{id}"}}}
	return &backend{demo: true, token: "test-secret", rows: demoRows(), config: c}
}
func TestEveryAdvertisedDemoAction(t *testing.T) {
	for kind, ops := range actionIDs {
		for _, op := range ops {
			t.Run(kind+"/"+op, func(t *testing.T) {
				b := testBackend()
				items, err := b.inventory(context.Background(), kind)
				if err != nil || len(items) == 0 {
					t.Fatalf("inventory: %v", err)
				}
				r := actionRequest{Kind: kind, IDs: []string{items[0].ID}, Op: op}
				if op == "exec" {
					r.Args = []string{"sh", "-c", "printf demo"}
				}
				p, summary, err := b.preview(context.Background(), r)
				if err != nil {
					t.Fatal(err)
				}
				if summary == "" || len(p.Targets) == 0 {
					t.Fatal("missing exact confirmation targets")
				}
				if err = b.validatePlan(context.Background(), p); err != nil {
					t.Fatal(err)
				}
				count, text, err := b.execute(context.Background(), p)
				if err != nil || count != len(p.Targets) || text == "" {
					t.Fatalf("count=%d text=%s err=%v", count, text, err)
				}
			})
		}
	}
}
func TestEveryAdvertisedDetailAndStats(t *testing.T) {
	for kind, tabs := range viewIDs {
		for _, tab := range tabs {
			t.Run(kind+"/"+tab, func(t *testing.T) {
				b := testBackend()
				items, err := b.inventory(context.Background(), kind)
				if err != nil || len(items) == 0 {
					t.Fatal(err)
				}
				w := apiCall(b, "GET", "/api/detail/"+kind+"/"+url.PathEscape(items[0].ID)+"?tab="+tab, nil)
				if w.Code != 200 {
					t.Fatalf("%d %s", w.Code, w.Body)
				}
				var out object
				if err = json.Unmarshal(w.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if out["text"] == "" || out["metrics"] == nil || out["points"] == nil {
					t.Fatal("incomplete detail schema")
				}
				if tab == "stats" {
					metrics := out["metrics"].([]any)
					if len(metrics) != 2 {
						t.Fatal("expected CPU and memory graphs")
					}
					if len(metrics[0].(map[string]any)["history"].([]any)) != 1 {
						t.Fatal("missing normalized graph history")
					}
				}
			})
		}
	}
}
func TestConfirmationOneUseExpiryAndChanges(t *testing.T) {
	b := testBackend()
	preview := func() string {
		w := apiCall(b, "POST", "/api/action/preview", actionRequest{Kind: "containers", IDs: []string{"demo-api"}, Op: "remove"})
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		var v struct {
			Confirmation string `json:"confirmation"`
			ExpiresAt    string `json:"expiresAt"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		if _, err := time.Parse(time.RFC3339, v.ExpiresAt); err != nil {
			t.Fatal(err)
		}
		return v.Confirmation
	}
	token := preview()
	w := apiCall(b, "POST", "/api/action", object{"confirmation": token})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = apiCall(b, "POST", "/api/action", object{"confirmation": token}); w.Code != 409 {
		t.Fatal("replay accepted")
	}
	b = testBackend()
	token = preview()
	p := b.plans[token]
	p.Expires = time.Now().Add(-time.Second)
	b.plans[token] = p
	if w = apiCall(b, "POST", "/api/action", object{"confirmation": token}); w.Code != 409 {
		t.Fatal("expired plan accepted")
	}
	b = testBackend()
	token = preview()
	b.rows = b.rows[1:]
	if w = apiCall(b, "POST", "/api/action", object{"confirmation": token}); w.Code != 409 {
		t.Fatal("missing target accepted")
	}
	b = testBackend()
	token = preview()
	b.config.Terminal = []string{"other-terminal"}
	if w = apiCall(b, "POST", "/api/action", object{"confirmation": token}); w.Code != 409 {
		t.Fatal("config change accepted")
	}
}
func TestPruneFreezesTargetsAndRevalidatesEligibility(t *testing.T) {
	b := testBackend()
	p, _, err := b.preview(context.Background(), actionRequest{Kind: "containers", Op: "prune"})
	if err != nil {
		t.Fatal(err)
	}
	b.rows = append(b.rows, containerRow{ID: "new-exit", Name: "new-exit", State: "exited", ExitCode: 0})
	if err = b.validatePlan(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if _, _, err = b.execute(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	for _, r := range b.rows {
		if r.ID == "new-exit" {
			return
		}
	}
	t.Fatal("prune removed unpreviewed container")
}
func TestPruneRefusesNowRunningTarget(t *testing.T) {
	b := testBackend()
	p, _, err := b.preview(context.Background(), actionRequest{Kind: "containers", Op: "prune"})
	if err != nil {
		t.Fatal(err)
	}
	b.rows[0].State = "running"
	if b.validatePlan(context.Background(), p) == nil {
		t.Fatal("prune accepted now-running target")
	}
}

func TestLazydockerLifecycleAndResourceRemovalFlags(t *testing.T) {
	cases := []struct{ kind, op, method, path, query string }{
		{"containers", "start", "POST", "/containers/target/start", ""}, {"containers", "stop", "POST", "/containers/target/stop", ""}, {"containers", "restart", "POST", "/containers/target/restart", ""},
		{"containers", "pause", "POST", "/containers/target/pause", ""}, {"containers", "unpause", "POST", "/containers/target/unpause", ""}, {"containers", "kill", "POST", "/containers/target/kill", "signal=SIGKILL"},
		{"containers", "remove", "DELETE", "/containers/target", ""}, {"containers", "remove-volumes", "DELETE", "/containers/target", "v=1"}, {"containers", "remove-force", "DELETE", "/containers/target", "force=1"}, {"containers", "remove-force-volumes", "DELETE", "/containers/target", "force=1&v=1"},
		{"images", "remove", "DELETE", "/images/target", "noprune=1"}, {"images", "remove-no-prune", "DELETE", "/images/target", "noprune=1"}, {"images", "remove-force", "DELETE", "/images/target", "force=1&noprune=1"}, {"images", "remove-force-no-prune", "DELETE", "/images/target", "force=1&noprune=1"},
		{"images", "prune", "DELETE", "/images/target", "noprune=1"}, {"images", "prune-all", "DELETE", "/images/target", "noprune=1"},
		{"volumes", "remove", "DELETE", "/volumes/target", ""}, {"volumes", "remove-force", "DELETE", "/volumes/target", "force=1"}, {"networks", "remove", "DELETE", "/networks/target", ""},
	}
	for _, tt := range cases {
		t.Run(tt.kind+"/"+tt.op, func(t *testing.T) {
			called := false
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Method != tt.method || strings.TrimPrefix(r.URL.Path, "/v1.48") != tt.path || r.URL.RawQuery != tt.query {
					t.Errorf("got %s %s?%s, want %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery, tt.method, tt.path, tt.query)
				}
				w.Header().Set("Content-Type", "application/json")
				if tt.kind == "images" {
					_, _ = w.Write([]byte(`[]`))
				} else {
					w.WriteHeader(204)
				}
			}))
			defer s.Close()
			cli, err := client.NewClientWithOpts(client.WithHost(s.URL), client.WithVersion("1.48"))
			if err != nil {
				t.Fatal(err)
			}
			defer cli.Close()
			b := &backend{docker: &commands.DockerCommand{Client: cli, Log: logrus.NewEntry(logrus.New())}}
			if _, err = b.executeOne(context.Background(), actionRequest{Kind: tt.kind, Op: tt.op}, item{ID: "target", Name: "target"}); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("Docker core was not called")
			}
		})
	}
}

func TestComposeArgvEveryOperationAndInjection(t *testing.T) {
	p := projectConfig{ID: "sample", Path: "/tmp/project with spaces", Files: []string{"/tmp/project with spaces/compose.yaml"}}
	want := map[string][]string{"up": {"up", "-d"}, "up-attached": {"up"}, "down": {"down"}, "down-volumes": {"down", "--volumes"}, "down-images": {"down", "--rmi", "all"}, "down-volumes-images": {"down", "--volumes", "--rmi", "all"}, "start": {"start"}, "stop": {"stop"}, "restart": {"restart"}, "pause": {"pause"}, "unpause": {"unpause"}, "pull": {"pull"}, "build": {"build", "--parallel", "--force-rm"}, "rebuild": {"up", "-d", "--build"}, "recreate": {"up", "-d", "--force-recreate"}, "remove": {"rm", "--stop", "--force"}, "remove-volumes": {"rm", "--stop", "--force", "--volumes"}, "logs": {"logs", "--no-color", "--tail", "300"}, "logs-follow": {"logs", "--tail", "300", "--follow"}, "config": {"config"}, "top": {"top"}}
	for op, tail := range want {
		t.Run(op, func(t *testing.T) {
			args, err := composeArgv(p, op, "api")
			if err != nil {
				t.Fatal(err)
			}
			expected := append([]string{"docker", "compose", "--project-name", "sample", "--project-directory", p.Path, "-f", p.Files[0]}, tail...)
			expected = append(expected, "--", "api")
			if !reflect.DeepEqual(args, expected) {
				t.Fatalf("%q != %q", args, expected)
			}
		})
	}
	for _, bad := range []string{"--privileged", "api;touch /tmp/no", "$(id)", "api/name"} {
		if _, err := composeArgv(p, "up", bad); err == nil {
			t.Fatalf("invalid service accepted: %s", bad)
		}
	}
}
func TestTerminalExecAttachAndCustomUseArgumentArrays(t *testing.T) {
	for _, op := range []string{"exec", "attach", "logs-follow"} {
		t.Run(op, func(t *testing.T) {
			called := false
			b := &backend{docker: &commands.DockerCommand{Log: logrus.NewEntry(logrus.New())}, runner: func(ctx context.Context, cwd string, args []string, terminal bool) (string, error) {
				called = true
				if !terminal || args[0] != "docker" {
					t.Fatal("missing native terminal")
				}
				if op == "exec" && args[len(args)-1] != "hello; $(touch /tmp/no)" {
					t.Fatal("argv was interpreted")
				}
				return "completed", nil
			}}
			_, err := b.executeOne(context.Background(), actionRequest{Kind: "containers", Op: op, Args: []string{"printf", "%s", "hello; $(touch /tmp/no)"}}, item{ID: "target"})
			if err != nil || !called {
				t.Fatalf("%v", err)
			}
		})
	}
	b := testBackend()
	r := actionRequest{Kind: "containers", Op: "custom:container-echo"}
	c, err := b.customFor(r, item{ID: "name; rm -rf /"})
	if err != nil || c.Argv[2] != "name; rm -rf /" || len(c.Argv) != 3 {
		t.Fatalf("custom argv escaping changed boundaries: %q %v", c.Argv, err)
	}
}
func TestSensitiveReadAuthenticationAndRedaction(t *testing.T) {
	b := testBackend()
	for _, path := range []string{"/api/inventory?kind=containers", "/api/detail/containers/demo-api?tab=env", "/api/config", "/api/jobs/x"} {
		w := call(b, "GET", path, "", "", "")
		if w.Code != 401 {
			t.Fatalf("unauthenticated read %s=%d", path, w.Code)
		}
	}
	for _, tab := range []string{"env", "inspect"} {
		w := apiCall(b, "GET", "/api/detail/containers/demo-api?tab="+tab, nil)
		if strings.Contains(w.Body.String(), "sample-not-a-credential") {
			t.Fatal("default detail leaked env value")
		}
		w = apiCall(b, "GET", "/api/detail/containers/demo-api?tab="+tab+"&reveal=1", nil)
		if !strings.Contains(w.Body.String(), "sample-not-a-credential") {
			t.Fatal("explicit reveal failed")
		}
	}
}
func TestCustomStatsPathHistoryWindowAndUnits(t *testing.T) {
	b := testBackend()
	b.config.Stats = []statGraph{{Label: "Bytes", StatPath: "ClientStats.MemoryStats.Usage", Min: 0, Max: 1000}, {Label: "Percent", StatPath: "DerivedStats.MemoryPercentage", Min: 0, Max: 100}}
	s := commands.ContainerStats{}
	s.MemoryStats.Usage = 500
	s.MemoryStats.Limit = 1000
	for n := 0; n < 65; n++ {
		text := b.statsText("sample", object{"ClientStats": s, "DerivedStats": object{"MemoryPercentage": s.CalculateContainerMemoryUsage()}}, 1)
		var v struct {
			Metrics []struct {
				Label, Value string
				Percent      float64
				History      []float64
			}
		}
		if err := json.Unmarshal([]byte(text), &v); err != nil {
			t.Fatal(err)
		}
		if v.Metrics[0].Value != "500.00" || v.Metrics[1].Value != "50.00" || v.Metrics[0].Percent != 50 {
			t.Fatal("bytes and percentage confused")
		}
		if n == 64 && len(v.Metrics[0].History) != 65 {
			t.Fatal("samples inside configured duration must be retained")
		}
	}
	for key, times := range b.statsTimes {
		for n := range times {
			times[n] = time.Now().Add(-3 * time.Minute)
		}
		b.statsTimes[key] = times
	}
	text := b.statsText("sample", object{"ClientStats": s, "DerivedStats": object{"MemoryPercentage": 50}}, 1)
	var result struct{ Metrics []struct{ History []float64 } }
	_ = json.Unmarshal([]byte(text), &result)
	if len(result.Metrics[0].History) != 1 {
		t.Fatal("expired graph samples retained")
	}
}
func TestConfigValidationAndJobCompletion(t *testing.T) {
	for _, raw := range []string{`{"unknown":1}`, `{"projects":[{"id":"x","path":"relative","files":[]}]}`, `{"commands":[{"id":"x","label":"x","scope":"bogus","argv":["echo"]}]}`, `{"stats":[{"label":"CPU","statPath":"DerivedStats.CPUPercentage","min":10,"max":0}]}`} {
		if _, err := parseConfig([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	b := testBackend()
	p, _, err := b.preview(context.Background(), actionRequest{Kind: "commands", IDs: []string{"hello"}, Op: "run"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := b.startJob(p)
	if err != nil {
		t.Fatal(err)
	}
	for end := time.Now().Add(time.Second); time.Now().Before(end); {
		w := apiCall(b, "GET", "/api/jobs/"+id, nil)
		var j job
		_ = json.Unmarshal(w.Body.Bytes(), &j)
		if j.Status == "succeeded" {
			if j.Count != 1 || j.Text == "" {
				t.Fatal("incomplete job result")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job did not complete")
}
func TestDashboardPollingDoesNotInspectEveryContainer(t *testing.T) {
	requests := map[string]int{}
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/v1.48")
		requests[path]++
		w.Header().Set("Content-Type", "application/json")
		switch path {
		case "/containers/json":
			_, _ = fmt.Fprint(w, `[{"Id":"sample","Names":["/sample"],"State":"exited","Status":"Exited (7) yesterday","Ports":[],"Labels":{}}]`)
		case "/system/df":
			_, _ = fmt.Fprint(w, `{"Images":[],"Volumes":[]}`)
		case "/networks":
			_, _ = fmt.Fprint(w, `[]`)
		default:
			t.Errorf("unexpected expensive polling request %s", path)
			w.WriteHeader(404)
		}
	}))
	defer daemon.Close()
	cli, err := client.NewClientWithOpts(client.WithHost(daemon.URL), client.WithVersion("1.48"))
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	b := &backend{docker: &commands.DockerCommand{Client: cli, Log: logrus.NewEntry(logrus.New())}}
	for n := 0; n < 5; n++ {
		if _, err = b.dashboard(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	for path, count := range requests {
		if count != 1 {
			t.Errorf("%s called %d times; want cached1", path, count)
		}
	}
}

func TestConfirmedCommandsFreezeConfiguration(t *testing.T) {
	b := testBackend()
	b.demo = false
	b.config.Terminal = []string{"original-terminal", "-e"}
	b.config.Commands = []customCommand{{ID: "safe", Label: "Safe", Scope: "commands", Argv: []string{"printf", "original"}, Cwd: "/tmp", Terminal: true}}
	p, _, err := b.preview(context.Background(), actionRequest{Kind: "commands", IDs: []string{"safe"}, Op: "run"})
	if err != nil {
		t.Fatal(err)
	}
	b.config.Commands[0].Argv = []string{"touch", "/tmp/unconfirmed"}
	b.config.Terminal = []string{"unconfirmed-terminal"}
	command := p.Commands["safe"]
	if command.Argv[1] != "original" || command.TerminalArgv[0] != "original-terminal" || command.Cwd != "/tmp" {
		t.Fatal("plan aliases mutable configuration")
	}
	b.runner = func(ctx context.Context, cwd string, argv []string, terminal bool) (string, error) {
		if !reflect.DeepEqual(argv, []string{"printf", "original"}) || cwd != "/tmp" || !terminal {
			t.Fatalf("unconfirmed command executed: %q", argv)
		}
		return "original", nil
	}
	if _, _, err = b.execute(context.Background(), p); err != nil {
		t.Fatal(err)
	}
}
func TestCacheInvalidationRejectsInflightStaleWrite(t *testing.T) {
	b := testBackend()
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_, _ = b.cached("probe", time.Minute, func() (any, error) { close(started); <-release; return "old", nil })
	}()
	<-started
	b.invalidate()
	close(release)
	<-done
	v, err := b.cached("probe", time.Minute, func() (any, error) { return "new", nil })
	if err != nil || v != "new" {
		t.Fatalf("stale cache repopulated: %v %v", v, err)
	}
}
func TestCustomTemplatesHaveNoExecutableMethodsAndFilterServices(t *testing.T) {
	b := testBackend()
	b.config.Commands = []customCommand{{ID: "templated", Label: "Template", Scope: "services", Argv: []string{"{{ .DockerCompose }}", "exec", "{{ .Service.Name }}", "printf", "{{ .Container.ID }}"}, ServiceNames: []string{"api"}}}
	i := item{ID: "kodus/api", Name: "api", Project: "kodus", Service: "api"}
	c, err := b.customFor(actionRequest{Kind: "services", Op: "custom:templated"}, i)
	if err != nil {
		t.Fatal(err)
	}
	if c.Argv[0] != "docker" || c.Argv[1] != "compose" || c.Argv[len(c.Argv)-3] != "api" {
		t.Fatalf("incorrect safe template: %q", c.Argv)
	}
	i.Service = "worker"
	if _, err = b.customFor(actionRequest{Kind: "services", Op: "custom:templated"}, i); err == nil {
		t.Fatal("serviceNames filter bypassed")
	}
	i.Service = "api"
	b.config.Commands[0].Argv = []string{"{{ .Container.Start }}"}
	if _, err = b.customFor(actionRequest{Kind: "services", Op: "custom:templated"}, i); err == nil {
		t.Fatal("templates must not expose Docker methods")
	}
}
func TestCommandInventoryMasksArguments(t *testing.T) {
	b := testBackend()
	b.config.Commands[0].Argv = []string{"echo", "private-value"}
	w := apiCall(b, "GET", "/api/inventory?kind=commands", nil)
	if strings.Contains(w.Body.String(), "private-value") {
		t.Fatal("command inventory revealed argv")
	}
}

func TestDriverOptionsMaskedUntilReveal(t *testing.T) {
	for _, key := range []string{"Options", "DriverOpts", "driver_opts"} {
		t.Run(key, func(t *testing.T) {
			value := object{key: object{"o": "username=sample,password=private-credential", "device": "//sample/share"}}
			masked := pretty(redacted(value))
			if strings.Contains(masked, "private-credential") || strings.Contains(masked, "//sample/share") {
				t.Fatalf("driver option disclosed: %s", masked)
			}
			if !strings.Contains(pretty(value), "private-credential") {
				t.Fatal("redaction mutated reveal source")
			}
		})
	}
}

func TestLogsAutoscaleAndOperationTemplateConfiguration(t *testing.T) {
	c, err := parseConfig([]byte(`{"logs":{"since":"15m","tail":45,"timestamps":true},"statsMaxDuration":"30s","stats":[{"label":"CPU","statPath":"DerivedStats.CPUPercentage","min":null,"max":null}],"commandTemplates":{"containers.stop":["printf","{{ .Container.ID }}"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	b := testBackend()
	b.config = c
	args := b.logFlags()
	if !reflect.DeepEqual(args, []string{"--tail", "45", "--since", "15m", "--timestamps"}) {
		t.Fatalf("incorrect log flags: %q", args)
	}
	for _, value := range []float64{10, 20} {
		b.statsText("x", object{"DerivedStats": object{"CPUPercentage": value}}, value)
	}
	text := b.statsText("x", object{"DerivedStats": object{"CPUPercentage": 15}}, 15)
	var output struct {
		Metrics []struct {
			Percent float64
			History []float64
		}
	}
	_ = json.Unmarshal([]byte(text), &output)
	if output.Metrics[0].Percent != 50 || !reflect.DeepEqual(output.Metrics[0].History, []float64{0, 100, 50}) {
		t.Fatalf("autoscale failed: %s", text)
	}
	p, _, err := b.preview(context.Background(), actionRequest{Kind: "containers", IDs: []string{"demo-api"}, Op: "stop"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Commands["demo-api"].Argv, []string{"printf", "demo-api"}) {
		t.Fatalf("override not resolved: %+v", p.Commands)
	}
	b.config.CommandTemplates["containers.stop"] = []string{"unconfirmed"}
	if p.Commands["demo-api"].Argv[0] != "printf" {
		t.Fatal("operation override not frozen")
	}
}
func TestImageParentClosureAndRevalidation(t *testing.T) {
	tagged := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch strings.TrimPrefix(r.URL.Path, "/v1.48") {
		case "/containers/json":
			_, _ = fmt.Fprint(w, `[]`)
		case "/images/json":
			tags := `[]`
			if tagged {
				tags = `["keep:latest"]`
			}
			_, _ = fmt.Fprintf(w, `[{"Id":"child","ParentId":"base","RepoTags":["child:latest"],"Created":1},{"Id":"base","ParentId":"tagged","RepoTags":%s,"Created":2},{"Id":"tagged","RepoTags":["tagged:latest"],"Created":3}]`, tags)
		default:
			w.WriteHeader(404)
		}
	}))
	defer s.Close()
	cli, err := client.NewClientWithOpts(client.WithHost(s.URL), client.WithVersion("1.48"))
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	b := &backend{docker: &commands.DockerCommand{Client: cli, Log: logrus.NewEntry(logrus.New())}, config: emptyConfig()}
	p, _, err := b.preview(context.Background(), actionRequest{Kind: "images", IDs: []string{"child"}, Op: "remove"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Targets) != 2 || p.Targets[0].ID != "child" || p.Targets[1].ID != "base" {
		t.Fatalf("wrong explicit image closure: %+v", p.Targets)
	}
	tagged = true
	if b.validatePlan(context.Background(), p) == nil {
		t.Fatal("newly tagged automatic parent accepted")
	}
	p, _, err = b.preview(context.Background(), actionRequest{Kind: "images", IDs: []string{"child"}, Op: "remove"})
	if err != nil || len(p.Targets) != 1 {
		t.Fatalf("tagged parent included: %+v %v", p.Targets, err)
	}
}
