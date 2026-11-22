# Upstream dependency notices

These files fill gaps in the published Cargo archives used by the native release.
`sources.json` records immutable upstream commit URLs and SHA-256 hashes. The
packager checks these hashes before copying notices into the Debian package.
Dependency versions and the complete collected notice inventory are included in
`/usr/share/doc/lazyconteiner/dependency-notices.json`.

`drm-fourcc` 2.2.0 declares MIT in its manifest but provides no standalone license
file at the pinned revision. Its original manifest and README preserve that
statement and the upstream contributor attribution. This exception remains
explicit in the generated notices rather than inventing a copyright notice.

These third-party files retain their original terms; the repository's MIT license
does not replace them or the separate Pagedone icon terms.
