# Lazydocker parity and validation checklist

Reference: Lazydocker `596fd59df6b088bb1e38c5bb08cd4fbf28b3ebfb`. The [backend inventory](FEATURES-backend.md) lists every implemented action, detail tab, functional setting, upstream source and named test. It contains 57 action IDs and 21 detail tabs. Human validation is recorded only from actual user feedback. On 2026-11-22 the user confirmed: “Clicks and layout work.” This covers the updated demo’s clicks and layout, not every Docker operation.

## Verification recorded on 2026-11-22

| Gate | Automated evidence | Human validation |
| --- | --- | --- |
| Physical pointer delivery | Direct X11 host; actual XTest clicks, typing, wheel, window movement and resizing pass. `make test-input`. | **Pass** — user confirmed fixed-build clicks on 2026-11-22 |
| Native GUI behavior | `make test`: 16 native tests, 16 Bun adapter tests; all pass. | Pending |
| Backend actions and safeguards | `go test -race ./...`: 153 cases/subtests pass. | Pending |
| Real Docker and Compose | `make test-live`: 36 disposable-resource checks pass. | Pending |
| Responsive layout | Native assertions at 320, 360, 640, 768, 1024 and 1440 px; 600×480 short dialog. | **Pass** — user confirmed layout; individual viewport sizes not reported |
| Performance | Warm dashboard median 0.30 ms, p95 1.46 ms; cold response 1093.01 ms. See [measurement limits](PERFORMANCE.md). | Pending |

Full-app physical clicks also switched Dashboard/Containers before and after resizing. Reviewed screenshots: [desktop dashboard](dashboard.png), [containers](containers.png), [360 px window](responsive.png). These are agent-observed results, not human sign-off.

Physical tests must run alone: another native test window can steal focus and invalidate the result. Semantic Exact taps do not prove physical input delivery. Tests seed private session credentials in isolated stores; they do not bypass authentication.

## Frontend feature pass

| Feature family | Frontend entry point | Automated evidence | Human |
| --- | --- | --- | --- |
| Dashboard, filter, selection, pause and alerts | Dashboard sidebar page | Native navigation, filter, alerts and confirmation tests | Pending |
| Container lifecycle, removal, terminal and browser actions | Containers: select resource, choose action; expand destructive actions | Backend matrix tests every advertised action; native action preview/confirm/cancel; live lifecycle | Pending |
| Compose project operations | Stacks: select project, choose action | Backend argv/action matrix; native navigation; live up/down | Pending |
| Compose service operations | Compose services: select service, choose action | Backend argv/action matrix; native details; live start/stop/restart/pause/unpause/recreate | Pending |
| Bulk operations | Resource page: Select all visible or individual checkboxes, then action | Native six-target preview, filtered one-target selection, cancellation and Clear | Pending |
| Image removal/prune and history | Images: choose image, history or destructive actions | SDK removal flags and parent closure tests; native history; live history/removal | Pending |
| Volumes and networks | Corresponding resource page, inspect/destructive actions | SDK/demo matrix; native cancel preserves target; live inspect/removal | Pending |
| Logs, stats, environment, inspect and top | Containers/Compose services: detail tabs | Backend detail matrix; native stats and Reveal; live Docker detail checks | Pending |
| Project/custom-command/host configuration reveal | Config detail tab: Reveal | Backend masking; native project Reveal; adapter closed-shape tests | Pending |
| Custom commands | Custom commands for global commands; resource actions for scoped commands | Native edit/save/run; live global echo; argv/template/filter security tests | Pending |
| Host info, disk and events | Host & systemd / Events | Backend detail matrix and native page tests | Pending |
| Settings and functional configuration | Settings: Edit, Save, Confirm | Config validation/atomic-save tests; native settings/custom command test | Pending |
| Responsive dialogs and layout | Resize any page; open confirmation | Native viewport and modal-bound assertions; physical resize regression | **Pass** for overall layout; exhaustive viewport pass Pending |
| Failure and retry states | Resource, detail, config and job panels | Adapter extra-field projection, invalidation race and transient-job retry tests | Pending |

“Select all visible” selects the current filtered inventory. Clear the search first to operate on every listed resource. Confirmations show exact IDs/names; cancellation executes nothing. Individual action and detail evidence is enumerated in [FEATURES-backend.md](FEATURES-backend.md), including prerequisites and limitations.

## Real daemon coverage

`scripts/live_check.py` creates uniquely named/labeled disposable resources from the pre-existing `alpine:3.20` image and cleans up only those resources. The 36 checks cover inventory/details, logs, stats, top, masked/revealed environment, container lifecycle/removal, custom echo, Compose project/service lifecycle/recreate/config, image history/removal, volume removal and network removal.

Global prune was **not run** against the user's daemon. Eligibility, frozen targets, state changes, exact image parents and removal flags are covered by isolated demo/fake-SDK tests. Exec, attach, follow logs, attached-up and browser launching have command-generation coverage; actual interactive terminal/browser behavior still needs a desktop human pass. Build/pull and workstation-specific Compose recipes need the appropriate projects/images/network and their own human checks. Passing routing tests is not a claim that every external executable succeeded on this workstation.

## Human validation runbook

Start with `make demo`. For each row above and each action/detail in the backend matrix, record **Pass**, **Fail**, or **Not tested**, window size, and reproduction steps. Do not replace Pending using automated results.

1. Click Dashboard, Containers, Images and Settings with the mouse. Move the window between monitors and repeat.
2. Type a filter, toggle selection, use Select all visible, clear selection, open/cancel/confirm a demo action. Verify the background remains clickable afterward.
3. Resize through 1440×900, 1024×768, 768×768, 640×768, 360×640 and 320×640. Scroll every panel; confirm controls do not overlap and dialog actions remain reachable at 600×480.
4. Visit every detail tab, Reveal protected content intentionally, edit/save settings, and run a demo custom command.
5. Use `make run` with deliberately chosen disposable resources. Verify lifecycle, Compose, image/volume/network operations individually. Check terminal typing/resizing/exit and browser URLs. Review exact targets before destructive confirmation.
6. Record perceived responsiveness and any freeze, stale state, clipping or focus problem. Keep destructive global cleanup Not tested unless intentionally validated in an isolated Docker environment.

Human result record, 2026-11-22: **Clicks and layout work** (direct user response after the updated demo was opened). Per-feature Docker execution, interactive terminals, performance and exhaustive viewport checks remain **Pending**. This response is not a blanket feature sign-off.
