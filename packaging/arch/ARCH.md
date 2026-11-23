# LazyConteiner 1.0.0 for Arch Linux

This is an additional x86_64 distribution package for the existing v1.0.0
release. It repackages the published Linux GUI and Go backend without rebuilding
or changing those binaries. It is not a new application release or an official
Arch repository/AUR package.

## Install

Use an up-to-date Arch Linux x86_64 desktop. Arch does not support partial upgrades.
Review your normal update requirements before proceeding:

```sh
sudo pacman -Syu
sha256sum -c lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst.sha256
sudo pacman -U ./lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst
lazyconteiner --version
lazyconteiner --demo
```

Download both package and checksum from the existing v1.0.0 GitHub Release:
https://github.com/AxiomaAbsurdo/LazyConteiner/releases/tag/v1.0.0

Run as your regular desktop user, not with sudo. Close demo mode before running
`lazyconteiner` for live mode. An accessible existing local Docker Unix socket is
required for live mode. Docker and Compose are optional dependencies because demo
mode does not need them. This package does not install/enable a Docker daemon,
change groups, expose TCP ports or create a system service.

For live workflows, install/configure Docker and Compose according to Arch's
Docker documentation: https://wiki.archlinux.org/title/Docker
Confirm `docker info` works as your normal user. Docker socket access remains
root-equivalent for a rootful daemon; rootless Unix sockets are also supported.

## Terminal and desktop integration

The original v1 default is Debian's `x-terminal-emulator -e`. This package depends
on `xterm` and adds a private alias under `/usr/lib/lazyconteiner/bin/`, visible
only to the app and its child commands through PATH. No system-wide alternatives
or terminal settings are changed. You can select another terminal through the
app's Settings; use its supported command-line arguments.

The desktop entry and all eleven application icon variants are included. Standard
Arch hooks refresh the desktop/icon caches; no custom root install scripts are
used. Settings and private logs keep the same XDG locations as the Debian build.
The installed Debian-oriented upstream README describes application behavior;
this file supplies the Arch-specific installation instructions.

## Build and inspect the package

The recipe consumes the published v1.0.0 Debian archive as a binary payload,
checks its pinned SHA-256, extracts it with bsdtar, and stages files with makepkg.
No Debian package manager or Debian maintainer scripts run on Arch. The `-bin`
suffix explicitly identifies a prebuilt-binary package.

Install Arch's `base-devel`, then inspect the PKGBUILD, patch and this file before
running the recipe as an ordinary user:

```sh
makepkg --verifysource
makepkg -s
makepkg --printsrcinfo
namcap PKGBUILD
namcap lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst
```

`makepkg -s` uses pacman to resolve missing build/runtime dependencies. Install
`namcap` separately if you want its packaging diagnostics. The installer includes
all collected dependency notices and the separate Pagedone terms.

## Upgrade and remove

Install another compatible package using `sudo pacman -U ./<package>.pkg.tar.zst`.
GitHub downloads do not configure a pacman repository or automatic updates.

```sh
sudo pacman -R lazyconteiner-bin
```

Removal leaves your user settings/logs, Docker resources and dependencies intact.
For rollback, retain the earlier package file and install it with `pacman -U`.
No ARM build is supplied. Rolling-release compatibility needs continued testing;
a package built successfully is not a substitute for desktop and live-Docker
validation. See the repository's docs/ARCH.md for the recorded verification.
