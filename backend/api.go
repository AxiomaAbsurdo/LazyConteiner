package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

type actionRequest struct {
	Kind string   `json:"kind"`
	IDs  []string `json:"ids"`
	Op   string   `json:"op"`
	Args []string `json:"args"`
}

var ansiEscape = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")

type actionPlan struct {
	Request       actionRequest
	Targets       []item
	Expires       time.Time
	ConfigDigest  string
	ProjectProofs map[string]string
	Commands      map[string]resolvedCommand
}
type job struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Text   string `json:"text"`
	Error  string `json:"error"`
	Count  int    `json:"count"`
}

func (b *backend) authorized(r *http.Request) bool {
	return b.token != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-LazyContainer-Token")), []byte(b.token)) == 1
}
func decodeBody(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 512*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("expected one JSON object")
	}
	return nil
}
func failure(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, object{"error": err.Error()})
}
func (b *backend) registerAPI(mux *http.ServeMux) {
	secure := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !b.authorized(r) {
				failure(w, 401, fmt.Errorf("session token required"))
				return
			}
			handler(w, r)
		}
	}
	mux.HandleFunc("GET /api/inventory", secure(func(w http.ResponseWriter, r *http.Request) {
		kind := r.URL.Query().Get("kind")
		items, err := b.inventory(r.Context(), kind)
		if err != nil {
			failure(w, 502, err)
			return
		}
		views := []object{}
		for _, id := range viewIDs[kind] {
			views = append(views, object{"id": id, "label": id})
		}
		writeJSON(w, 200, object{"kind": kind, "items": items, "actions": b.capabilities(kind), "views": views, "mode": b.mode(), "error": ""})
	}))
	mux.HandleFunc("GET /api/detail/{kind}/{id}", secure(func(w http.ResponseWriter, r *http.Request) {
		kind, id, tab := r.PathValue("kind"), r.PathValue("id"), r.URL.Query().Get("tab")
		value, err := b.detail(r.Context(), kind, id, tab, r.URL.Query().Get("reveal") == "1")
		if err != nil {
			failure(w, 502, err)
			return
		}
		value = ansiEscape.ReplaceAllString(value, "")
		metrics, points := any([]object{}), any([]float64{})
		if tab == "stats" {
			var stats object
			if json.Unmarshal([]byte(value), &stats) == nil {
				if v, ok := stats["metrics"]; ok {
					metrics = v
				}
				if v, ok := stats["points"]; ok {
					points = v
				}
			}
		}
		writeJSON(w, 200, object{"title": id + " · " + tab, "text": value, "mode": b.mode(), "metrics": metrics, "points": points})
	}))
	mux.HandleFunc("POST /api/action/preview", secure(func(w http.ResponseWriter, r *http.Request) {
		var request actionRequest
		if err := decodeBody(w, r, &request); err != nil {
			failure(w, 400, err)
			return
		}
		plan, summary, err := b.preview(r.Context(), request)
		if err != nil {
			failure(w, 400, err)
			return
		}
		token, err := randomID()
		if err != nil {
			failure(w, 500, err)
			return
		}
		b.mu.Lock()
		if b.plans == nil {
			b.plans = map[string]actionPlan{}
		}
		for id, p := range b.plans {
			if time.Now().After(p.Expires) {
				delete(b.plans, id)
			}
		}
		if len(b.plans) >= 128 {
			b.mu.Unlock()
			failure(w, 429, fmt.Errorf("too many pending confirmations"))
			return
		}
		b.plans[token] = plan
		b.mu.Unlock()
		writeJSON(w, 200, object{"confirmation": token, "summary": summary, "targets": plan.Targets, "expiresAt": plan.Expires.UTC().Format(time.RFC3339)})
	}))
	mux.HandleFunc("POST /api/action", secure(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Confirmation string `json:"confirmation"`
		}
		if err := decodeBody(w, r, &request); err != nil {
			failure(w, 400, err)
			return
		}
		b.mu.Lock()
		plan, ok := b.plans[request.Confirmation]
		delete(b.plans, request.Confirmation)
		b.mu.Unlock()
		if !ok || time.Now().After(plan.Expires) {
			failure(w, 409, fmt.Errorf("confirmation expired or already used; preview again"))
			return
		}
		if err := b.validatePlan(r.Context(), plan); err != nil {
			failure(w, 409, err)
			return
		}
		if !b.demo || plan.Request.Kind == "services" || plan.Request.Kind == "projects" || plan.Request.Kind == "commands" || strings.HasPrefix(plan.Request.Op, "custom:") || isTerminal(plan.Request.Op) {
			id, err := b.startJob(plan)
			if err != nil {
				failure(w, 429, err)
				return
			}
			writeJSON(w, 202, object{"count": 0, "mode": b.mode(), "text": "Operation started. Follow job status for completion.", "jobId": id})
			return
		}
		count, text, err := b.execute(r.Context(), plan)
		if err != nil {
			writeJSON(w, 502, object{"count": count, "error": err.Error(), "text": text, "jobId": "", "mode": b.mode()})
			return
		}
		writeJSON(w, 200, object{"count": count, "mode": b.mode(), "text": text, "jobId": ""})
	}))
	mux.HandleFunc("GET /api/jobs/{id}", secure(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		j, ok := b.jobs[r.PathValue("id")]
		if !ok {
			failure(w, 404, fmt.Errorf("job not found"))
			return
		}
		writeJSON(w, 200, j)
	}))
	mux.HandleFunc("GET /api/config", secure(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		data, _ := json.MarshalIndent(b.config, "", "  ")
		writeJSON(w, 200, object{"text": string(data), "path": b.configPath})
	}))
	mux.HandleFunc("POST /api/config", secure(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
		}
		if err := decodeBody(w, r, &body); err != nil {
			failure(w, 400, err)
			return
		}
		if err := b.saveConfig([]byte(body.Text)); err != nil {
			failure(w, 400, err)
			return
		}
		writeJSON(w, 200, object{"ok": true, "text": "Configuration saved. Pending action confirmations invalidated."})
	}))
}
func randomID() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
func isTerminal(op string) bool {
	return op == "exec" || op == "attach" || op == "logs-follow" || op == "up-attached" || op == "open-browser"
}

func (b *backend) preview(ctx context.Context, r actionRequest) (actionPlan, string, error) {
	p := actionPlan{Request: r, Expires: time.Now().Add(2 * time.Minute), Targets: []item{}, ProjectProofs: map[string]string{}, Commands: map[string]resolvedCommand{}}
	valid := false
	for _, c := range b.capabilities(r.Kind) {
		if c.ID == r.Op {
			valid = true
			break
		}
	}
	if !valid {
		return p, "", fmt.Errorf("unsupported action %s/%s", r.Kind, r.Op)
	}
	if len(r.IDs) > 500 || len(r.Args) > 128 {
		return p, "", fmt.Errorf("too many targets or arguments")
	}
	for _, arg := range r.Args {
		if strings.ContainsRune(arg, 0) || len(arg) > 16384 {
			return p, "", fmt.Errorf("invalid argument")
		}
	}
	if r.Op != "exec" && len(r.Args) > 0 {
		return p, "", fmt.Errorf("only exec accepts extra argv")
	}
	if r.Op == "exec" && len(r.Args) == 0 {
		p.Request.Args = []string{"/bin/sh"}
	}
	b.invalidate()
	items, err := b.inventory(ctx, r.Kind)
	if err != nil {
		return p, "", err
	}
	if strings.HasPrefix(r.Op, "prune") {
		p.Targets, err = b.pruneCandidates(ctx, r.Kind, r.Op)
		if err != nil {
			return p, "", err
		}
	} else {
		wanted := map[string]bool{}
		for _, id := range r.IDs {
			wanted[id] = true
		}
		for _, i := range items {
			if wanted[i.ID] {
				p.Targets = append(p.Targets, i)
				delete(wanted, i.ID)
			}
		}
		if len(wanted) > 0 {
			return p, "", fmt.Errorf("target no longer exists; refresh inventory")
		}
	}
	if len(p.Targets) == 0 {
		return p, "", fmt.Errorf("no eligible targets")
	}
	sort.Slice(p.Targets, func(i, j int) bool { return p.Targets[i].ID < p.Targets[j].ID })
	if r.Kind == "images" && strings.HasPrefix(r.Op, "remove") && !strings.Contains(r.Op, "no-prune") {
		p.Targets, err = b.imageParentTargets(ctx, p.Targets)
		if err != nil {
			return p, "", err
		}
	}
	b.mu.Lock()
	p.ConfigDigest = configDigest(b.config)
	b.mu.Unlock()
	summary := fmt.Sprintf("%s %d %s. Mode: %s.\n", r.Op, len(p.Targets), r.Kind, b.mode())
	for _, i := range p.Targets {
		summary += i.Name + " [" + i.ID + "]\n"
		if r.Kind == "projects" || r.Kind == "services" {
			project, err := b.project(ctx, i.Project)
			if err != nil {
				return p, "", err
			}
			proof, err := b.projectProof(project)
			if err != nil {
				return p, "", err
			}
			p.ProjectProofs[i.Project] = proof
			if !strings.HasPrefix(r.Op, "custom:") && (!isTerminal(r.Op) || r.Op == "logs-follow" || r.Op == "up-attached") {
				argv, e := b.composeArgs(project, r.Op, i.Service)
				if e != nil {
					return p, "", e
				}
				summary += "Command: " + strings.Join(argv, " ") + "\n"
			}
		}
		if r.Kind == "commands" || strings.HasPrefix(r.Op, "custom:") {
			c, e := b.customFor(r, i)
			if e != nil {
				return p, "", e
			}
			summary += "Command argv: " + pretty(c.Argv) + "\n"
		}
		command, e := b.resolveCommand(ctx, p.Request, i)
		if e != nil {
			return p, "", e
		}
		if command != nil {
			p.Commands[i.ID] = *command
			summary += "Exact argv: " + pretty(command.Argv) + "\n"
			if command.Terminal {
				summary += "Terminal argv prefix: " + pretty(command.TerminalArgv) + "\n"
			}
		}
	}
	if strings.Contains(r.Op, "volumes") {
		summary += "WARNING: volume removal permanently deletes stored data.\n"
	}
	if strings.Contains(r.Op, "force") || r.Op == "kill" {
		summary += "WARNING: forced termination can lose unsaved data.\n"
	}
	if isTerminal(r.Op) {
		summary += "A native terminal/window opens. Terminal provides interactive input and resizing.\n"
	}
	return p, summary, nil
}
func (b *backend) validatePlan(ctx context.Context, p actionPlan) error {
	b.mu.Lock()
	digest := configDigest(b.config)
	b.mu.Unlock()
	if digest != p.ConfigDigest {
		return fmt.Errorf("configuration changed; preview again")
	}
	b.invalidate()
	items, err := b.inventory(ctx, p.Request.Kind)
	if err != nil {
		return err
	}
	if strings.HasPrefix(p.Request.Op, "prune") {
		items, err = b.pruneCandidates(ctx, p.Request.Kind, p.Request.Op)
		if err != nil {
			return err
		}
	}
	if p.Request.Kind == "images" && strings.HasPrefix(p.Request.Op, "remove") && !strings.Contains(p.Request.Op, "no-prune") {
		selected := []item{}
		wanted := map[string]bool{}
		for _, id := range p.Request.IDs {
			wanted[id] = true
		}
		for _, i := range items {
			if wanted[i.ID] {
				selected = append(selected, i)
			}
		}
		items, err = b.imageParentTargets(ctx, selected)
		if err != nil {
			return err
		}
	}
	current := map[string]item{}
	for _, i := range items {
		current[i.ID] = i
	}
	for _, i := range p.Targets {
		now, ok := current[i.ID]
		if !ok || now.Version != i.Version {
			return fmt.Errorf("target %s changed or disappeared; preview again", i.Name)
		}
	}
	for id, proof := range p.ProjectProofs {
		project, err := b.project(ctx, id)
		if err != nil {
			return err
		}
		now, err := b.projectProof(project)
		if err != nil {
			return err
		}
		if now != proof {
			return fmt.Errorf("Compose files changed; preview again")
		}
	}
	return nil
}
func (b *backend) startJob(plan actionPlan) (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	if b.jobs == nil {
		b.jobs = map[string]*job{}
	}
	if len(b.jobs) >= 64 {
		for key, j := range b.jobs {
			if j.Status != "running" {
				delete(b.jobs, key)
			}
		}
	}
	if len(b.jobs) >= 64 {
		b.mu.Unlock()
		return "", fmt.Errorf("too many active jobs")
	}
	b.jobs[id] = &job{ID: id, Status: "running"}
	b.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		count, text, err := b.execute(ctx, plan)
		b.mu.Lock()
		defer b.mu.Unlock()
		j := b.jobs[id]
		j.Count, j.Text = count, text
		j.Status = "succeeded"
		if err != nil {
			j.Status, j.Error = "failed", err.Error()
		}
	}()
	return id, nil
}
