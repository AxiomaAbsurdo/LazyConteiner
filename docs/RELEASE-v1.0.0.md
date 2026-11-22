# LazyConteiner v1.0.0

**A GUI for containers—for those days when you’re not feeling nerdy enough for the terminal.**

The first production release brings a native local-Docker desktop interface to
**Debian 13 amd64**. It combines Expo Exact with Lazydocker's Go core and ships as
a `.deb` with compiled binaries, a desktop launcher and the application icon.

## Included

- Dashboard, filters, bulk selection and Docker resource inventory.
- Containers, Compose projects/services, images, volumes and networks.
- Logs, statistics, inspection, process lists, image history and events.
- Exact-target action previews, confirmations and asynchronous results.
- External-terminal integration for interactive operations.
- Settings and custom commands; isolated demo mode without Docker access.
- Responsive native layout, persistent scroll indicators and Pagedone interface icons.
- MIT application license, contribution guide, PayPal support link and product media kit.

## Install or upgrade

Download the `.deb` and its adjacent `.sha256` asset, then run:

```sh
sha256sum -c lazyconteiner_1.0.0-1_amd64.deb.sha256
sudo apt install ./lazyconteiner_1.0.0-1_amd64.deb
lazyconteiner --demo
```

APT upgrades the earlier release candidates in place. Close the demo and run
`lazyconteiner` for live mode, or use the application menu. Run as your normal
user. Docker and its socket permissions must already be configured for live mode;
the package does not provision a Docker daemon or change group membership.

## Release assets

- `lazyconteiner_1.0.0-1_amd64.deb`: Debian installer.
- `lazyconteiner_1.0.0-1_amd64.deb.sha256`: installer checksum.
- `lazyconteiner-v1.0.0-media-kit.zip`: real demo screenshots, application icon,
  full product description, storyboard and video-agent handoff.
- `lazyconteiner-v1.0.0-media-kit.zip.sha256`: media-kit checksum.

## Verification and limits

Release preparation includes Exact contract validation, Go race tests, 16 Bun
adapter tests, 16 authenticated native interaction/responsive tests, production
build and extracted-package checks. Package checks cover integrity, permissions,
APT upgrade simulation, duplicate/occupied-port refusal and process/token cleanup.
The user confirmed earlier installed candidates and the application icon work.

The feature inventory and prior disposable Docker checks are documented in
`docs/FEATURES.md`. Per-operation human acceptance, interactive terminal testing,
clean-machine installation/removal and wider desktop validation remain incomplete.
This release does not turn those pending checks into passes.

This installer targets Debian 13 amd64. Debian 12, ARM64, Windows and macOS are not
supported by this artifact. Remote Docker endpoints, Podman and Kubernetes are
not supported. Interactive terminals are external, not embedded. GitHub downloads
do not auto-update. Docker access remains root-equivalent; custom commands run
with the user's privileges.

Report reproducible issues on GitHub. See the README and `CONTRIBUTING.md` for
setup, logs, safe test selection and contribution guidelines.

Optional support: https://paypal.me/mmortara01
