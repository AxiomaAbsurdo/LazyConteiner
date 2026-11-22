#!/usr/bin/env python3
"""Check a built .deb without installing it or changing Docker resources."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import signal
import socket
import stat
import subprocess
import tarfile
import tempfile
import time
import urllib.request

ADDRESS = ('127.0.0.1', 17878)


def run(*args, **kwargs):
    return subprocess.run(args, check=True, text=True, **kwargs)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('package', type=Path)
    parser.add_argument('--gui', action='store_true', help='also check native window lifecycle in this desktop session')
    args = parser.parse_args()
    package = args.package.resolve()
    expected = Path(str(package) + '.sha256').read_text().split()[0]
    assert hashlib.sha256(package.read_bytes()).hexdigest() == expected, 'Archive SHA-256 mismatch'
    metadata = run('dpkg-deb', '-f', str(package), capture_output=True).stdout
    package_version = next(line.split(': ', 1)[1] for line in metadata.splitlines() if line.startswith('Version: '))
    assert 'Architecture: amd64' in metadata
    assert 'Depends:' in metadata and 'python3' in metadata
    archive = subprocess.check_output(['dpkg-deb', '--fsys-tarfile', str(package)])
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        for member in tar:
            assert member.uid == member.gid == 0, member.name
            assert not member.mode & 0o6022, (member.name, oct(member.mode))
    run('lintian', '--fail-on', 'error', str(package))
    run('apt-get', '-s', 'install', str(package))
    with tempfile.TemporaryDirectory(prefix='lazy-package-check-') as directory:
        work = Path(directory)
        extracted = work / 'extracted'
        run('dpkg-deb', '-x', str(package), str(extracted))
        run('dpkg-deb', '-e', str(package), str(work / 'control'))
        assert not any((work / 'control' / name).exists() for name in ('preinst', 'postinst', 'prerm', 'postrm'))
        for line in (work / 'control/md5sums').read_text().splitlines():
            digest, name = line.split('  ', 1)
            assert hashlib.md5((extracted / name).read_bytes()).hexdigest() == digest, name
        run('desktop-file-validate', str(extracted / 'usr/share/applications/dev.axioma.lazyconteiner.desktop'))
        launcher = str(extracted / 'usr/bin/lazyconteiner')
        env = {**os.environ, 'PATH': '/usr/bin:/bin', 'DOCKER_HOST': 'unix:///nonexistent-package-check.sock'}
        for name in ('CONFIG', 'DATA', 'STATE', 'CACHE', 'RUNTIME'):
            path = work / name.lower()
            path.mkdir(mode=0o700)
            env['XDG_' + name + ('_DIR' if name == 'RUNTIME' else '_HOME')] = str(path)
        state = work / 'state/lazyconteiner'
        runtime = work / 'runtime/lazyconteiner'
        version = run(launcher, '--version', env=env, cwd=work, capture_output=True).stdout
        assert version.strip() == 'LazyConteiner ' + package_version, version
        # An existing unrelated listener must survive refusal by the launcher.
        with socket.socket() as listener:
            listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            listener.bind(ADDRESS)
            listener.listen()
            refused = subprocess.run([launcher, '--smoke-test'], env=env, cwd=work, capture_output=True, text=True, timeout=5)
            assert refused.returncode == 1 and 'already in use' in refused.stderr, refused.stderr
            assert listener.getsockname() == ADDRESS
        smoke = run(launcher, '--smoke-test', env=env, cwd=work, capture_output=True, timeout=75)
        print(smoke.stdout.strip())
        assert 'PASS:' in smoke.stdout
        assert not list(runtime.glob('session-*')), 'Token directory leaked after smoke test'
        for path in state.iterdir():
            assert stat.S_IMODE(path.stat().st_mode) == 0o600, path
        if args.gui:
            for failure in (False, True):
                app = subprocess.Popen([launcher, '--demo'], env=env, cwd=work, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
                children = []
                try:
                    for _ in range(150):
                        assert app.poll() is None, app.communicate()
                        children_file = Path(f'/proc/{app.pid}/task/{app.pid}/children')
                        children = children_file.read_text().split()
                        tokens = list(runtime.glob('session-*/exact/dev.axioma.lazyconteiner/data/session-token'))
                        if len(children) == 2 and tokens:
                            break
                        time.sleep(0.1)
                    else:
                        raise AssertionError('Packaged native window did not start')
                    token_file = tokens[0]
                    assert stat.S_IMODE(token_file.stat().st_mode) == 0o600
                    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
                    request = urllib.request.Request('http://127.0.0.1:17878/api/config', headers={'X-LazyContainer-Token': token_file.read_text()})
                    with opener.open(request, timeout=2) as response:
                        json.load(response)
                    duplicate = subprocess.run([launcher, '--demo'], env=env, cwd=work, capture_output=True, text=True, timeout=5)
                    assert duplicate.returncode == 1 and 'already running' in duplicate.stderr, duplicate.stderr
                    if failure:
                        backend_pid = next(pid for pid in children if Path(f'/proc/{pid}/exe').resolve().name == 'backend')
                        os.kill(int(backend_pid), signal.SIGTERM)
                        _, error = app.communicate(timeout=10)
                        assert app.returncode == 1 and 'Backend stopped unexpectedly' in error, error
                    else:
                        time.sleep(2)
                        app.terminate()
                        app.communicate(timeout=10)
                        assert app.returncode == 130
                finally:
                    if app.poll() is None:
                        app.terminate()
                        app.communicate(timeout=15)
                assert not list(runtime.glob('session-*')), 'Token directory leaked'
                assert all(not Path(f'/proc/{pid}').exists() for pid in children), 'Child process leaked'
            print('PASS: duplicate refusal, token permissions, signal and backend-failure cleanup')
        assert not list((work / 'config').rglob('config.json')), 'Demo changed configuration'
        print('PASS: package metadata, ownership, checksums, APT simulation, relocation and occupied-port refusal')


if __name__ == '__main__':
    main()
