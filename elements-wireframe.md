# Fleet Dashboard · Elements Wireframe for Exact

Source design: `★ Selected · Fleet dashboard (bar: Portainer)` (BetaB.dc.html on the design canvas).
Target runtime: [expo/exact](https://github.com/expo/exact). The view goes in Contract (`app.contract`). The data logic goes in TypeScript (`app.ts`) or a Rust data crate.

This file lists every element on that artboard. For each element it gives the Exact element to use, its attributes, bindings, events, `testId`, style, states and accessibility notes. Build it top to bottom.

---

## 0. Ground rules from Exact that shape this spec

* An app has `app.contract` (view, state, actions), `app.ts` (data: `appId`, `grants`, `answer`), `app.json` (manifest) and `app.test.contract` (tests).
* Contract has no JavaScript in the view, no I/O, no loops in actions and no `await`. Effects happen only in actions.
* State changes only inside an `action`. Data from the system comes in through a `resource`. Changes go out through `mutation` + `send`.
* Under a `for`, `state` belongs to each keyed row. This spec uses that for "expanded" flags on stack rows and alert cards.
* Events are attributes that name actions: `press`, `input`, `submit`, `change`.
* Loops are `for x in xs key=x.id`. The key is required.
* A `style` is a named set of CSS properties, applied with `class=`. Node attributes win. There is no cascade.
* Layout is CSS block, flex and grid. A bare node is `display: block`, `flex-direction: row`.
* Colors and fonts come from the platform unless the author sets them. The palette in section 9 is therefore optional. Ship v1 with platform defaults and add colors only where meaning needs them (status).
* `testId` is used by tests, agents and accessibility tools. Every interactive element below has one.

### Syntax marked "VERIFY"

The Exact README is pre-1.0 and does not show every form. Items tagged **VERIFY** are inferred, not copied from the README. Check them against the current repo before building:

* The exact body syntax of a `style` block.
* `and` / `or` in expressions (the README shows `not` and `!=`).
* `select` with `option` children and a `change` event.
* `svg` children (`path d=...`). The README lists full SVG support but shows no example.
* `disabled=` on `button`, and `title=` (hover tooltip) on any node.
* `position: absolute` for the badge dots.
* `task every(ms, action)` gated by state inside the action.

### Linux host caveat

The Exact Linux host paints to DRM/KMS or runs headless. The README says "Linux reads but does not edit". The search fields (T2.3, S1.4) may not accept typing on Linux yet. Keep them, but make every flow usable without typing (filters, buttons, keyboard actions).

---

## 1. File layout

```
fleet/
  app.contract          // root component Fleet + child components
  sidebar.contract      // component Sidebar, NavItem     (use … from "./sidebar.contract")
  dashboard.contract    // TotalsCard, AlertCard, StackGroup, ContainerRow, HostPanel, EventsPanel, CleanupPanel
  styles.contract       // all `style` blocks              (VERIFY: styles importable via use)
  app.ts                // sources + answer
  app.json
  app.test.contract
```

Generate the TypeScript types after each Contract change:

```
bun exact.mjs contract types app.contract -o app.contract.d.ts
```

---

## 2. Shapes (data contracts)

```contract
shape Container
  id: string            // docker container id (short)
  name: string
  stack: string         // compose project label, or "" for standalone
  state: string         // "running" | "exited" | "paused" | "created" | "restarting"
  exitCode: number      // -1 when running
  ports: string         // "127.0.0.1:8000→8000" or "—"
  cpu: string           // "0.01%"
  selected: bool        // UI selection, kept in app.ts store

shape Stack
  id: string            // "kodus", "openhands", "__standalone"
  name: string          // display name; "Standalone containers" for __standalone
  isCompose: bool
  running: number
  total: number
  status: string        // "Running" | "Partial" | "Stopped"
  containers: Container[]

shape AlertItem
  id: string
  name: string
  exitCode: number

shape Alert
  id: string            // "oom" | "crash" | …
  title: string         // "4 containers were killed (exit 137)"
  hint: string
  cta: string           // "Restart all 4" | "Remove all 5…"
  ctaAction: string     // "restart" | "remove"
  tone: string          // "warn" | "neutral"
  items: AlertItem[]

shape Totals
  containers: number
  running: number
  failed: number
  cleanExit: number
  images: number
  imagesSize: string
  imagesUnused: number
  volumes: number
  volumesUnused: number
  networks: number
  networksDetail: string   // "7 bridge · host · none"

shape Meter
  label: string            // "CPU" | "Memory" | "/var/lib/docker"
  valueText: string        // "12%" | "6.1 GB / 16 GB"
  percent: number          // 0–100
  warn: bool               // true above 60%

shape Consumer
  id: string
  name: string
  cpu: string

shape Event
  id: string
  name: string
  what: string             // "died · exit 137" | "started"
  tone: string             // "warn" | "ok" | "neutral"
  at: string               // "14:02"

shape CleanupItem
  id: string               // "containers" | "images" | "volumes" | "cache"
  label: string
  size: string
  checked: bool
  dangerous: bool          // volumes = true

shape Environment
  id: string
  label: string            // "local · Debian"
  connected: bool
  detail: string           // "connected · local socket"

shape Dashboard
  env: Environment
  envs: Environment[]
  totals: Totals
  alerts: Alert[]
  stacks: Stack[]
  selectedCount: number
  host: Meter[]
  serviceActive: bool
  consumers: Consumer[]
  events: Event[]
  cleanup: CleanupItem[]
  unread: number
  attention: number        // badge on Containers nav item

shape Change
  count: number
```

---

## 3. Root component state, resources and actions

```contract
component Fleet
  // ---- UI state
  state collapsed = false          // sidebar mode
  state page = "dashboard"         // active nav item id
  state filter = "all"             // "all" | "running" | "stopped"
  state query = ""
  state live = true
  state showDismissed = false

  // ---- data
  mutation changed as shape Change refreshes dash
  resource dash = dashboard(filter, query, showDismissed) as shape Dashboard

  derive isDashboard = page == "dashboard"
  derive hasSelection = dash.selectedCount > 0
  derive noRows = dash.stacks.length == 0
  derive noAlerts = dash.alerts.length == 0

  // ---- navigation & chrome
  action toggleNav
    collapsed = not collapsed
  action go(id: string)
    page = id
  action toggleLive
    live = not live
  action tick
    if live
      send changed = poll()

  // ---- filters
  action setFilter(value: string)
    filter = value
  action search(value: string)
    query = value

  // ---- selection & bulk
  action select(id: string, on: bool)
    send changed = setSelected(id, on)
  action clearSelection
    send changed = clearSelected()
  action bulk(op: string)                 // "start" | "stop" | "restart" | "remove"
    send changed = bulkOp(op)

  // ---- single container
  action start(id: string)
    send changed = containerOp(id, "start")
  action restart(id: string)
    send changed = containerOp(id, "restart")
  action openLogs(id: string)
    page = `logs:${id}`                  // placeholder route until the drawer exists
  action openShell(id: string)
    page = `shell:${id}`

  // ---- stacks
  action stackOp(id: string, op: string)  // "start" | "stop"
    send changed = stackOp(id, op)

  // ---- alerts
  action dismiss(id: string)
    send changed = dismissAlert(id)
  action restoreAlerts
    showDismissed = true
  action alertCta(id: string, op: string)
    send changed = alertOp(id, op)

  // ---- cleanup
  action toggleCleanup(id: string, on: bool)
    send changed = setCleanup(id, on)
  action dryRun
    send changed = pruneDryRun()

  // ---- environment
  action pickEnv(id: string)
    send changed = switchEnv(id)

task every(2000, tick)                     // VERIFY: task placement and binding to Fleet
```

Rule: confirmations for destructive actions (Remove, Remove all 5…, Prune) must open a confirm step before the `send`. Add a `state confirm = ""` and a confirm sheet when you build them. They are not drawn in the beta.

---

## 4. Wireframe overview

```
┌──────────────┬─────────────────────────────────────────────────────────────────────────┐
│ S1 Brand     │ T1 TopBar: [Env ▾] / Dashboard      [⌕ Search or run a command  Ctrl K] [●Live] [🔔9] │
│──────────────├─────────────────────────────────────────────────────────────────────────┤
│ OVERVIEW     │ P1 Dashboard                                  [Deploy stack…] [+ Run container] │
│  Dashboard   │    Streaming from docker events · local · /var/run/docker.sock          │
│  Events      │ ┌K1 Containers┐┌K2 Images┐┌K3 Volumes┐┌K4 Networks┐                       │
│ WORKLOADS    │ └─────────────┘└─────────┘└──────────┘└───────────┘                       │
│  Stacks    2 │ A0 Needs attention                                   [Show dismissed]     │
│  Containers 9│ ┌A1 ⚠ 4 containers were killed (exit 137) [Show 4][Restart all 4][×]┐    │
│  Templates   │ └A2 ⚠ 5 auto-named containers exited…    [Show 5][Remove all 5…][×]┘    │
│ RESOURCES    │ ┌S Stacks [All|Running|Stopped] [⌕ Filter]───┐ ┌H Host──────────────┐   │
│  Images   15 │ │ B Bulk bar (only when selection)           │ │ CPU ▓░░░            │   │
│  Volumes 13+ │ │ G kodus  0 of 7 running [Stopped][Edit][Start]│ Memory ▓▓░░         │   │
│  Networks  9 │ │   R ☐ kodus_api   —   Exited(137) [≡][>_][▶]│ Disk ▓▓▓░           │   │
│  Registries  │ │   …                                        │ │ Top consumers       │   │
│ SYSTEM       │ │ G openhands 1 of 1 running                 │ ├E Recent events──────┤   │
│  Host        │ │ G Standalone containers (collapsed)        │ ├C Cleanup────────────┤   │
│  Settings    │ └────────────────────────────────────────────┘ └─────────────────────┘   │
│ ● Docker…    │                                                                          │
│ [⟨ Collapse] │                                                                          │
└──────────────┴─────────────────────────────────────────────────────────────────────────┘
Collapsed sidebar = 72 px rail: icons only, group titles become 1 px dividers.
```

Root layout:

```contract
  view
    row class="app" testId="app"                          // flex-wrap: wrap
      Sidebar collapsed=collapsed page=page attention=dash.attention env=dash.env
      column class="workspace"                            // VERIFY: use a bare node with flex-direction: column if no `column`
        TopBar …
        if isDashboard
          DashboardPage …
        else
          PlaceholderPage title=pageTitle
```

`pageTitle` is a `fn` that maps the page id to a label:

```contract
fn pageTitle(id: string) = id == "dashboard" ? "Dashboard" : id == "events" ? "Events" : id == "stacks" ? "Stacks" : id == "containers" ? "Containers" : id == "templates" ? "Templates" : id == "images" ? "Images" : id == "volumes" ? "Volumes" : id == "networks" ? "Networks" : id == "registries" ? "Registries" : id == "host" ? "Host & systemd" : "Settings"
```

VERIFY: ternary support. If missing, return the label from `app.ts` in a small `nav` resource.

---

## 5. Sidebar (`component Sidebar`)

Props: `collapsed: bool`, `page: string`, `attention: number`, `env: Environment`.

| # | Element | Exact element | Attributes and bindings | Events | testId | Style | Notes |
|---|---|---|---|---|---|---|---|
| S0 | Sidebar container | node (block) | width 248 expanded, 72 collapsed; `flex-shrink: 0`; `flex-direction: column`; gap 2; padding 16/12 (10 collapsed); transition on width 200 ms ease | none | `sidebar` | `nav`, `nav-collapsed` | `role="navigation"`, `aria-label="Main"`. Never sticky. On narrow windows it wraps above content. |
| S1 | Brand row | `row` | min-height 40; centered when collapsed | none | `brand` | `brand` | Logo SVG 26 px (2×2 rounded squares). Text "[App name]" only `if not collapsed`. |
| S2 | Group title | `text` | "OVERVIEW", "WORKLOADS", "RESOURCES", "SYSTEM"; 11 px, uppercase, letter-spacing .8 | none | `group-overview` … | `group-title` | Only `if not collapsed`. |
| S3 | Group divider | node | height 1, margin 10/14 | none | none | `divider` | Only `if collapsed`. Decorative. |
| S4 | Nav item ×11 | `NavItem` component → `button` | `id`, `label`, `icon`, `count`, `active=page == id`, `collapsed` | `press=go(id)` | `nav-<id>` | `nav-item`, `nav-item-active`, `nav-item-collapsed` | Min height 44. `aria-label` always set (label + count). `title=label` when collapsed (VERIFY). Count is shown only when expanded. |
| S5 | Attention badge (expanded) | `text` | `` `${attention}` `` on Containers item; pill, warn color | none | `nav-containers-badge` | `badge-warn` | Hidden when `attention == 0`. |
| S6 | Attention dot (collapsed) | node | 8×8 circle at top 7 / right 14, 2 px ring in sidebar color | none | `nav-containers-dot` | `dot-warn` | Needs `position: absolute` (VERIFY). Fallback: place it after the icon in the flow. |
| S7 | Spacer | node | `flex: 1`, min-height 24 | none | none | none | Pushes S8 and S9 to the bottom. |
| S8 | Engine status | `row` | 10 px dot (green if `env.connected`, grey if not); "Docker Engine" + `env.detail` when expanded | none | `engine-status` | `engine` | `aria-label` = "Docker Engine connected" or "disconnected". |
| S9 | Collapse toggle | `button` | icon: panel with chevron (direction flips); label "Collapse" when expanded | `press=toggleNav` | `nav-toggle` | `nav-item` | `aria-label` "Collapse sidebar" or "Expand sidebar". `aria-expanded=not collapsed` (VERIFY attr). |

Nav items, in order:

| Group | id | Label | Icon (SVG paths, 24 viewBox, stroke 1.8) | Count |
|---|---|---|---|---|
| Overview | `dashboard` | Dashboard | 4 rects: `3,3,7,9` `14,3,7,5` `14,12,7,9` `3,16,7,5` (rx 1.5) | none |
| Overview | `events` | Events | `M3 12h4l3-8 4 16 3-8h4` | none |
| Workloads | `stacks` | Stacks | `M12 3l9 5-9 5-9-5z` + `M3 13l9 5 9-5` | `stacks.length` |
| Workloads | `containers` | Containers | rect `3,7,18,13` + `M7 7V4h10v3` + `M3 13h18` | `attention` badge |
| Workloads | `templates` | Templates | rects `3,3,18,7` `3,14,8,7` `15,14,6,7` | none |
| Resources | `images` | Images | rect `3,3,18,18` + circle `9,9,r2` + `M21 15l-5-5L5 21` | `totals.images` |
| Resources | `volumes` | Volumes | ellipse `12,5,8,3` + `M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5` + `M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3` | `totals.volumes` |
| Resources | `networks` | Networks | circles `12,5` `5,19` `19,19` r2 + `M12 7v5M12 12l-6 5M12 12l6 5` | `totals.networks` |
| Resources | `registries` | Registries | `M3 7l9-4 9 4-9 4z` + `M3 7v10l9 4 9-4V7` + `M12 11v10` | none |
| System | `host` | Host & systemd | rects `3,4,18,7` `3,13,18,7` + `M7 7.5h.01M7 16.5h.01` | none |
| System | `settings` | Settings | circle r3 + `M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M4.9 19.1L7 17M17 7l2.1-2.1` | none |

`NavItem` sketch:

```contract
component NavItem
  // props: id, label, count, active, collapsed
  view
    button press=go(id) aria-label=label testId=`nav-${id}` class=active ? "nav-item-active" : "nav-item"
      svg width=20 height=20 viewBox="0 0 24 24" aria-hidden=true     // VERIFY svg child syntax
        // paths from the table above
      if not collapsed
        text label flex=1
        if count != ""
          text count class="nav-count"
```

VERIFY: how a child component calls a parent action (`go`). If it can't, keep NavItem inline in Sidebar with a `for` over a static nav list from `app.ts`.

---

## 6. Top bar (`component TopBar`)

| # | Element | Exact element | Attributes and bindings | Events | testId | Style | Notes |
|---|---|---|---|---|---|---|---|
| T0 | Bar | `row` | padding 12/32; bottom border; wraps | none | `topbar` | `topbar` | |
| T1 | Environment picker | `select` | value `dash.env.id`; options from `dash.envs` plus "+ Add environment…" | `change=pickEnv` | `env-select` | `select` | `aria-label="Environment"`. Options: local · Debian, Podman · rootless, remote host over SSH. VERIFY select API. |
| T2 | Breadcrumb | `row` + `text` | "/" separator (decorative) + `pageTitle(page)` bold | none | `breadcrumb` | `crumb` | `role="navigation"`, `aria-label="Breadcrumb"`. |
| T3 | Spacer | node | `flex: 1` | none | none | none | |
| T4 | Command palette button | `button` | search icon + "Search or run a command" + key cap "Ctrl K"; min-width 300 | `press=openPalette` (future) | `cmd-open` | `cmd-button` | Opens the palette (not built yet). Keyboard shortcut belongs to the platform layer. |
| T5 | Live toggle | `button` | dot (green if live, grey if paused) + `live ? "Live" : "Paused"`; height 40 | `press=toggleLive` | `live-toggle` | `chip-button` | `aria-pressed=live` (VERIFY). Pausing stops the 2 s poll. |
| T6 | Notifications | `button` | bell icon 18 px; badge `dash.unread` (hidden if 0) | `press=openNotifications` (future) | `notifications` | `icon-button` | `aria-label` = `` `Notifications, ${dash.unread} unread` ``. 40×40. |

---

## 7. Dashboard page (`component DashboardPage`)

Container: padding 24/32/40, `flex-direction: column`, gap 20, max-width 1320.

### 7.1 Page header

| # | Element | Exact element | Attributes and bindings | Events | testId | Notes |
|---|---|---|---|---|---|---|
| P1 | Title | `text` | "Dashboard"; 28 px semibold; `role="heading"` | none | `page-title` | |
| P2 | Subtitle | `text` | `live ? "Streaming from docker events · local · /var/run/docker.sock" : "Updates paused · numbers may be stale"` | none | `page-subtitle` | |
| P3 | Deploy stack | `button` | "Deploy stack from compose…" | `press=deployStack` (future) | `deploy-stack` | Opens a compose file picker or editor. |
| P4 | Run container | `button` | "+ Run container"; `buttonStyle="filled"` | `press=runContainer` (future) | `run-container` | Primary action of the page. |

### 7.2 Totals grid (K1–K4)

Grid: 4 equal columns, gap 16 (`grid-template-columns: repeat(4, minmax(0, 1fr))`). Wraps to 2 columns under 900 px and 1 column under 560 px.

Each card is a `button` (whole card clickable) with radius 12, padding 18, column, gap 8. The top row has the label and a "→" glyph (`aria-hidden`).

| # | Card | Big number | Detail line | press | testId |
|---|---|---|---|---|---|
| K1 | Containers | `totals.containers` (14) | Stacked bar: running / failed / clean exit, widths = share of total. Below it: `` `${running} running` `` (ok), `` `${failed} failed` `` (warn), `` `${cleanExit} clean exit` `` (muted) | `go("containers")` | `kpi-containers` |
| K2 | Images | `totals.images` (15) | `` `${imagesSize} on disk · ${imagesUnused} unused` `` | `go("images")` | `kpi-images` |
| K3 | Volumes | `totals.volumes` (13+) | `` `${volumesUnused} not used by any container` `` (warn) | `go("volumes")` | `kpi-volumes` |
| K4 | Networks | `totals.networks` (9) | `networksDetail` | `go("networks")` | `kpi-networks` |

K1 stacked bar: a `row` with height 6, radius 3, overflow hidden, 3 children with `flex` = their count. Add `aria-hidden` because the text line carries the same info.

### 7.3 Needs attention (A0–A2)

| # | Element | Exact element | Attributes and bindings | Events | testId | Notes |
|---|---|---|---|---|---|---|
| A0 | Section header | `row` | "Needs attention" heading; "Show dismissed" link button only if some alert is dismissed | `press=restoreAlerts` | `alerts-restore` | |
| A0e | Empty state | `text` | "Nothing needs attention." in a bordered box | none | `alerts-empty` | `if noAlerts`. |
| A1… | Alert card | `AlertCard` in `for a in dash.alerts key=a.id` | tone `warn` = warm background and border; `neutral` = white card | none | `` `alert-${a.id}` `` | Row-owned `state open = false`. |
| A·icon | Warning icon | `svg` 20 px | triangle + line + dot; stroke = tone color | none | none | `aria-hidden`. |
| A·title | Title + hint | `text` ×2 | `a.title` (semibold), `a.hint` (13 px, muted) | none | `` `alert-${a.id}-title` `` | min-width 240 so it wraps before the buttons squeeze it. |
| A·show | Show/Hide | `button` | `open ? "Hide" : `Show ${a.items.length}`` | `press=toggleOpen` (row action: `open = not open`) | `` `alert-${a.id}-toggle` `` | `aria-expanded=open`. |
| A·cta | Primary action | `button` | `a.cta`; warn tone = filled warm, neutral = outlined warm | `press=alertCta(a.id, a.ctaAction)` | `` `alert-${a.id}-cta` `` | "Remove all 5…" needs a confirm step. |
| A·dismiss | Dismiss | `button` | "×", 36×36 | `press=dismiss(a.id)` | `` `alert-${a.id}-dismiss` `` | `aria-label="Dismiss"`. |
| A·chips | Affected containers | `row` wrap + `for i in a.items key=i.id` → `text` | `` `${i.name} · exit ${i.exitCode}` ``, monospace 12, bordered pill | none | `` `alert-${a.id}-item-${i.id}` `` | Only `if open`. Indented 32 px to align under the title. |

Rules for generating alerts in `app.ts`:

* `oom`: containers with exit code 137. Title `N containers were killed (exit 137)`. Hint: "Exit 137 means SIGKILL. It is often the OOM killer or a forced stop. Check memory limits before restarting." CTA "Restart all N".
* `crash`: containers with a Docker auto-generated name (adjective_surname pattern) and a non-zero exit code. Title `N auto-named containers exited with errors`. Hint: "Random names mean they were started without --name, usually one-off docker run tests. They are likely safe to remove." CTA "Remove all N…".
* Add more rules later: restarting loops, unhealthy healthchecks, low disk.

Sample state (from the beta): `oom` = kodus_api, kodus-worker-prod, kodus-webhooks-prod, kodus-mcp-manager. `crash` = cool_hodgkin (1), friendly_boyd (2), gracious_tesla (1), infallible_margulis (1), loving_neumann (1).

### 7.4 Two-column area

Grid `minmax(0, 2fr) minmax(300px, 1fr)`, gap 16, `align-items: start`. It becomes one column under 1000 px.

#### 7.4.1 Stacks panel (left)

| # | Element | Exact element | Attributes and bindings | Events | testId | Notes |
|---|---|---|---|---|---|---|
| S1.1 | Panel | node | white card, radius 12, overflow hidden | none | `stacks-panel` | `aria-label="Stacks and containers"`. |
| S1.2 | Header row | `row` wrap | heading "Stacks" (flex 1) | none | none | |
| S1.3 | State filter | `row` of 3 `button` | All, Running, Stopped; selected = white raised pill, bold | `press=setFilter("all")` etc. | `filter-all`, `filter-running`, `filter-stopped` | `role="group"`, `aria-label="State filter"`, `aria-pressed` per button. |
| S1.4 | Filter input | `input` | `value=query`, `placeholder="Filter"`, width 180, search icon before it | `input=search` | `stacks-search` | `aria-label="Filter containers"`. See the Linux caveat. |
| S1.5 | Bulk bar | `row` | `` `${dash.selectedCount} selected` `` + Start, Stop, Restart, Remove…, Clear | `press=bulk("start")`, `bulk("stop")`, `bulk("restart")`, `bulk("remove")`, `clearSelection` | `bulk-bar`, `bulk-start`, `bulk-stop`, `bulk-restart`, `bulk-remove`, `bulk-clear` | `if hasSelection`. `role="toolbar"`. Remove is destructive (`destructive=true`) and needs a confirm step. |
| S1.6 | Empty state | `text` | "No containers match this filter." | none | `stacks-empty` | `if noRows`. |
| S1.7 | Stack group | `StackGroup` in `for s in dash.stacks key=s.id` | row-owned `state open` (default true for compose stacks, false for standalone) | none | `` `stack-${s.id}` `` | |

`StackGroup` header row (background slightly tinted, padding 12/18):

| # | Element | Exact element | Bindings | Events | testId | Notes |
|---|---|---|---|---|---|---|
| G1 | Expand toggle | `button` 32×32 | chevron down when open, right when closed | `press=toggleOpen` (`open = not open`) | `` `stack-${s.id}-toggle` `` | `aria-label` `` `Toggle ${s.name}` ``, `aria-expanded=open`. |
| G2 | Name + summary | `text` ×2 | `s.name` (semibold); `` `${s.isCompose ? "Compose project · " : ""}${s.running} of ${s.total} running` `` | none | `` `stack-${s.id}-name` `` | |
| G3 | Status badge | `text` | `s.status`: Running = ok, Partial = info, Stopped = neutral | none | `` `stack-${s.id}-status` `` | Text, not color only. |
| G4 | Edit compose | `button` | "Edit compose" | `press=editCompose(s.id)` (future) | `` `stack-${s.id}-edit` `` | `if s.isCompose`. |
| G5 | Stack action | `button` | `s.running == 0 ? "Start stack" : "Stop stack"` | `press=stackOp(s.id, s.running == 0 ? "start" : "stop")` | `` `stack-${s.id}-action` `` | `if s.isCompose`. Runs `docker compose -p <id> up -d` or `stop`. |
| G6 | Container rows | `ContainerRow` in `for c in s.containers key=c.id` | | | | `if open`. Rows sit inside a horizontal scroll box with min-width 640. |

`ContainerRow` (grid: `32px minmax(180px, 1fr) minmax(150px, auto) 116px 132px`, gap 10, padding 6/18, top border):

| # | Element | Exact element | Bindings | Events | testId | Notes |
|---|---|---|---|---|---|---|
| R1 | Select | `input type="checkbox"` | `checked=c.selected`; 18×18 | `change=select(c.id, not c.selected)` | `` `row-${c.id}-select` `` | `aria-label` `` `Select ${c.name}` ``. |
| R2 | Name | `text` | `c.name`, monospace 13, `text-overflow: ellipsis`, `line-clamp=1` | none | `` `row-${c.id}-name` `` | |
| R3 | Ports | `text` | `c.ports`, monospace 12, link color | none | `` `row-${c.id}-ports` `` | Later: make it a button that opens `http://host:port`. |
| R4 | State pill | `text` | `c.state == "running" ? "Running" : `Exited (${c.exitCode})``; ok / warn (exit ≠ 0) / neutral (exit 0) | none | `` `row-${c.id}-state` `` | |
| R5 | Logs | `button` 36×36 | icon `M4 6h16M4 12h10M4 18h13` | `press=openLogs(c.id)` | `` `row-${c.id}-logs` `` | `aria-label` `` `Logs for ${c.name}` ``. |
| R6 | Shell | `button` 36×36 | icon `M4 17l6-5-6-5` + `M12 19h8`; disabled when not running | `press=openShell(c.id)` | `` `row-${c.id}-shell` `` | `disabled=c.state != "running"` (VERIFY). Hint "Start the container to open a shell". |
| R7a | Restart | `button` 36×36 | icon `M20 12a8 8 0 1 1-2.3-5.7L20 8` + `M20 3v5h-5` | `press=restart(c.id)` | `` `row-${c.id}-restart` `` | `if c.state == "running"`. |
| R7b | Start | `button` 36×36 | filled play triangle `M7 4l13 8-13 8z`, ok color | `press=start(c.id)` | `` `row-${c.id}-start` `` | `else` branch of R7a. |

#### 7.4.2 Right column (column, gap 16)

**Host panel (H)**

| # | Element | Exact element | Bindings | testId | Notes |
|---|---|---|---|---|---|
| H1 | Header | `row` | "Host" heading + `serviceActive ? "docker.service active" : "docker.service inactive"` (ok / warn) | `host-header` | |
| H2 | Meter ×3 | `for m in dash.host key=m.label` | label + `m.valueText` on one row; bar: track height 8 radius 4, fill width `` `${m.percent}%` ``, warn color if `m.warn` | `` `meter-${m.label}` `` | CPU, Memory, /var/lib/docker. |
| H3 | Top consumers | `text` title + `for k in dash.consumers key=k.id` | name (monospace) left, `k.cpu` right | `consumers` | Top 5 by CPU. Separated by a top border. |

**Recent events (E)**

| # | Element | Exact element | Bindings | Events | testId |
|---|---|---|---|---|---|
| E1 | Header | `row` | "Recent events" + "View all" link button | `press=go("events")` | `events-all` |
| E2 | Event row | `for e in dash.events key=e.id` → grid `10px 1fr auto` | dot (tone), `e.name` monospace + `e.what` muted, `e.at` right | none | `` `event-${e.id}` `` |

Keep the last 6 events. Source: `docker events --format '{{json .}}'` filtered to container start, die, oom, kill and destroy.

**Cleanup (C)**

| # | Element | Exact element | Bindings | Events | testId | Notes |
|---|---|---|---|---|---|---|
| C1 | Title | `text` heading | "Cleanup" | none | none | |
| C2 | Item ×4 | `for x in dash.cleanup key=x.id` → `row` with checkbox, label (flex 1), size | `checked=x.checked` | `change=toggleCleanup(x.id, not x.checked)` | `` `cleanup-${x.id}` `` | Defaults: containers and dangling images on; volumes and cache off. |
| C3 | Warning | `text` 13 px muted | "Volumes are off by default. Removing them deletes data." | none | `cleanup-warning` | |
| C4 | Dry run | `button` | "Dry run: preview what gets removed" (warn outline) | `press=dryRun` | `cleanup-dryrun` | Shows a list and total before the real prune. Never prune without this step. |

### 7.5 Placeholder page

When `page != "dashboard"`: heading `pageTitle(page)` and the text "[<page> view] This beta only designs the Dashboard." `testId="placeholder"`. Remove it once each page exists.

---

## 8. Data sources (`app.ts` or a Rust data crate)

| Source | Args | Returns | System call (suggested) |
|---|---|---|---|
| `dashboard` | filter, query, showDismissed | `Dashboard` | Docker Engine API over `/var/run/docker.sock`: `GET /containers/json?all=1`, `GET /system/df`, `GET /networks`, `GET /volumes`, `GET /containers/{id}/stats?stream=false` for running ones; `systemctl is-active docker`. Groups by label `com.docker.compose.project`. Applies filter and query. Builds alerts. |
| `poll` | none | `Change` | Re-reads the same data. Called every 2 s while Live. Later: replace with a pushed stream from `/events`. |
| `setSelected` | id, on | `Change` | Store only. |
| `clearSelected` | none | `Change` | Store only. |
| `bulkOp` | op | `Change` | `POST /containers/{id}/start\|stop\|restart`, `DELETE /containers/{id}` for each selected id. |
| `containerOp` | id, op | `Change` | `POST /containers/{id}/{op}`. |
| `stackOp` | id, op | `Change` | `docker compose -p <id> up -d` / `stop`, or the same calls per container. |
| `dismissAlert` | id | `Change` | Store only (persist in SQLite so it survives restarts). |
| `alertOp` | id, op | `Change` | Restart or remove the alert's containers. |
| `setCleanup` | id, on | `Change` | Store only. |
| `pruneDryRun` | none | `Change` | Computes what `docker system prune` (+ `--volumes` if checked) would remove. Writes the result into the store for a confirm sheet. |
| `switchEnv` | id | `Change` | Switches the socket: local Docker, rootless Podman (`$XDG_RUNTIME_DIR/podman/podman.sock`, Docker-compatible API), or an SSH tunnel. |

Grants: the README lists grants for network, files, SQLite and secrets. It does not show a grant for a Unix socket or for running processes. Talking to `/var/run/docker.sock` is the main risk of this build. Recommended path: a Rust data crate that uses the `bollard` crate against the socket, since Rust crates run natively. Confirm with the Exact maintainers how a crate gets socket access.

Security notes:
* The user must be in the `docker` group, or use rootless Podman. Being in the `docker` group is equivalent to root. Show that in Settings.
* Never show env var values by default. Mask them, like `OPENHANDS_AUTOMATION_API_KEY` in Beta A.
* Destructive actions (remove, prune, remove all) always need a confirm step that lists what will be removed.

---

## 9. Styles

Ship v1 with platform colors and fonts. Add the colors below only where they carry meaning. Use `light-dark()` so dark mode comes for free.

| Token | Light | Dark | Used by |
|---|---|---|---|
| sidebar-bg | #10213F | #0B1730 | S0 |
| sidebar-fg | #DCE6F7 | #DCE6F7 | nav text |
| sidebar-active | #1F3A6B | #1F3A6B | active nav item |
| sidebar-muted | #8FA6CC | #8FA6CC | group titles |
| surface | #FFFFFF | #161A20 | cards, top bar |
| canvas | #F5F6F8 | #0F1115 | page background |
| border | #E2E5EA | #262B33 | cards, rows |
| text | #16191F | #E6E8EB | body |
| muted | #5B6472 | #9AA3AE | secondary text |
| accent | #1F5FD6 | #7DB4FF | primary button, links, ports |
| ok | #1A7F37 | #7EE2A0 | running, active |
| ok-bg | #E6F4EA | #1C2A1F | running pill |
| warn | #8A4500 (text) / #B25A00 (fill) | #F0A35E | failed, attention |
| warn-bg | #FFF8F0 | #2A1C14 | oom alert, exit≠0 pill (#FDEBD8) |
| info-bg | #E8EFFC | #1A2333 | partial stack, bulk bar |
| neutral-bg | #EEF0F3 | #2A3140 | stopped pill, exit 0 |

Sizes: radius 12 for cards, 8 for buttons, 6 for small toggles. Touch targets at least 44 px for nav items and 36 px for icon buttons inside dense rows. Fonts in the beta: Inter (UI), Space Grotesk (headings and big numbers), JetBrains Mono (names, ports, ids). Declare them with `font` or drop them for platform fonts.

Example (VERIFY body syntax):

```contract
style nav-item
  display: flex
  align-items: center
  gap: 12px
  min-height: 44px
  padding: 10px 12px
  border-radius: 8px

style nav-item-active
  display: flex
  align-items: center
  gap: 12px
  min-height: 44px
  padding: 10px 12px
  border-radius: 8px
  background: light-dark(#1F3A6B, #1F3A6B)
  color: #FFFFFF
  font-weight: 600
```

There is no cascade, so every style variant repeats its base properties.

---

## 10. Responsive rules

| Window width | Change |
|---|---|
| ≥ 1280 | As drawn. Sidebar expanded by default. |
| 1000–1279 | Sidebar starts collapsed (72 px). Two-column area stays. |
| 760–999 | Two-column area becomes one column. Right panels go under Stacks. |
| < 760 | Totals become 2 columns. Top bar wraps: env and breadcrumb on one line, buttons on the next. Command button drops to icon only. |
| < 560 | Totals become 1 column. Container rows scroll sideways inside their box. |

---

## 11. Tests (`app.test.contract`)

```contract
test "sidebar collapses to icons"
  expect tree has "nav-dashboard"
  tap "nav-toggle"
  expect tree missing "group-overview"
  tap "nav-toggle"
  expect tree has "group-overview"

test "nav changes the page"
  tap "nav-images"
  expect text "breadcrumb" == "/ Images"          // VERIFY text of a row node
  expect tree has "placeholder"
  tap "nav-dashboard"
  expect tree has "stacks-panel"

test "filter running hides stopped stacks"
  tap "filter-running"
  expect tree missing "stack-kodus"
  expect tree has "stack-openhands"
  tap "filter-all"
  expect tree has "stack-kodus"

test "selection shows the bulk bar"
  expect tree missing "bulk-bar"
  tap "row-kodus_api-select"
  expect tree has "bulk-bar"
  tap "bulk-clear"
  expect tree missing "bulk-bar"

test "alerts expand and dismiss"
  tap "alert-oom-toggle"
  expect tree has "alert-oom-item-kodus_api"
  tap "alert-oom-dismiss"
  expect tree missing "alert-oom"
  tap "alerts-restore"
  expect tree has "alert-oom"

test "live toggle"
  expect text "live-toggle" == "Live"
  tap "live-toggle"
  expect text "live-toggle" == "Paused"
```

Use a fake `dashboard` source with the sample state below for these tests.

---

## 12. Sample state (from the beta)

| Container | Stack | State | Exit | Ports | CPU |
|---|---|---|---|---|---|
| kodus_api | kodus | exited | 137 | — | — |
| kodus-web-prod | kodus | exited | 0 | — | — |
| kodus-worker-prod | kodus | exited | 137 | — | — |
| kodus-webhooks-prod | kodus | exited | 137 | — | — |
| kodus-mcp-manager | kodus | exited | 137 | — | — |
| db_kodus_postgres | kodus | exited | 0 | — | — |
| db_kodus_mongodb | kodus | exited | 0 | — | — |
| openhands-agent-canvas | openhands | running | — | 127.0.0.1:8000→8000 | 0.01% |
| ccs-tts | standalone | running | — | — | 0.00% |
| cool_hodgkin | standalone | exited | 1 | — | — |
| friendly_boyd | standalone | exited | 2 | — | — |
| gracious_tesla | standalone | exited | 1 | — | — |
| infallible_margulis | standalone | exited | 1 | — | — |
| loving_neumann | standalone | exited | 1 | — | — |

Totals: 14 containers, 2 running, 9 failed, 3 clean exit. 15 images, 13+ volumes, 9 networks (7 bridge, host, none).
Stack grouping by name is an assumption in the beta. In the real app, use the `com.docker.compose.project` label.

---

## 13. Not drawn yet (next design pass)

* Container detail drawer: Logs, Stats, Shell, Env, Inspect.
* Command palette overlay (Ctrl K).
* Confirm sheets for Remove, Remove all, Prune.
* Notifications panel.
* Add environment flow (Podman, SSH).
* Containers, Images, Volumes, Networks, Events, Host and Settings pages.
* Dark mode switch (the tokens above already support it).
