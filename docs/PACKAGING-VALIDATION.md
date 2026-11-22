# Debian release candidate validation

Validated locally on Debian 13 (trixie), amd64, on 2026-11-22.
Artifact: `dist/lazyconteiner_1.0.0~rc1-1_amd64.deb`, with adjacent SHA-256 file.
The sections below retain the candidate evidence. Production v1.0.0 validation is recorded separately.

## Icon update: rc1-2

The user confirmed the installed rc1-1 package works. Revision `1.0.0~rc1-2`
adds the supplied application icon at nine PNG sizes plus scalable and symbolic
SVG variants. All 11 packaged icons match `lazycontainers-icon.zip` byte-for-byte;
PNG dimensions, SVG syntax and desktop-entry identity pass validation.
The package declares `hicolor-icon-theme` for Debian's standard cache trigger.
Lintian passes, the archive checksum passes, and APT simulates one upgrade from
rc1-1 with no new packages or removals on this host. The launcher, native GUI and
backend binaries are byte-identical to the user-validated rc1-1 package.
The rc1-2 upgrade has not been installed by the agent.

## Passed

- Production-profile Exact build with the pinned X11 and scrollbar patches; stripped native GUI and static Go bridge.
- Archive files owned by root, directories/executables mode 0755, data mode 0644; no maintainer scripts.
- SHA-256 archive check and all packaged md5sums verified after extraction.
- Desktop entry validation and lintian error gate. One expected warning: `initial-upload-closes-no-bugs` (Debian archive tracking). The intentional static Go backend has a narrowly scoped override.
- APT installation simulation: one new package (LazyConteiner), zero upgrades and zero removals on this host.
- Extracted launcher runs outside the checkout with `PATH=/usr/bin:/bin` and isolated XDG directories.
- Version output, authenticated demo backend, occupied-port refusal, duplicate-instance refusal, immediate restart and 0600 token/log permissions.
- SIGTERM and backend-failure handling stop owned processes and remove temporary credentials.
- Physical X11 clicks on all 11 navigation options in expanded and collapsed sidebars update the selected background and clear the previous selection.
- Native screenshot reviewed: local Pagedone icons, selected Settings option and visible scrollbar render correctly. Local evidence: `.runtime/package-selected.png`.

Runnable package checks: `make package-check`; add `--gui` to the command in
`PACKAGING.md` for desktop lifecycle checks. Physical clicks used the existing
native XTest helpers against the extracted package. No existing Docker workloads
were changed; demo tests pointed Docker at a nonexistent socket.

The requested TypeSafe packaging judgment selected `.deb` with confidence 1.0
over Warp, wraplinux and insufficient evidence. This supports the format choice;
the tests above supply runtime evidence.

## Remaining checks

The user confirmed the installed rc1-1 package works on this host. Actual APT
installation/removal, desktop-menu launch and upgrade on a clean machine remain
unverified. The current host already has the declared runtime
libraries. Debian 12, ARM64 and other distributions are not validated. The user's
Docker-operation and interactive-terminal acceptance rounds remain pending.
The package does not install a Docker daemon or grant Docker permissions.

Production Exact ignores `EXACT_SMOKE`; the installed `--smoke-test` intentionally
checks executables and the demo bridge only. GUI validation uses a real window.
The dependency notice inventory documents drm-fourcc's upstream MIT declaration
without inventing a missing upstream copyright notice.

## Production v1.0.0

The final `lazyconteiner_1.0.0-1_amd64.deb` was rebuilt from the release sources.
Fresh gates passed: Exact contract validation, Go race tests, 16 Bun adapter
tests, 16 authenticated native tests and the production package checks with
`--gui`. APT simulated an upgrade without package removals. All package integrity,
permission, occupied-port, duplicate-session and cleanup assertions passed.
Five new native demo screenshots were captured for the product media kit.

The user authorized the first production release after confirming the installed
package and icon. This authorization does not replace the outstanding per-feature
human and clean-machine checks recorded above. See `RELEASE-v1.0.0.md`.
