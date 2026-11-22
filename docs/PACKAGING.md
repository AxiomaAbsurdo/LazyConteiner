# Debian package

The first installer targets **Debian 13 (trixie), amd64**. It contains the compiled
Exact GUI, the Go bridge, local SVG assets, dependency notices, a desktop menu
entry, and a Python standard-library launcher. Users need no Rust, Go, Bun, or
source checkout. The application runs as the logged-in desktop user.

## Install and run

From the directory containing the downloaded release package:

```sh
sha256sum -c lazyconteiner_1.0.0-1_amd64.deb.sha256
sudo apt install ./lazyconteiner_1.0.0-1_amd64.deb
lazyconteiner --version
lazyconteiner --demo
lazyconteiner
```

Alternatively, open **LazyConteiner** from the desktop application menu. The menu
also provides a Demo action. Close a development run before starting the packaged
app: both use port 17878. Run the application without sudo.

APT installs declared desktop libraries and Python dependencies. Docker CLI,
Compose and a terminal emulator are recommended for operations that invoke them.
An existing accessible local Docker daemon is required for live mode. Installation
does not install or configure a Docker daemon, change groups, or create a system
service. Demo mode works without Docker. Rootless Unix sockets are supported.

For a headless installation check:

```sh
lazyconteiner --smoke-test
```

This checks the packaged executables and boots an authenticated demo bridge, then
exits. It does not boot the GUI or validate rendering or terminal interaction.
Exact's production trust policy disables its development-only headless UI mode;
the package preserves that policy. Test the actual window with `--demo`.

## Build

Build on Debian 13 amd64 after the development prerequisites in the README and
`make setup`. Packaging also needs `dpkg-dev`, `desktop-file-utils`, `xz-utils`
and, for package checking, `lintian`.

```sh
make package
make package-check
```

`make package` builds a stripped static Go backend and a release-profile native
GUI with the pinned Exact runtime patches. It derives linked-library requirements
with `dpkg-shlibdeps`, adds libraries loaded dynamically, collects dependency
notices and writes a root-owned archive plus a SHA-256 checksum under `dist/`.
It does not install anything. The application MIT license and third-party terms
are included separately. Missing notices fail the build; the drm-fourcc upstream
license-declaration exception is documented in `packaging/licenses/README.md`.

For packaging-only iteration after a successful release build:

```sh
python3 scripts/build-deb.py --skip-build
```

The default version is `1.0.0-1`, which sorts after the `1.0.0~rc1` candidates in Debian.
`--version` sets another Debian version. `SOURCE_DATE_EPOCH` controls archive
metadata timestamps; this does not claim bit-identical native compilation across
machines. `--skip-build` requires the caller to ensure binaries match sources.

The current GUI requires GLIBC 2.39; Debian 12's GLIBC 2.36 cannot run it. Debian 12,
ARM64 and other distributions require separate builds and validation. Do not
rename this amd64 artifact to claim support for another architecture.

## Files and removal

- Launcher: `/usr/bin/lazyconteiner`
- Compiled binaries and assets: `/usr/lib/lazyconteiner/`
- Documentation and notices: `/usr/share/doc/lazyconteiner/`
- Configuration: `$XDG_CONFIG_HOME/lazyconteiner/config.json`, normally `~/.config/lazyconteiner/config.json`
- Logs: `$XDG_STATE_HOME/lazyconteiner/`, normally `~/.local/state/lazyconteiner/`
- Session lock and temporary token: `$XDG_RUNTIME_DIR/lazyconteiner/`, falling back to `$XDG_CACHE_HOME/lazyconteiner/`

The launcher authenticates bridge readiness, refuses an occupied port or duplicate
session, and stops its owned bridge/UI processes when closed. Temporary credentials
are removed on normal exit and handled signals. A forced kill or power loss can
leave a private session directory; it is never reused by the next launch.

```sh
sudo apt remove lazyconteiner
```

APT removal retains user configuration and logs. Delete those user directories
separately only if their contents are no longer wanted. Existing Docker resources
remain unchanged. Upgrades use `sudo apt install ./<new-version>.deb`. Downloads
from GitHub do not auto-update; an APT repository is not configured.

## Why a .deb

A `.deb` integrates dependency resolution, desktop menus, versioned upgrades and
removal with Debian's existing package manager. The app already has native
executables, so another executable wrapper is unnecessary.

[Warp](https://github.com/dgiagio/warp) creates a self-extracting executable and
caches extracted files in the user's data directory. It may suit an optional
portable download, but does not supply APT dependency management or remove GLIBC
requirements. [wraplinux](https://packages.debian.org/stable/wraplinux) wraps Linux
kernels and initrds in ELF or NBI boot formats. It is unrelated to desktop app
packaging. Neither is needed for this installer.

## Release validation

Automated package checks cover metadata, ownership, checksums, desktop entry,
APT installation simulation, authenticated bridge startup, token permissions, duplicate
and occupied-port refusal, and cleanup after signals or backend failure. Extracted
packages are tested outside the source checkout with a system-only PATH.
Run `python3 scripts/check-deb.py dist/lazyconteiner_1.0.0-1_amd64.deb --gui`
in a desktop session to include real native UI startup and lifecycle checks.
Lintian has a scoped override for the intentional static Go bridge. Its initial
upload warning concerns Debian archive bug tracking, not GitHub distribution.

Release evidence and remaining environment checks are tracked separately:

- [ ] Install and remove through APT on a clean Debian 13 amd64 desktop.
- [ ] Launch from the application menu; test demo rendering and window resizing.
- [ ] Complete Docker-operation and interactive-terminal rounds in `FEATURES.md`.
- [ ] Check upgrade from the release package to the final version.
- Release publication is tracked in `RELEASE-v1.0.0.md`; the unchecked environment checks above remain limitations, not implied passes.

No final v1 tag or release is created by the build commands.
