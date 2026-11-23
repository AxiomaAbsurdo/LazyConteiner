#!/usr/bin/env python3
"""Verify the installed Arch bundle using demo data in an isolated container."""
import json
import os
from pathlib import Path
import signal
import stat
import subprocess
import tempfile
import time
import urllib.request


def wait_for(check, description, timeout=15):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        result = check()
        if result:
            return result
        time.sleep(0.1)
    raise AssertionError(description)


def main():
    lib = Path('/usr/lib/lazyconteiner')
    missing = subprocess.run(['ldd', str(lib / 'ui')], capture_output=True, text=True, check=True)
    assert 'not found' not in missing.stdout, missing.stdout
    subprocess.run(['pacman', '-Qkk', 'lazyconteiner-bin'], check=True)
    assert os.access(lib / 'bin/x-terminal-emulator', os.X_OK)
    assert not Path('/usr/bin/x-terminal-emulator').exists(), 'Compatibility alias must stay private'
    with tempfile.TemporaryDirectory(prefix='lazy-arch-check-') as work:
        work = Path(work)
        env = {**os.environ, 'DISPLAY': ':99', 'DOCKER_HOST': 'unix:///nonexistent-test.sock'}
        for key in ('CONFIG', 'DATA', 'STATE', 'CACHE', 'RUNTIME'):
            directory = work / key.lower()
            directory.mkdir(mode=0o700)
            env['XDG_' + key + ('_DIR' if key == 'RUNTIME' else '_HOME')] = str(directory)
        subprocess.run(['lazyconteiner', '--smoke-test'], env=env, check=True, timeout=20)
        display = subprocess.Popen(['Xvfb', ':99', '-screen', '0', '1440x900x24', '-nolisten', 'tcp'], stdout=subprocess.DEVNULL)
        app = None
        children = []
        try:
            wait_for(lambda: Path('/tmp/.X11-unix/X99').exists(), 'Xvfb did not start')
            app = subprocess.Popen(['lazyconteiner', '--demo'], env=env)
            runtime = work / 'runtime/lazyconteiner'
            def ready():
                assert app.poll() is None, 'Launcher exited before rendering'
                path = work / 'state/lazyconteiner/ui.log'
                return path.exists() and 'desktop window presented' in path.read_text()
            wait_for(ready, 'Production UI did not present a native window', timeout=30)
            children = Path(f'/proc/{app.pid}/task/{app.pid}/children').read_text().split()
            assert len(children) == 2
            tokens = list(runtime.glob('session-*/exact/dev.axioma.lazyconteiner/data/session-token'))
            assert len(tokens) == 1
            assert stat.S_IMODE(tokens[0].stat().st_mode) == 0o600
            headers = {'X-LazyContainer-Token': tokens[0].read_text()}
            opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
            def api(path):
                with opener.open(urllib.request.Request('http://127.0.0.1:17878' + path, headers=headers), timeout=5) as response:
                    return json.load(response)
            api('/api/config')
            # Verify the backend actually inherited the private terminal adapter.
            backend_pid = next(pid for pid in children if Path(f'/proc/{pid}/exe').resolve() == lib / 'backend')
            entries = Path(f'/proc/{backend_pid}/environ').read_bytes().split(b'\0')
            backend_path = next(v.split(b'=', 1)[1].decode() for v in entries if v.startswith(b'PATH='))
            assert backend_path.split(os.pathsep)[0] == str(lib / 'bin')
            marker = work / 'terminal-ok'
            subprocess.run(['x-terminal-emulator', '-e', '/bin/sh', '-c', 'printf terminal-ok > "$1"', 'sh', str(marker)],
                           env={**env, 'PATH': backend_path}, check=True, timeout=10)
            assert marker.read_text() == 'terminal-ok'
            duplicate = subprocess.run(['lazyconteiner', '--demo'], env=env, capture_output=True, text=True, timeout=5)
            assert duplicate.returncode == 1 and 'already running' in duplicate.stderr
            tree = subprocess.check_output(['xwininfo', '-root', '-tree'], env=env, text=True)
            assert 'dev.axioma.lazyconteiner' in tree
            time.sleep(1)
            subprocess.run(['import', '-display', ':99', '-window', 'root', '/tmp/lazy-arch-demo.png'], check=True, timeout=10)
            print('PASS: actual Arch native GUI, authenticated demo API, private xterm adapter, duplicate refusal and token permissions')
        finally:
            if app and app.poll() is None:
                app.send_signal(signal.SIGTERM)
                app.wait(timeout=15)
            display.terminate()
            display.wait(timeout=5)
        assert all(not Path(f'/proc/{pid}').exists() for pid in children), 'Owned process leaked'
        assert not list((work / 'runtime/lazyconteiner').glob('session-*')), 'Session token leaked'
        subprocess.run(['lazyconteiner', '--smoke-test'], env=env, check=True, timeout=20)
        print('PASS: native shutdown cleanup and immediate restart')
    print('PASS: Arch bundle integration; no Docker socket or workloads accessed')


if __name__ == '__main__':
    main()
