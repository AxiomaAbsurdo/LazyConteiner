# Arch bundle validation

This records the additional Arch Linux x86_64 package for **LazyConteiner v1.0.0**.
It does not change the released application version or rebuild its GUI/backend.

## Environment

- Official Arch container image: `archlinux@sha256:d4594bb2300cba11f41c2f27923eb56f63d9988909c402f9c95517d1e362c899`.
- Image build ID: `20261108.0.623074`; repositories fully updated during validation.
- makepkg 7.1.0, namcap 3.6.0-3, Python 3.14.10-1.
- glibc `2.44+r78+gfcfb867c58ab-1`; libgcc/libstdc++ `16.2.1+r23+gd564253eb6c8-1`.
- Non-root package build. A separate runtime image installed the package through
  pacman without build tools. The container image's documentation exclusions were
  removed for full installed-file checks.
- Native tests ran with Xvfb at 1440×900, no network, all Linux capabilities
  dropped, and no host Docker socket or desktop mounted.

## Passed checks

- PKGBUILD syntax, fixed source SHA-256 validation, `makepkg` and source-recipe generation.
- Generated `.SRCINFO`, `.PKGINFO`, `.BUILDINFO` and `.MTREE`; no `.INSTALL` script.
- Root-owned package files; ordinary executable/data modes; expected license symlinks.
- Both compiled binaries match the published Debian v1.0.0 payload byte-for-byte.
- All eleven icon files match the supplied application icons; desktop-entry validation passes.
- The source archive matches the checked-in recipe, patch and documentation.
  makepkg adds comments to its archived `.SRCINFO`; metadata fields match.
- pacman dependency resolution and installation in the separate Arch runtime.
- `pacman -Qkk`: **822 total files, 0 altered files**.
- Dynamic library resolution, version output and authenticated demo backend smoke check.
- Actual native GUI window presented under Arch/Xvfb; [captured screenshot](arch-demo.png).
- Private terminal adapter inherited by the backend; xterm executed a shell command
  and wrote its expected output. The bundled DejaVu font avoids legacy-font warnings.
- Duplicate-instance refusal, 0600 token permissions, owned process/token cleanup,
  and immediate restart.
- `pacman -R` removes the app while retaining the terminal dependency; reinstall
  from the package succeeds and the regular-user smoke check passes again.

## namcap diagnostics

No `E:` diagnostics were reported. Warnings remain and were reviewed:

- The unchanged v1 static Go backend lacks PIE/FULL RELRO according to namcap.
  This distribution package preserves the release binary; it does not claim to
  change its build hardening.
- The interpreter scanner reports `python`/`sh` as uninstalled dependencies even
  though pacman confirms their ownership by the explicitly required `python` and
  `bash` packages. Both interpreters ran successfully in the runtime checks.
- Libraries loaded dynamically, fonts and the external terminal are reported as
  potentially unnecessary because static ELF scanning does not see those uses.
  They are retained to support the actual native window and terminal behavior.
- namcap reports the GUI's ELF interpreter as an unused shared library.

The requested TypeSafe packaging judgment selected an Arch binary package for
this same-version distro addition with confidence 1.0. The runtime checks above,
not that model confidence, establish the recorded test evidence.

## Artifacts

| Artifact | Bytes | SHA-256 |
| --- | ---: | --- |
| `lazyconteiner-bin-1.0.0-1-x86_64.pkg.tar.zst` | 22910256 | `8b847f8f8c5affe035916896ec96540320e7135a41b2b53a927bcf8c11eaa99e` |
| `lazyconteiner-bin-1.0.0-1.src.tar.gz` | 3854 | `c1217afcd8ceae7fdb1e8866a2cc284792cb3aba765c912702cbcecabddce413` |

Each artifact has an adjacent `.sha256` file on the existing v1.0.0 release.
The recipe pins the input Debian artifact to
`f2385cf41143d1672fb1875d8646638e7d37c16d998ff61a7e7ef185b65bcaf8`.

## Remaining human/environment checks

A physical Arch desktop session, native Wayland, different graphics drivers,
interactive terminal typing/resizing and live Docker/Compose operations on Arch
have not been manually validated. Tests used demo data and never connected to a
Docker daemon. Container validation is not a devtools clean-chroot certificate or
a claim of support for every future rolling snapshot or Arch-derived distro.
The source recipe is not submitted to the AUR.

Reproduce the build and container checks with the commands in [ARCH.md](ARCH.md).
