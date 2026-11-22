# Backend feature inventory and verification

This inventory is bounded to Lazydocker commit `596fd59df6b088bb1e38c5bb08cd4fbf28b3ebfb`: the six resource panels, their action menus and bulk menus, Docker command models, and functional configuration. It distinguishes executable capabilities from human validation. A passing demo test proves routing and deterministic state handling, not successful execution against a real Docker daemon.

The backend imports `github.com/jesseduffield/lazydocker/pkg/commands`. Container lifecycle, image/volume/network removal, inspection, process tables, image history, and CPU/memory calculations use that package. Compose commands preserve upstream operation semantics through argument arrays because upstream service methods interpolate shell command strings. Container polling uses the same Docker SDK directly to avoid Lazydocker's eager inspection of every container.

## Source boundaries

- [Container tabs, lifecycle menus, exec, browser and bulk actions](https://github.com/jesseduffield/lazydocker/blob/596fd59df6b088bb1e38c5bb08cd4fbf28b3ebfb/pkg/gui/containers_panel.go)
- [Compose service tabs, lifecycle, recreate and rebuild](https://github.com/jesseduffield/lazydocker/blob/596fd59df6b088bb1e38c5bb08cd4fbf28b3ebfb/pkg/gui/services_panel.go)
- [Image configuration/history and four removal variants](https://github.com/jesseduffield/lazydocker/blob/596fd59df6b088bb1e38c5bb08cd4fbf28b3ebfb/pkg/gui/images_panel.go)
- [Volume inspection, removal and force removal](https://github.com/jesseduffield/lazydocker/blob/596fd59df6b088bb1e38c5bb08cd4fbf28b3ebfb/pkg/gui/volumes_panel.go)
- [Network inspection and removal](https://github.com/jesseduffield/lazydocker/blob/596fd59df6b088bb1e38c5bb08cd4fbf28b3ebfb/pkg/gui/networks_panel.go)
- [Project logs, Compose configuration and local configuration access](https://github.com/jesseduffield/lazydocker/blob/596fd59df6b088bb1e38c5bb08cd4fbf28b3ebfb/pkg/gui/project_panel.go)
- [Custom commands and terminal subprocess behavior](https://github.com/jesseduffield/lazydocker/blob/596fd59df6b088bb1e38c5bb08cd4fbf28b3ebfb/pkg/gui/custom_commands.go)
- [Default command templates, bulk Compose commands, logging, graph settings and service filters](https://github.com/jesseduffield/lazydocker/blob/596fd59df6b088bb1e38c5bb08cd4fbf28b3ebfb/pkg/config/app_config.go)
- [User-visible panel command inventory](https://github.com/jesseduffield/lazydocker/blob/596fd59df6b088bb1e38c5bb08cd4fbf28b3ebfb/docs/keybindings/Keybindings_en.md)

Graph verification used project `tmp-lazy-lazydocker`, generation `2026-11-22T18:01:29Z`; relevant `pkg/commands`, `pkg/gui`, and `pkg/config` scopes had no recorded gaps. Exact source was also read. This is a best-effort index signal, not a proof that unrelated upstream features cannot exist.

## API contract

All resource inventories, details, configuration, jobs and action requests require `X-LazyContainer-Token`. Requests must use a numeric loopback Host, without browser Origin or Fetch-Site headers. `--listen` accepts only loopback IPs; the live Docker client accepts only Unix sockets. Health and the non-secret dashboard summary are readable locally without the session token.

- `GET /api/inventory?kind=containers|services|projects|images|volumes|networks|commands|host` returns items, available actions and detail tabs.
- `GET /api/detail/{kind}/{encoded-id}?tab=...` returns plain text and graph metrics/history. `reveal=1` explicitly reveals protected fields.
- `POST /api/action/preview` accepts `{kind,ids,op,args?}` and returns exact targets, resolved command arguments, an expiring confirmation token and a summary.
- `POST /api/action` accepts `{confirmation}`. Tokens are single-use, expire after two minutes and revalidate target identity/configuration. Live actions return HTTP 202 with `jobId`.
- `GET /api/jobs/{id}` returns `running`, `succeeded` or `failed`, output, completed count and error. Partial completion is reported; it is never relabelled success.
- `GET /api/config` and `POST /api/config` read/save validated JSON configuration. Saving invalidates outstanding confirmations. Demo configuration stays in memory.
- The previous direct `POST /api/containers/{id}/{op}` mutator returns HTTP 410.

Bulk actions use the same operation with multiple exact IDs. “Stop all” is select-all plus stop; “remove all” is select-all plus force remove. Prune previews freeze eligible IDs and remove only those IDs. New resources created after preview are never silently included. Image default removal previews eligible untagged parent images and then removes explicit IDs with Docker `noprune=1`; tagged/in-use parents are excluded and parent eligibility is checked again.

## Executable actions

Every listed action has isolated demo coverage through the named subtest. “SDK” tests use an HTTP fake Docker daemon and invoke real Lazydocker methods. “Argv” tests verify exact executable/argument boundaries. Live checks and their results are maintained in [FEATURES.md](FEATURES.md); human validation remains Pending until a person confirms the corresponding GUI behavior.

| Feature ID | Backend implementation / upstream equivalent | Automated evidence | Live prerequisite | Human |
|---|---|---|---|---|
| `containers.start` | Container core / Docker SDK | `TestEveryAdvertisedDemoAction/containers/start`; SDK `containers/start` | Local Docker socket | Pending |
| `containers.stop` | Container core / Docker SDK | `TestEveryAdvertisedDemoAction/containers/stop`; SDK `containers/stop` | Local Docker socket | Pending |
| `containers.restart` | Container core / Docker SDK | `TestEveryAdvertisedDemoAction/containers/restart`; SDK `containers/restart` | Local Docker socket | Pending |
| `containers.pause` | Container core / Docker SDK | `TestEveryAdvertisedDemoAction/containers/pause`; SDK `containers/pause` | Local Docker socket | Pending |
| `containers.unpause` | Container core / Docker SDK | `TestEveryAdvertisedDemoAction/containers/unpause`; SDK `containers/unpause` | Local Docker socket | Pending |
| `containers.kill` | Docker ContainerKill(SIGKILL), additional GUI capability | `TestEveryAdvertisedDemoAction/containers/kill`; SDK `containers/kill` | Local Docker socket | Pending |
| `containers.remove` | Container core / Docker SDK | `TestEveryAdvertisedDemoAction/containers/remove`; SDK `containers/remove` | Local Docker socket | Pending |
| `containers.remove-volumes` | Container core / Docker SDK | `TestEveryAdvertisedDemoAction/containers/remove-volumes`; SDK `containers/remove-volumes` | Local Docker socket | Pending |
| `containers.remove-force` | Container core / Docker SDK | `TestEveryAdvertisedDemoAction/containers/remove-force`; SDK `containers/remove-force` | Local Docker socket | Pending |
| `containers.remove-force-volumes` | Container core / Docker SDK | `TestEveryAdvertisedDemoAction/containers/remove-force-volumes`; SDK `containers/remove-force-volumes` | Local Docker socket | Pending |
| `containers.exec` | Native terminal + Docker/Compose argv | `TestEveryAdvertisedDemoAction/containers/exec` | Local Docker socket; terminal emulator | Pending |
| `containers.attach` | Native terminal + Docker/Compose argv | `TestEveryAdvertisedDemoAction/containers/attach` | Local Docker socket; terminal emulator | Pending |
| `containers.logs-follow` | Native terminal + Docker/Compose argv | `TestEveryAdvertisedDemoAction/containers/logs-follow` | Local Docker socket; terminal emulator | Pending |
| `containers.open-browser` | Published TCP port + xdg-open | `TestEveryAdvertisedDemoAction/containers/open-browser` | Local Docker socket; desktop URL handler | Pending |
| `containers.prune` | Preview eligible IDs, then exact-ID removal | `TestEveryAdvertisedDemoAction/containers/prune` | Local Docker socket | Pending |
| `services.start` | Compose service argv | `TestEveryAdvertisedDemoAction/services/start`; Argv `start` | Docker Compose + readable project files | Pending |
| `services.stop` | Compose service argv | `TestEveryAdvertisedDemoAction/services/stop`; Argv `stop` | Docker Compose + readable project files | Pending |
| `services.restart` | Compose service argv | `TestEveryAdvertisedDemoAction/services/restart`; Argv `restart` | Docker Compose + readable project files | Pending |
| `services.pause` | Compose service argv | `TestEveryAdvertisedDemoAction/services/pause`; Argv `pause` | Docker Compose + readable project files | Pending |
| `services.unpause` | Compose service argv | `TestEveryAdvertisedDemoAction/services/unpause`; Argv `unpause` | Docker Compose + readable project files | Pending |
| `services.up` | Compose service argv | `TestEveryAdvertisedDemoAction/services/up`; Argv `up` | Docker Compose + readable project files | Pending |
| `services.recreate` | Compose service argv | `TestEveryAdvertisedDemoAction/services/recreate`; Argv `recreate` | Docker Compose + readable project files | Pending |
| `services.rebuild` | Compose service argv | `TestEveryAdvertisedDemoAction/services/rebuild`; Argv `rebuild` | Docker Compose + readable project files | Pending |
| `services.remove` | Compose service argv | `TestEveryAdvertisedDemoAction/services/remove`; Argv `remove` | Docker Compose + readable project files | Pending |
| `services.remove-volumes` | Compose service argv | `TestEveryAdvertisedDemoAction/services/remove-volumes`; Argv `remove-volumes` | Docker Compose + readable project files | Pending |
| `services.exec` | Native terminal + Docker/Compose argv | `TestEveryAdvertisedDemoAction/services/exec` | Docker Compose + readable project files; terminal emulator | Pending |
| `services.attach` | Native terminal + Docker/Compose argv | `TestEveryAdvertisedDemoAction/services/attach` | Docker Compose + readable project files; terminal emulator | Pending |
| `services.logs-follow` | Native terminal + Docker/Compose argv | `TestEveryAdvertisedDemoAction/services/logs-follow` | Docker Compose + readable project files; terminal emulator | Pending |
| `services.open-browser` | Published TCP port + xdg-open | `TestEveryAdvertisedDemoAction/services/open-browser` | Docker Compose + readable project files; desktop URL handler | Pending |
| `projects.up` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/up`; Argv `up` | Docker Compose + readable project files | Pending |
| `projects.up-attached` | Native terminal + Docker/Compose argv | `TestEveryAdvertisedDemoAction/projects/up-attached` | Docker Compose + readable project files; terminal emulator | Pending |
| `projects.down` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/down`; Argv `down` | Docker Compose + readable project files | Pending |
| `projects.down-volumes` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/down-volumes`; Argv `down-volumes` | Docker Compose + readable project files | Pending |
| `projects.down-images` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/down-images`; Argv `down-images` | Docker Compose + readable project files | Pending |
| `projects.down-volumes-images` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/down-volumes-images`; Argv `down-volumes-images` | Docker Compose + readable project files | Pending |
| `projects.start` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/start`; Argv `start` | Docker Compose + readable project files | Pending |
| `projects.stop` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/stop`; Argv `stop` | Docker Compose + readable project files | Pending |
| `projects.restart` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/restart`; Argv `restart` | Docker Compose + readable project files | Pending |
| `projects.pause` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/pause`; Argv `pause` | Docker Compose + readable project files | Pending |
| `projects.unpause` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/unpause`; Argv `unpause` | Docker Compose + readable project files | Pending |
| `projects.pull` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/pull`; Argv `pull` | Docker Compose + readable project files | Pending |
| `projects.build` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/build`; Argv `build` | Docker Compose + readable project files | Pending |
| `projects.rebuild` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/rebuild`; Argv `rebuild` | Docker Compose + readable project files | Pending |
| `projects.recreate` | Compose project argv | `TestEveryAdvertisedDemoAction/projects/recreate`; Argv `recreate` | Docker Compose + readable project files | Pending |
| `projects.logs-follow` | Native terminal + Docker/Compose argv | `TestEveryAdvertisedDemoAction/projects/logs-follow` | Docker Compose + readable project files; terminal emulator | Pending |
| `images.remove` | Image.Remove / explicit parent closure | `TestEveryAdvertisedDemoAction/images/remove`; SDK `images/remove` | Local Docker socket | Pending |
| `images.remove-no-prune` | Image.Remove / explicit parent closure | `TestEveryAdvertisedDemoAction/images/remove-no-prune`; SDK `images/remove-no-prune` | Local Docker socket | Pending |
| `images.remove-force` | Image.Remove / explicit parent closure | `TestEveryAdvertisedDemoAction/images/remove-force`; SDK `images/remove-force` | Local Docker socket | Pending |
| `images.remove-force-no-prune` | Image.Remove / explicit parent closure | `TestEveryAdvertisedDemoAction/images/remove-force-no-prune`; SDK `images/remove-force-no-prune` | Local Docker socket | Pending |
| `images.prune` | Preview eligible IDs, then exact-ID removal | `TestEveryAdvertisedDemoAction/images/prune`; SDK `images/prune` | Local Docker socket | Pending |
| `images.prune-all` | Preview eligible IDs, then exact-ID removal | `TestEveryAdvertisedDemoAction/images/prune-all`; SDK `images/prune-all` | Local Docker socket | Pending |
| `volumes.remove` | Volume.Remove | `TestEveryAdvertisedDemoAction/volumes/remove`; SDK `volumes/remove` | Local Docker socket | Pending |
| `volumes.remove-force` | Volume.Remove | `TestEveryAdvertisedDemoAction/volumes/remove-force`; SDK `volumes/remove-force` | Local Docker socket | Pending |
| `volumes.prune` | Preview eligible IDs, then exact-ID removal | `TestEveryAdvertisedDemoAction/volumes/prune` | Local Docker socket | Pending |
| `networks.remove` | Network.Remove | `TestEveryAdvertisedDemoAction/networks/remove`; SDK `networks/remove` | Local Docker socket | Pending |
| `networks.prune` | Preview eligible IDs, then exact-ID removal | `TestEveryAdvertisedDemoAction/networks/prune` | Local Docker socket | Pending |
| `commands.run` | Configured argv subprocess | `TestEveryAdvertisedDemoAction/commands/run` | Configured executable and cwd | Pending |

SDK test prefix: `TestLazydockerLifecycleAndResourceRemovalFlags`. Argv test prefix: `TestComposeArgvEveryOperationAndInjection`. Terminal argument-array coverage is `TestTerminalExecAttachAndCustomUseArgumentArrays`. A running native terminal supplies real PTY input and resizing; the application does not misrepresent captured output as an interactive terminal. Interactive exec, attach, follow and attached-up require human validation on the desktop.

## Detail tabs

| Feature ID | Source / behavior | Automated evidence | Human |
|---|---|---|---|
| `containers.logs` | Bounded Docker/Compose log snapshot | `TestEveryAdvertisedDetailAndStats/containers/logs` (demo) | Pending |
| `containers.stats` | Lazydocker-derived CPU/memory + configured graphs | `TestEveryAdvertisedDetailAndStats/containers/stats` (demo) | Pending |
| `containers.env` | Environment names masked until Reveal | `TestEveryAdvertisedDetailAndStats/containers/env` (demo) | Pending |
| `containers.inspect` | Docker inspect; sensitive fields masked until Reveal | `TestEveryAdvertisedDetailAndStats/containers/inspect` (demo) | Pending |
| `containers.top` | Lazydocker RenderTop / Compose top | `TestEveryAdvertisedDetailAndStats/containers/top` (demo) | Pending |
| `services.logs` | Bounded Docker/Compose log snapshot | `TestEveryAdvertisedDetailAndStats/services/logs` (demo) | Pending |
| `services.stats` | Lazydocker-derived CPU/memory + configured graphs | `TestEveryAdvertisedDetailAndStats/services/stats` (demo) | Pending |
| `services.env` | Environment names masked until Reveal | `TestEveryAdvertisedDetailAndStats/services/env` (demo) | Pending |
| `services.inspect` | Docker inspect; sensitive fields masked until Reveal | `TestEveryAdvertisedDetailAndStats/services/inspect` (demo) | Pending |
| `services.top` | Lazydocker RenderTop / Compose top | `TestEveryAdvertisedDetailAndStats/services/top` (demo) | Pending |
| `projects.logs` | Bounded Docker/Compose log snapshot | `TestEveryAdvertisedDetailAndStats/projects/logs` (demo) | Pending |
| `projects.config` | Explicit configuration view; protected content requires Reveal | `TestEveryAdvertisedDetailAndStats/projects/config` (demo) | Pending |
| `images.inspect` | Docker inspect; sensitive fields masked until Reveal | `TestEveryAdvertisedDetailAndStats/images/inspect` (demo) | Pending |
| `images.history` | Lazydocker Image.RenderHistory, including ancestor layers; ANSI removed | `TestEveryAdvertisedDetailAndStats/images/history` (demo) | Pending |
| `volumes.inspect` | Docker inspect; sensitive fields masked until Reveal | `TestEveryAdvertisedDetailAndStats/volumes/inspect` (demo) | Pending |
| `networks.inspect` | Docker inspect; sensitive fields masked until Reveal | `TestEveryAdvertisedDetailAndStats/networks/inspect` (demo) | Pending |
| `commands.config` | Explicit configuration view; protected content requires Reveal | `TestEveryAdvertisedDetailAndStats/commands/config` (demo) | Pending |
| `host.info` | Docker Engine information | `TestEveryAdvertisedDetailAndStats/host/info` (demo) | Pending |
| `host.events` | Bounded last-hour Docker events | `TestEveryAdvertisedDetailAndStats/host/events` (demo) | Pending |
| `host.disk` | Docker DiskUsage | `TestEveryAdvertisedDetailAndStats/host/disk` (demo) | Pending |
| `host.config` | Explicit configuration view; protected content requires Reveal | `TestEveryAdvertisedDetailAndStats/host/config` (demo) | Pending |

Real daemon validation is separate from these demo detail tests. Sensitive-read and reveal behavior is tested by `TestSensitiveReadAuthenticationAndRedaction`; CIFS/network driver option payloads by `TestDriverOptionsMaskedUntilReveal`.

## Functional settings

Settings use this application's JSON format, not Lazydocker's YAML schema. Existing Lazydocker YAML is neither overwritten nor automatically imported.

| Setting / capability | JSON field or behavior | Verification | Human |
|---|---|---|---|
| Compose project roots and files | `projects:[{id,path,files}]`; missing roots are inferred from Compose labels | Config validation; Compose argv tests; live fixture check | Pending |
| Custom commands and shell commands | `commands:[{id,label,scope,argv,cwd,terminal,serviceNames}]`; a shell is used only if explicitly configured in argv | `TestTerminalExecAttachAndCustomUseArgumentArrays` | Pending |
| Selected-object templates | Go-template metadata fields such as `.Container.ID`, `.Service.Name`, `.Image.ID`, `.Volume.Name`, `.Network.Name`; `{id}`, `{name}`, `{project}`, `{service}` aliases | `TestCustomTemplatesHaveNoExecutableMethodsAndFilterServices` | Pending |
| Compose command template token | A whole argv entry `{{ .DockerCompose }}` or `{compose}` expands to argument-separated Docker Compose project flags | Same template regression | Pending |
| Service-name filtering | `commands[].serviceNames` limits service-scoped commands | Same template regression | Pending |
| Per-operation command overrides | `commandTemplates:{"services.restart":["{compose}","restart","{service}"]}`; arguments are resolved and frozen before confirmation | `TestLogsAutoscaleAndOperationTemplateConfiguration`, `TestConfirmedCommandsFreezeConfiguration` | Pending |
| Native terminal selection | `terminal:["x-terminal-emulator","-e"]`; prefix frozen with the confirmed command | `TestConfirmedCommandsFreezeConfiguration` | Pending |
| Log lookback / lines / timestamps | `logs:{since:"60m",tail:300,timestamps:false}` | `TestLogsAutoscaleAndOperationTemplateConfiguration`; live logs | Pending |
| Numeric custom graph paths | `stats:[{label,statPath,min,max}]`; paths address `ClientStats` and `DerivedStats` | `TestCustomStatsPathHistoryWindowAndUnits` | Pending |
| Automatic graph bounds | Null/omitted min/max or `autoMin`/`autoMax` use the active history range | `TestLogsAutoscaleAndOperationTemplateConfiguration` | Pending |
| Graph history duration | `statsMaxDuration:"2m"`; maximum 24 hours and 2048 retained samples per series | `TestCustomStatsPathHistoryWindowAndUnits` | Pending |
| Config editing/opening equivalent | Native Settings editor, validation and atomic file replacement | `TestConfigValidationAndJobCompletion` | Pending |

Example settings:

```json
{
  "projects": [],
  "commands": [
    {"id":"service-status","label":"Service status","scope":"services","argv":["{{ .DockerCompose }}","ps","{{ .Service.Name }}"],"cwd":"","terminal":false,"serviceNames":["api"]}
  ],
  "terminal": ["x-terminal-emulator", "-e"],
  "logs": {"since":"60m","tail":300,"timestamps":false},
  "statsMaxDuration": "2m",
  "stats": [
    {"label":"CPU (%)","statPath":"DerivedStats.CPUPercentage","min":0,"max":100},
    {"label":"Memory bytes","statPath":"ClientStats.MemoryStats.Usage","min":null,"max":null}
  ],
  "commandTemplates": {}
}
```

Templates receive plain metadata, never executable Lazydocker objects. They cannot call `.Container.Start` or other Docker methods while rendering a preview. The supported metadata fields are identity/name/project/service fields; arbitrary upstream Go-object methods are intentionally unavailable. Read-only detail rendering remains controlled by the application's typed implementation and log/graph settings; operation overrides are executable actions with confirmation.

## Performance and safety regression coverage

- `TestDashboardPollingDoesNotInspectEveryContainer`: five dashboard reads cause one cached container list, one disk-usage call and one network list, with no per-container inspect.
- Container inventory cache: 2 seconds. Disk usage/network totals: 30 seconds. Invalidation uses a generation counter so an in-flight older fetch cannot repopulate stale data (`TestCacheInvalidationRejectsInflightStaleWrite`).
- Background metrics sampling is separate from UI requests, limited to four concurrent running-container stats calls and an eight-second sampling deadline. Recent events and top consumers are snapshots. Native host CPU/memory/filesystem meters use `/proc`/`statfs`.
- Live actions run as jobs; SDK timeout is 45 seconds so Docker's normal ten-second stop grace does not race the old ten-second client timeout. Jobs retain partial counts and errors.
- `TestConfirmationOneUseExpiryAndChanges`: expired/replayed confirmation and changed identities/config are refused.
- `TestPruneFreezesTargetsAndRevalidatesEligibility` and `TestPruneRefusesNowRunningTarget`: prune cannot silently expand or delete a newly running candidate.
- `TestImageParentClosureAndRevalidation`: automatic image parents must remain untagged and eligible; explicit removal order is child before parent.
- `TestConfirmedCommandsFreezeConfiguration`: confirmed argv, cwd and terminal cannot change underneath a queued job.
- `TestLocalDockerClientRejectsRemoteEndpoints`: the native SDK never inherits a remote Docker endpoint.
- `TestLegacyMutationEndpointDisabled`: direct lifecycle calls cannot bypass the preview/confirmation API.

## Explicit boundaries

Standalone container recreation and standalone image builds are not dedicated actions in the pinned upstream menus; recreation/rebuild operate through Compose, and additional recipes can be configured as explicit custom argv commands. Global build-cache pruning is not an upstream core menu action and is not advertised. GUI-specific filtering, selection, navigation and layout tests belong to the frontend checklist.

The application implements the listed Docker/Compose operations, not every TUI appearance/keybinding preference. It does not import upstream YAML configuration or expose arbitrary Go object methods to templates. Docker socket access is root-equivalent. User-configured commands execute with the desktop user's privileges; they are not a sandbox. Configuration, terminal interactivity, destructive global cleanup and workstation-specific project recipes still require human validation. No test listed here claims that human validation has occurred.
