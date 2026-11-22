# Full Lazydocker frontend implementation

## Current scope

Fix physical mouse clicks, expose the pinned Lazydocker feature inventory through Exact, and validate responsive layout and performance. Semantic driver taps do not prove native input delivery.

## Public interfaces under test

1. Loopback API: Lazydocker adapters against fake Docker responses and isolated command runners; denied requests and destructive-action confirmation.
2. TypeScript adapter: typed responses, cached filtering/selection, errors, and operation transitions.
3. Native window: physical pointer events, keyboard input, dialogs, scrolling, and resizing.
4. End-to-end demo: identical frontend/API with isolated fixtures; no real workload mutations.
5. Human validation: explicit pending/pass/fail status recorded from user observations.

## Verification status

Implemented the source-backed 57-action / 21-tab inventory. Final automated results: 153 Go race cases, 16 adapter tests, 16 authenticated native tests, and 36 live disposable Docker checks pass. Physical X11 input regression passes when run alone. User confirmed clicks and layout work on 2026-11-22. Individual Docker operations and terminal interaction remain pending; docs/FEATURES.md records the precise evidence.

## Completed implementation steps

- Diagnose pointer delivery; add a reproducible regression and tracked runtime patch if required.
- Inventory upstream menus, detail views, commands, Compose behavior, and settings before claiming parity.
- Implement inventory features with exact-target confirmations for destructive operations.
- Use supported Exact layout primitives. Check widths 640/768/1024/1440, short heights, and resize transitions.
- Remove network requests from local filtering/selection; load expensive details on demand and bound output.
- Measure API latency, payload, startup, and resource use with limitations recorded.
- Run automated and physical-input acceptance. Human validation remains pending until the user reports results.

Evidence lives in `docs/FEATURES.md` and `docs/PERFORMANCE.md`. A button or route alone is not feature validation.

## Previous iteration

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
