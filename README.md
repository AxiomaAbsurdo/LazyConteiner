# LazyConteiner

A native Debian Linux Docker dashboard built with [Expo Exact](https://github.com/expo/exact) and [Lazydocker](https://github.com/jesseduffield/lazydocker)'s Go core. The first iteration implements the Fleet dashboard from [elements-wireframe.md](elements-wireframe.md).

![Native Debian dashboard running with isolated demo data](docs/dashboard.png)

## Run locally

Prerequisites: Go 1.23.6 or newer, Bun 1.4.2 or newer, Rust through rustup, Git, Python 3, and a working Wayland desktop. On this computer's X11 desktop, the launcher uses the existing `kwin_wayland` compositor in a nested window. Docker is required only for live mode.

```sh
make setup             # one-time Exact toolchain/dependency setup
make demo              # native GUI with isolated sample data; no Docker access
make run               # native GUI with real Docker inventory
```

The first build downloads and compiles Exact and its Rust dependencies. Later builds reuse the local cache. Close the window or press Ctrl+C to stop the app and its backend.

If using an already prepared Exact checkout, point to it explicitly:

```sh
EXACT=/path/to/exact make demo
```

Setup pins Exact to `d7b6871218430ed57c5a0fbf34860b28f523d5df`. Lazydocker is pinned by `backend/go.mod` and `backend/go.sum`. Runtime downloads, builds, logs, and session credentials stay outside Git.

## First iteration

- Native sidebar, dashboard cards, Compose groups, container rows, search, filters, selection, and alerts.
- Real Docker inventory using Lazydocker's `DockerCommand.GetContainers` and Docker SDK resource queries.
- Start, stop, and restart through Lazydocker container methods, with confirmation and session authentication.
- Two-second refresh with pause/resume, connection errors, and explicit demo labeling.
- Cleanup inventory preview without deletion.

Other resource pages are labeled as planned. Host CPU/memory metrics, per-container statistics, event collection, logs, terminals, stack-wide Compose operations, remote hosts, Podman, persistent alert dismissal, and actual pruning are not implemented yet. Cleanup preview does not claim to reproduce Docker prune eligibility.

## Development checks

```sh
make check             # Contract validation
make test              # backend and native Exact interaction tests
make build             # Go bridge and native Linux binary
cd backend && go test -race ./...
```

`fleet/app.contract` owns the UI; `fleet/app.ts` adapts the loopback JSON API. `backend/main.go` owns Docker access. `scripts/dev.py` starts both processes and cleans up session credentials. `plan.md` records the first iteration's acceptance checks.

Exact also provides a web development host via `./scripts/exact-run.sh web`; this is an optional layout preview, not the Debian native app. The bridge deliberately rejects browser requests, so Docker data and lifecycle actions require the native app.

## Local access and safety

The bridge binds to `127.0.0.1:17878`. The launcher creates a random session token in an owner-readable file under `.runtime/data`, passes it to the backend, and removes it on exit. Start/stop/restart require this token. The backend rejects untrusted browser origins and unexpected Host headers; it exposes no delete, prune, shell, or exec endpoint.

Docker socket access grants root-equivalent control. Run as your normal user with an appropriately configured Docker socket; rootless Docker is supported with `DOCKER_HOST=unix:///path/to/docker.sock`. Do not launch the app with sudo. This iteration accepts only local Unix sockets and rejects TCP, HTTP, and SSH endpoints before connecting.

Tests use fake Docker responses and explicit demo fixtures. Verification must not stop, restart, or remove existing user containers.

## Design references

`BetaB.dc.html` is the original standalone artboard. `support.js` and `vendor/react*.js` belong to that design preview, not the application runtime. Serve this folder with `python3 -m http.server` to view the artboard. The supplied ZIP is retained unchanged.

Lazydocker's MIT notice is preserved in `backend/LICENSE.lazydocker`. See [WORKFLOW.md](WORKFLOW.md) for standing development skill preferences.
