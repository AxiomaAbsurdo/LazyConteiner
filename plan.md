# LazyConteiner first iteration

## Background

Build a Debian Linux GUI with expo/exact, using Lazydocker's Docker core behind a local bridge. Follow `elements-wireframe.md` and the supplied design reference. Keep iterations locally runnable.

## Setup

1. Preserve the supplied design assets and initialize Git.
2. Verify GitHub authentication and create a private project repository.
3. Pin and verify the upstream Exact runtime and Lazydocker integration.

## Tasks and verification

1. Runtime worker: provide reproducible setup and native Linux launch commands; verify Contract compilation and runtime startup.
2. Backend worker: reuse Lazydocker core for Docker inventory and explicit lifecycle actions; verify tests without mutating existing workloads.
3. UI worker: implement the dashboard, selection, filters, alerts, and resource panels; verify supported Exact syntax and interaction tests.
4. Integration worker: exercise the UI/backend boundary and local startup; fix integration failures.
5. Review worker: check trust boundaries, destructive-action safeguards, and documented limitations.
6. Supervisor: run final checks, commit, push, and document the local test command.

## Acceptance

- Actual Exact application runs on Debian using the supported Linux host.
- Dashboard renders real Docker data or a clearly labeled explicit demo mode.
- Existing Docker workloads remain untouched during verification.
- Documentation distinguishes implemented features from deferred wireframe controls.
- GitHub repository contains the tested source, pinned dependencies, and run instructions.

## Standing workflow

Apply Karpathy guidelines and relevant installed Matt Pocock skills to coding. Use Ralph-style concrete tasks and verification. Consult Firstmate for crew coordination, codebase-memory for structural discovery, Obscura for applicable browser tasks, and TypeSafe for evidence-based semantic decisions. Five worker tasks run in waves because this session permits three concurrent workers.

## First iteration verification

- Five worker tasks completed: runtime, backend, frontend, integration, and security review.
- `make test` passes from the persistent local checkout without an `EXACT` override: backend tests, five data-adapter tests, and four native interaction tests.
- `go test -race ./...` passes 18 backend checks, including fake Docker lifecycle calls and remote endpoint rejection.
- Native Wayland rendering was verified inside KWin on the current X11 desktop. The launcher defaults to 1440×900.
- Read-only live Docker inventory succeeded. Lifecycle integration was exercised only against demo fixtures.
- Port 17878 avoids the existing service on 8787. Launcher shutdown removes its backend process and session token.
- Secondary resource pages, metrics, events, logs, and destructive cleanup remain explicitly deferred in README and UI.
