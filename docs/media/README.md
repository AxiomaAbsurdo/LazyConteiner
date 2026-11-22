# LazyConteiner product and video kit

Use this kit to introduce **LazyConteiner v1.0.0**, the native local-Docker GUI for
Debian 13 amd64. The screenshots show the real packaged application with isolated
demo fixtures. They do not show a user's production workloads.

## Product identity

**Name:** LazyConteiner (preserve this spelling in titles and captions).

**Tagline:** A GUI for containers—for those days when you’re not feeling nerdy enough for the terminal.

**Short description:** A native Debian desktop app for seeing and managing local
Docker containers, Compose projects, images, volumes and networks, with resource
details, explicit action previews and access to your external terminal.

**Audience:** Developers, self-hosters and technically curious Debian users who
already use Docker and want a visual way to inspect and manage their local work.

**Problem:** Routine container work is spread across commands: finding the right
resource, checking its state, inspecting output and deciding what to change.

**Product promise:** Bring that local Docker workflow into a clear desktop
interface while keeping exact targets visible and terminal tools available.

**Primary action:** Download the Debian installer from the GitHub release page.
**Secondary action:** Try `lazyconteiner --demo`, contribute, or support maintenance
through https://paypal.me/mmortara01.

Repository: https://github.com/AxiomaAbsurdo/LazyConteiner
Release: https://github.com/AxiomaAbsurdo/LazyConteiner/releases/tag/v1.0.0

## Complete product description

LazyConteiner combines a native Expo Exact frontend with a small Go bridge built
on Lazydocker's core. The app connects to an existing local Docker Unix socket.
Its dashboard and inventory pages help users navigate containers, Compose
projects and services, images, volumes and networks. Search, selection and
navigation update locally; detailed information is loaded as needed.

Users can inspect logs, statistics, process lists, environment information, Docker
metadata, image history and events. Resource actions include lifecycle and
cleanup workflows, with previews that identify exact targets before execution.
Jobs expose progress and errors. Settings provide project paths, terminal
configuration, operation overrides, custom commands and statistics options.
Interactive exec, attach and following logs use an external native terminal.

The Debian package contains compiled binaries, local icons and a desktop menu
entry. It requires no development toolchain on the user's machine and does not
install a Docker daemon or change Docker permissions. Demo mode offers a way to
explore the interface without touching Docker resources. The app runs as the
logged-in user; closing it stops its bridge without stopping Docker containers.

## Assets

| File | Intended use |
| --- | --- |
| [01-dashboard.png](01-dashboard.png) | Hero reveal; environment overview |
| [02-containers.png](02-containers.png) | Core workflow; container inventory and available actions |
| [03-images.png](03-images.png) | Breadth; image inventory and details |
| [04-stacks.png](04-stacks.png) | Compose project workflow |
| [05-narrow.png](05-narrow.png) | Responsive desktop layout and scrolling |
| [Application icon, 512 px](../../packaging/icons/hicolor/512x512/apps/dev.axioma.lazyconteiner.png) | Opening and end card |
| [Application icon, vector](../../packaging/icons/hicolor/scalable/apps/dev.axioma.lazyconteiner.svg) | Scalable branding |
| [Shot and narration brief](VIDEO-BRIEF.md) | Ready-to-use handoff to a video-making agent |

Keep screenshots legible. Preserve aspect ratios, demo labels, visible status and
application controls. Use tight crops only when they do not hide context needed
to understand an action. Do not redraw the interface or invent additional buttons.
A screenshot is a still image; camera motion over it is not proof of interaction.

## Visual direction

Use the application's navy, near-white and blue palette. Let the supplied icon
and real interface lead. Prefer clear typography, short labels and restrained
transitions. Present one task at a time. Leave enough dwell time to read the UI.
Deliver a 16:9 master; reframe selected scenes for a 9:16 teaser rather than
shrinking the entire desktop into a phone-sized frame.

## Claims and boundaries

Supported claims: native desktop window; local Docker and Compose workflows;
Debian `.deb` installation; local bundled icons; demo mode; exact-target previews;
external-terminal integration; MIT application code.

Do not claim Kubernetes, Podman, remote Docker management, a built-in terminal,
cloud hosting, automatic updates, cross-platform installers or zero-resource
usage. Do not describe every feature as manually certified. The feature checklist
still has pending operation-specific and interactive-terminal acceptance.

Avoid universal speed numbers. The performance report separates warm-cache API
measurements, cold requests and a tiny input fixture; none is a full-app guarantee.
Use “responsive local navigation” only alongside real footage showing it.

If demonstrating a destructive action, use demo data or explicitly disposable
resources. Do not show real environment secrets, credentials, hosts or workloads.
Never suggest the app removes the privilege implications of Docker socket access.

## Distribution and attribution

Use the application icon supplied for this project. Interface captures include
Pagedone icons used within the application; retain that context and do not turn
them into an independently distributed icon collection. Source attribution and
license details live in [ICONS.md](../ICONS.md).

The release's `lazyconteiner-v1.0.0-media-kit.zip` contains these captures, the
brief, product copy, application icon and third-party icon terms. This kit is a
production input, not an already-produced video or a video-rendering service.
