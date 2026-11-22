#!/usr/bin/env python3
"""Build a Debian 13 amd64 release package without installing it on the host."""
import argparse
import gzip
import hashlib
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parent.parent


def run(*args, **kwargs):
    return subprocess.run(args, check=True, text=True, **kwargs)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', default='1.0.0-1')
    parser.add_argument('--skip-build', action='store_true', help='package already-built release binaries')
    args = parser.parse_args()
    if not re.fullmatch(r'[0-9][A-Za-z0-9.+~\-]*', args.version):
        parser.error('Version must be a Debian version without an epoch or path separators.')
    arch = run('dpkg', '--print-architecture', capture_output=True).stdout.strip()
    if arch != 'amd64':
        parser.error('This release package is verified for Debian 13 amd64 only.')
    exact = Path(os.environ.get('EXACT', ROOT / '.runtime/exact')).resolve()
    runtime = ROOT / '.runtime'
    runtime.mkdir(exist_ok=True)
    backend = runtime / 'backend-release'
    ui = ROOT / 'fleet/target/release/lazyconteiner-linux'
    if not args.skip_build:
        run('bash', str(ROOT / 'scripts/exact-patches.sh'), cwd=ROOT)
        run('go', 'build', '-trimpath', '-ldflags=-s -w', '-o', str(backend), '.', cwd=ROOT / 'backend', env={**os.environ, 'CGO_ENABLED': '0'})
        run('bun', str(ROOT / 'scripts/build-release.mjs'), cwd=ROOT)
    for binary in (backend, ui):
        if not binary.is_file():
            raise RuntimeError(f'Missing release binary: {binary}. Run without --skip-build.')
    epoch = int(os.environ.get('SOURCE_DATE_EPOCH', run('git', 'show', '-s', '--format=%ct', 'HEAD', cwd=ROOT, capture_output=True).stdout.strip()))
    dist = ROOT / 'dist'
    dist.mkdir(exist_ok=True)
    name = f'lazyconteiner_{args.version}_{arch}.deb'

    with tempfile.TemporaryDirectory(prefix='debian-', dir=runtime) as work:
        work = Path(work)
        stage = work / 'package'
        lib = stage / 'usr/lib/lazyconteiner'
        doc = stage / 'usr/share/doc/lazyconteiner'
        for directory in (lib, doc, stage / 'DEBIAN', stage / 'usr/bin', stage / 'usr/share/applications', stage / 'usr/share/icons/hicolor/scalable/apps', stage / 'usr/share/man/man1'):
            directory.mkdir(parents=True, exist_ok=True)
        shutil.copy2(backend, lib / 'backend')
        shutil.copy2(ui, lib / 'ui')
        shutil.copytree(ROOT / 'fleet/assets', lib / 'assets')
        shutil.copy2(ROOT / 'packaging/lazyconteiner', stage / 'usr/bin/lazyconteiner')
        (stage / 'usr/bin/lazyconteiner').chmod(0o755)
        (lib / 'VERSION').write_text(args.version + '\n')
        shutil.copy2(ROOT / 'packaging/debian/dev.axioma.lazyconteiner.desktop', stage / 'usr/share/applications/dev.axioma.lazyconteiner.desktop')
        overrides = stage / 'usr/share/lintian/overrides'
        overrides.mkdir(parents=True)
        shutil.copy2(ROOT / 'packaging/debian/lintian-overrides', overrides / 'lazyconteiner')
        shutil.copytree(ROOT / 'packaging/icons/hicolor', stage / 'usr/share/icons/hicolor', dirs_exist_ok=True)
        for file in ('README.md', 'docs/ICONS.md', 'docs/LICENSE-pagedone.txt', 'docs/PACKAGING.md'):
            shutil.copy2(ROOT / file, doc / Path(file).name)
        licenses = doc / 'licenses'
        licenses.mkdir()
        for source, target in [(ROOT / 'backend/LICENSE.lazydocker', 'Lazydocker'), (exact / 'LICENSE', 'Exact'), (exact / 'vendor/ibex/LICENSE', 'Ibex')]:
            shutil.copy2(source, licenses / target)
        if (ROOT / 'LICENSE').exists():
            shutil.copy2(ROOT / 'LICENSE', licenses / 'LazyConteiner')
        (doc / 'copyright').write_text(
            'LazyConteiner — https://github.com/AxiomaAbsurdo/LazyConteiner\n'
            'Copyright 2026 LazyConteiner contributors.\n'
            + ('Application license: see licenses/LazyConteiner.\n' if (ROOT / 'LICENSE').exists() else 'Application license: all rights reserved; public-release license not yet selected.\n')
            + 'Exact, Ibex and Lazydocker license texts are in licenses/.\n'
            'Pagedone SVGs retain their separate terms in LICENSE-pagedone.txt and ICONS.md.\n'
            'Additional bundled dependency notices are listed in THIRD-PARTY.txt.\n')
        run('python3', str(ROOT / 'scripts/package-notices.py'), str(doc), cwd=ROOT)
        date = time.strftime('%a, %d %b %Y %H:%M:%S +0000', time.gmtime(epoch))
        changelog = f'lazyconteiner ({args.version}) unstable; urgency=medium\n\n  * Native Debian release package.\n\n -- AxiomaAbsurdo <AxiomaAbsurdo@users.noreply.github.com>  {date}\n'
        (doc / 'changelog.Debian.gz').write_bytes(gzip.compress(changelog.encode(), mtime=epoch))
        man = (ROOT / 'packaging/debian/lazyconteiner.1').read_bytes()
        (stage / 'usr/share/man/man1/lazyconteiner.1.gz').write_bytes(gzip.compress(man, mtime=epoch))

        # Derive ABI requirements from these exact binaries; add libraries loaded at runtime.
        (work / 'debian').mkdir()
        control = (ROOT / 'packaging/debian/control').read_text()
        (work / 'debian/control').write_text(control)
        shlibs = run('dpkg-shlibdeps', '-O', '-e' + str(lib / 'ui'), '-e' + str(lib / 'backend'), cwd=work, capture_output=True)
        depends = next(line.split('=', 1)[1] for line in shlibs.stdout.splitlines() if line.startswith('shlibs:Depends='))
        depends += ', python3 (>= 3.9), hicolor-icon-theme, fontconfig-config, fonts-dejavu-core, libx11-6, libx11-xcb1, libxcb1, libxcb-shm0, libxcb-randr0, libxcb-render0, libxcb-shape0, libxcb-xfixes0, libxrandr2, libxi6, libxcursor1, libxkbcommon0, libxkbcommon-x11-0, libwayland-client0, libwayland-cursor0'
        installed_size = (sum(p.stat().st_size for p in (stage / 'usr').rglob('*') if p.is_file()) + 1023) // 1024
        binary_control = control.split('\n\n', 1)[1].replace('Architecture: amd64\n',
            f'Architecture: {arch}\nVersion: {args.version}\nSection: utils\nPriority: optional\n'
            'Maintainer: AxiomaAbsurdo <AxiomaAbsurdo@users.noreply.github.com>\n'
            'Homepage: https://github.com/AxiomaAbsurdo/LazyConteiner\n'
            f'Installed-Size: {installed_size}\nDepends: {depends}\n'
            'Recommends: docker-cli | docker-ce-cli, docker-compose | docker-compose-plugin, x-terminal-emulator, xdg-utils\n')
        (stage / 'DEBIAN/control').write_text(binary_control)
        files = sorted(p for p in (stage / 'usr').rglob('*') if p.is_file())
        (stage / 'DEBIAN/md5sums').write_text(''.join(f'{hashlib.md5(p.read_bytes()).hexdigest()}  {p.relative_to(stage)}\n' for p in files))
        executables = {lib / 'backend', lib / 'ui', stage / 'usr/bin/lazyconteiner'}
        for path in [stage, *stage.rglob('*')]:
            path.chmod(0o755 if path.is_dir() or path in executables else 0o644)
            os.utime(path, (epoch, epoch), follow_symlinks=False)
        run('desktop-file-validate', str(stage / 'usr/share/applications/dev.axioma.lazyconteiner.desktop'))
        output = dist / name
        run('dpkg-deb', '--root-owner-group', '-Zxz', '--threads-max=2', '--build', str(stage), str(output), env={**os.environ, 'SOURCE_DATE_EPOCH': str(epoch)})
        digest = hashlib.sha256(output.read_bytes()).hexdigest()
        (dist / (name + '.sha256')).write_text(f'{digest}  {name}\n')
        print(f'{output}\nSHA256 {digest}\nDepends: {depends}')


if __name__ == '__main__':
    main()
