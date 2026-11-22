# Performance measurements

## Method

Run `python3 scripts/benchmark.py` against the local dashboard endpoint. The benchmark makes 20 sequential read-only requests and reports aggregate latency and payload size. It does not mutate Docker or print container names, environment variables, or credentials.

The first response is reported separately. The median and p95 include all requests. Cached responses and freshly collected inventory must be distinguished when comparing results. These measurements describe this Debian machine and its local Docker daemon, not a cross-machine guarantee.

## Baseline — initial iteration

Measured on 2026-11-22 using commit `5e1bf13`, a separate read-only backend on port 17879, and 20 local containers:

| Measure | Result |
| --- | ---: |
| First response | 1479.22 ms |
| Median | 1061.85 ms |
| p95 | 1147.29 ms |
| Maximum | 1479.22 ms |
| Mean response size | 6970 bytes |

The initial frontend also refreshed the dashboard after checkbox/filter operations. A roughly one-second inventory request on these interactions is unacceptable for local UI state.

## Acceptance checks

- Search, selection, sidebar navigation, and panel expansion update locally without a dashboard network request.
- Dashboard inventory avoids per-container inspection; details are fetched only for the selected resource.
- Expensive disk inventory is cached with a documented lifetime and explicit refresh behavior.
- Logs and command output are bounded; repeated polling does not create unbounded processes or streams.
- Stats polling runs only for the visible stats view and stops when leaving it.
- Record both fresh and warm response latency after implementation; target warm dashboard p95 below 100 ms on this machine.
- Verify physical pointer feedback and responsive resize using the actual native window. Semantic-driver timing alone is insufficient.

## Updated dashboard — warm cache

The same 20-request read-only benchmark on the same 20-container daemon produced:

| Measure | Result |
| --- | ---: |
| First response (cold) | 1093.01 ms |
| Median | 0.30 ms |
| p95 | 1.46 ms |
| Maximum | 1093.01 ms |
| Mean response size | 7142 bytes |

The warm improvement comes from caching; cold Docker disk inventory remains roughly one second. This is not a claim that Docker itself became faster. Local filter and selection tests separately assert that those interactions reuse the snapshot without additional fetches.

## Native input and startup

The direct X11 regression uses physical XTest events and pixels from the real native window, not semantic driver taps. It verifies clicking, typing, wheel scrolling, movement between monitor coordinates, and resizing.

- Fixture window ready within 297 ms, including discovery and a deliberate 150 ms activation wait.
- Click result observed within 120 ms, including 120 ms deliberate input pacing. This is an upper bound, not a precise render-latency measurement.
- Fixture RSS: 238576 KiB. This is the Exact native process; a separate nested KWin compositor is no longer launched.

The tiny input fixture is not a benchmark of the complete dashboard. Its purpose is to prove that physical input reaches the same native executable correctly.

## Final physical-input rerun

The isolated native fixture passed clicks, keyboard, wheel, move and resize. Readiness was at most 278 ms including discovery/activation; click response at most 120 ms including deliberate input pacing; RSS 239092 KiB. These are fixture measurements, not full-dashboard startup or an interaction percentile. Run physical tests separately from other GUI tests to prevent focus interference.
