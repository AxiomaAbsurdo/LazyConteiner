# Contributing to LazyConteiner

Thank you for helping make local Docker work easier. Small, focused changes are
welcome: bug fixes, Debian compatibility reports, accessibility improvements,
documentation and tests that reproduce real failures.

## Before you start

Check existing issues and open an issue before a large feature or architectural
change. Describe the user problem, expected behavior and smallest useful scope.
For fixes, include reproduction steps and the affected version.

Keep changes scoped. Preserve existing design, accessibility labels and responsive
behavior. Reuse the pinned Exact components and existing adapters before adding a
dependency. Changes to Docker actions must preserve exact-target confirmation,
authentication, cancellation and error reporting.

## Development setup

The supported development target is Debian 13 amd64 with an X11 or Wayland
desktop. Install Git, Python 3, Go 1.23.6+, Bun 1.4.2+, and Rust through rustup.
The pinned Exact setup installs its Rust toolchain and Hermes dependencies.

Typical Debian build and verification packages:

```sh
sudo apt install build-essential pkg-config clang cmake ninja-build libssl-dev \
  libx11-dev libx11-xcb-dev libxcb1-dev libxcb-shm0-dev libxcb-randr0-dev \
  libxcb-render0-dev libxcb-shape0-dev libxcb-xfixes0-dev libxrandr-dev \
  libxi-dev libxcursor-dev libxkbcommon-dev libxkbcommon-x11-dev \
  libwayland-dev libfontconfig1-dev fonts-dejavu-core dpkg-dev \
  desktop-file-utils lintian xz-utils
```

For physical X11 checks, also install `libxtst6`, `x11-utils` and `imagemagick`.
Docker and Compose are only required for live workflows and integration checks.
Use your existing Docker installation; this project does not provision a daemon.

Fork the repository, clone your fork and create a branch:

```sh
git clone https://github.com/YOUR_USERNAME/LazyConteiner.git
cd LazyConteiner
git switch -c fix/short-description
make setup
make demo
```

The first build downloads dependencies and takes longer. Later builds reuse
caches. `EXACT=/path/to/exact` can point to an existing checkout, which must match
the pinned revision. See `scripts/exact-setup.sh` and `patches/README.md`.

## Where to make changes

- `fleet/app.contract`, `fleet/workbench.contract`: native layout and styles.
- `fleet/*.ts`: typed UI state, data projection and action flows.
- `backend/`: local Docker access, configuration, previews and execution.
- `scripts/`: development, package builds and verification.
- `packaging/`: installed launcher, desktop entry, application icons and notices.
- `docs/`: feature evidence, performance, packaging and product media.

`vendor/`, the original artboard HTML and `support.js` are reference materials.
Do not mistake them for the native frontend.

## Verify your change

Run checks appropriate to the change, then report the results in your pull request:

```sh
make check                          # Exact contracts
make test                           # Go, Bun and authenticated native tests
cd backend && go test -race ./...    # backend race checks
```

From the repository root, use these when relevant:

```sh
make test-input                     # physical X11 input and resizing
make package                        # production .deb and checksum
make package-check                  # package checks, no host installation
```

Run GUI checks one at a time: competing windows can steal focus. Close an existing
LazyConteiner or development instance before tests that use port 17878.

`make test-live` creates and removes uniquely labeled disposable resources using
the existing `alpine:3.20` image. Read `scripts/live_check.py` first and run it only
on a daemon where you authorize those test mutations. Never use unrelated
workloads or global prune operations as test fixtures.

For UI changes, include screenshots of normal and narrow windows, confirm scroll
indicators remain visible, and verify physical clicks and keyboard access. Use
`--demo` captures so screenshots do not expose private workload data. Record
manual results honestly; automated checks are not human acceptance.

## Submit a pull request

1. Keep one problem or feature per pull request.
2. Explain the trigger, previous behavior and new behavior.
3. List checks run, their results and any checks not run.
4. Include relevant screenshots and note platform/version differences.
5. Update docs when commands, settings or user behavior change.
6. Use a clear commit message such as `fix: preserve selection after refresh`.

Do not commit runtime downloads, build outputs, `.deb` files, credentials, session
tokens, personal settings or private logs. Attach release packages to GitHub
Releases. Preserve third-party attribution and license notices.

## Report a bug

Open a GitHub issue with:

- LazyConteiner version (`lazyconteiner --version`).
- Debian release, architecture, desktop environment and X11/Wayland session.
- Demo or live mode; relevant Docker/Compose versions.
- Exact reproduction steps, expected result and actual result.
- A redacted screenshot or log excerpt, if useful.
- Whether the failure repeats after restarting the app.

Logs normally live in `~/.local/state/lazyconteiner/`. Never post API/session
tokens, environment secrets, registry credentials or sensitive workload details.

For suspected security vulnerabilities, avoid publishing exploit details in a
public issue. Use GitHub's private vulnerability-reporting feature if available;
otherwise request a private contact channel without disclosing the vulnerability.

Contributions to the application's own code are under the repository's MIT
license. Bundled third-party files retain their original terms.
