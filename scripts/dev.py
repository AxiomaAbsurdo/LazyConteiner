#!/usr/bin/env python3
"""Start the local Docker bridge and Exact UI with a private session token."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import secrets
import signal
import subprocess
import sys
import time
import urllib.request

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--demo', action='store_true', help='use sample data without contacting Docker')
    parser.add_argument('--test', action='store_true', help='run native UI tests against an isolated demo backend')
    args = parser.parse_args()
    if args.test:
        args.demo = True
    runtime = ROOT / '.runtime'
    runtime.mkdir(exist_ok=True)
    lock = (runtime / 'dev.lock').open('w')
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        parser.exit(1, 'LazyConteiner is already running from this checkout.\n')
    subprocess.run(['go', 'build', '-o', str(runtime / 'backend'), '.'], cwd=ROOT / 'backend', check=True)
    subprocess.run([str(ROOT / 'scripts/exact-run.sh'), 'build'], cwd=ROOT, check=True)
    token = secrets.token_hex(32)
    data_home = runtime / 'data'
    token_file = data_home / 'exact/dev.axioma.lazyconteiner/data/session-token'
    token_file.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd = os.open(token_file, os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'w') as stream:
        os.fchmod(stream.fileno(), 0o600)
        stream.write(token)
    backend = ui = None

    def interrupted(signum, frame):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, interrupted)
    try:
        backend = subprocess.Popen([str(runtime / 'backend'), '--listen', '127.0.0.1:17878'] + (['--demo'] if args.demo else []),
                                   env={**os.environ, 'LAZYCONTAINER_TOKEN': token}, cwd=ROOT)
        for _ in range(100):
            if backend.poll() is not None:
                raise RuntimeError('Backend failed to start; check whether port 17878 is already in use.')
            try:
                with urllib.request.urlopen('http://127.0.0.1:17878/api/health', timeout=0.2) as response:
                    health = json.load(response)
                if health.get('core') == 'lazydocker':
                    break
            except (OSError, ValueError):
                pass
            time.sleep(0.1)
        else:
            raise RuntimeError('Backend did not become ready.')
        # Give a failed bind time to surface before accepting a different listener.
        time.sleep(0.1)
        if backend.poll() is not None:
            raise RuntimeError('Backend exited during startup.')
        print('LazyConteiner: ' + ('DEMO (no Docker access)' if args.demo else 'LIVE Docker') + ('. Running native UI tests.' if args.test else '. Close the window or press Ctrl+C to stop.'), flush=True)
        ui = subprocess.Popen([str(ROOT / 'scripts/exact-run.sh'), 'test' if args.test else 'run'], cwd=ROOT,
                              env={**os.environ, 'XDG_DATA_HOME': str(data_home), 'EXACT_SKIP_BUILD': '1'}, start_new_session=True)
        while ui.poll() is None:
            if backend.poll() is not None:
                raise RuntimeError('Backend stopped unexpectedly.')
            time.sleep(0.2)
        return ui.returncode
    finally:
        if ui and ui.poll() is None:
            os.killpg(ui.pid, signal.SIGTERM)
            try:
                ui.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(ui.pid, signal.SIGKILL)
                ui.wait()
        if backend and backend.poll() is None:
            backend.terminate()
            try:
                backend.wait(timeout=5)
            except subprocess.TimeoutExpired:
                backend.kill()
                backend.wait()
        token_file.unlink(missing_ok=True)


if __name__ == '__main__':
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        sys.exit(130)
    except (OSError, RuntimeError, subprocess.CalledProcessError) as error:
        print(f'LazyConteiner: {error}', file=sys.stderr)
        sys.exit(1)
