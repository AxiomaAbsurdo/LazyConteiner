# Arch Linux bundle for v1.0.0

LazyConteiner's Arch bundle is an additional package for the **existing v1.0.0
release**, targeting an up-to-date **Arch Linux x86_64** desktop. The package name
is `lazyconteiner-bin`. It is distributed through GitHub Releases, not the official
Arch repositories or the AUR.

## Install

Download these assets from the [v1.0.0 release](https://github.com/AxiomaAbsurdo/LazyConteiner/releases/tag/v1.0.0):

- `lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst`
- `lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst.sha256`

Review your normal rolling-release update requirements. Arch does not support
partial upgrades; update the system before installing a new package:

```sh
sudo pacman -Syu
sha256sum -c lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst.sha256
sudo pacman -U ./lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst
lazyconteiner --version
lazyconteiner --demo
```

Open **LazyConteiner** in your application menu, or close demo mode and run
`lazyconteiner` for live Docker mode. Run as your regular desktop user. Demo mode
does not access Docker. Live mode needs an existing accessible local Docker Unix
socket; install/configure Docker and Compose using [Arch's Docker guide](https://wiki.archlinux.org/title/Docker).

The package does not enable a daemon, change group membership or create a service.
`docker`, `docker-compose` and `xdg-utils` are optional dependencies for the
corresponding live operations. Rootful Docker socket access is root-equivalent.

## What is packaged

The recipe pins the published Debian v1.0.0 archive by SHA-256, extracts its payload
with `bsdtar`, and builds a proper pacman package using `makepkg`. The native GUI
and static Go backend are preserved byte-for-byte. No `dpkg` or Debian maintainer
scripts execute on Arch. Dependency names, package metadata, license locations
and terminal integration are adapted for Arch.

The original v1 terminal default is `x-terminal-emulator -e`. Arch does not use
Debian's alternatives system. This bundle includes a private alias to `xterm` and
adds that private directory to the app's child-process PATH. The adapter selects
DejaVu Sans Mono, already installed for the UI, instead of relying on legacy
bitmap fonts. `xterm` is a package
dependency, so the default interactive command has a terminal available. This
creates no system-wide alias and does not change your preferred terminal. Settings
can still specify another installed terminal and its supported arguments.

All eleven application icons and the desktop entry are included. Arch's existing
cache hooks handle icon/menu updates. The package has no root install script.
Configuration and logs retain their normal XDG paths. Application MIT, Pagedone
and bundled-dependency notices are preserved under `/usr/share/doc/lazyconteiner/`
and `/usr/share/licenses/lazyconteiner-bin/`.

## Build from the recipe

The release also includes `lazyconteiner-bin-1.0.0-1.src.tar.gz` and its checksum.
This is the **packaging recipe**, not a full source build of Exact and the backend;
it downloads the pinned prebuilt v1.0.0 payload.

On Arch, install `base-devel` through your normal system update workflow, unpack
the source archive, and review its `PKGBUILD`, patch and `ARCH.md`. Run `makepkg`
as a regular user:

```sh
makepkg --verifysource
makepkg -s
sudo pacman -U ./lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst
```

`makepkg -s` resolves missing dependencies with pacman. To inspect packaging
metadata and diagnostics, install `namcap` and use:

```sh
makepkg --printsrcinfo
namcap PKGBUILD
namcap lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst
pacman -Qip lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst
pacman -Qlp lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst
```

For maintainers on Debian, `packaging/arch/Dockerfile` builds with a non-root
`makepkg` user in the official Arch image. Its separate runtime stage installs the
package without build tools. The check stage adds Xvfb and verification utilities
only; it does not mount a host Docker socket or desktop.

```sh
docker build --target check -t lazyconteiner-arch-check:1.0.0 \
  -f packaging/arch/Dockerfile packaging/arch
docker run --rm --network none --cap-drop ALL --security-opt no-new-privileges \
  lazyconteiner-arch-check:1.0.0
```

The build uses Arch's current repositories, with `.BUILDINFO` recording installed
build dependencies. It is not a claim of reproducibility across future rolling
snapshots. A container is also not equivalent to Arch's recommended devtools clean
chroot or to testing every physical desktop environment.

## Upgrade, removal and rollback

Install a newer compatible package using `sudo pacman -U ./<package>.pkg.tar.zst`.
GitHub downloads do not configure a pacman repository or automatic updates.

```sh
sudo pacman -R lazyconteiner-bin
```

This removes the app while preserving user configuration/logs, Docker resources
and dependencies. Retain the previous package file if you need to roll back with
`pacman -U`. Do not downgrade system libraries to match an old package.

## Verification

See [Arch validation evidence](ARCH-VALIDATION.md) for actual checks and outstanding
human desktop/live-Docker validation. This bundle does not establish support for
ARM, Windows, macOS, or other Arch-derived distributions.

## Packaging references

- [Creating packages](https://wiki.archlinux.org/title/Creating_packages): makepkg,
  staging, testing, namcap and clean-environment guidance.
- [PKGBUILD](https://wiki.archlinux.org/title/PKGBUILD): metadata, sources and checksums.
- [Arch package guidelines](https://wiki.archlinux.org/title/Arch_package_guidelines):
  filesystem layout, dependencies and package conventions.
