#!/usr/bin/env python3
"""Exercise live API operations only on uniquely named disposable Docker resources."""
import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
URL = 'http://127.0.0.1:17880'


def docker(*args, check=True):
    result = subprocess.run(['docker', *args], check=check, text=True, capture_output=True, timeout=120)
    if check and result.returncode:
        raise RuntimeError(result.stderr)
    return result.stdout.strip()


def main():
    if os.environ.get('DOCKER_HOST', '').startswith(('tcp:', 'http:', 'https:', 'ssh:')):
        raise RuntimeError('Live checks require a local Unix Docker socket.')
    docker('image', 'inspect', 'alpine:3.20')  # Never download an image implicitly.
    runtime = ROOT / '.runtime'
    runtime.mkdir(exist_ok=True)
    binary = runtime / 'backend-live-check'
    subprocess.run(['go', 'build', '-o', str(binary), '.'], cwd=ROOT / 'backend', check=True)
    stem = 'lazyconteiner-check-' + secrets.token_hex(5)
    label = 'dev.axioma.lazyconteiner.check=' + stem
    token = secrets.token_hex(32)
    checks = []
    container_id = network_id = volume_name = image_id = None
    backend = None

    def api(path, data=None):
        request = urllib.request.Request(URL + path, data=None if data is None else json.dumps(data).encode(),
            headers={'X-LazyContainer-Token': token, 'Content-Type': 'application/json'})
        try:
            with urllib.request.urlopen(request, timeout=35) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            detail = json.load(error)
            raise RuntimeError(f'{path}: HTTP {error.code}: {detail.get("error", "request failed")}') from None

    def action(kind, target, op, args=None):
        preview = api('/api/action/preview', {'kind': kind, 'ids': [target], 'op': op, 'args': args or []})
        assert [item['id'] for item in preview['targets']] == [target], 'Preview included unexpected resources'
        result = api('/api/action', {'confirmation': preview['confirmation']})
        if result.get('jobId'):
            for _ in range(300):
                result = api('/api/jobs/' + result.get('id', result.get('jobId')))
                if result['status'] != 'running':
                    break
                time.sleep(0.1)
            assert result['status'] == 'succeeded', result.get('error') or 'Job timeout'
        assert result.get('count', 0) > 0, f'{kind}/{op} did not complete a target'
        checks.append(f'{kind}/{op}')
        return result

    def detail(kind, target, tab, reveal=False):
        value = api(f'/api/detail/{kind}/{urllib.parse.quote(target, safe="")}?tab={tab}&reveal={int(reveal)}')
        assert isinstance(value['text'], str) and value['text'], f'Empty {kind}/{tab}'
        checks.append(f'{kind}/{tab}')
        return value['text']

    with tempfile.TemporaryDirectory(prefix=stem + '-', dir=runtime) as directory:
        work = Path(directory)
        compose = work / 'compose.yaml'
        compose.write_text(json.dumps({'services': {'worker': {
            'image': 'alpine:3.20', 'command': ['sh', '-c', 'echo compose-check; sleep 600'],
            'labels': {'dev.axioma.lazyconteiner.check': stem},
        }}}))
        config = work / 'config.json'
        config.write_text(json.dumps({'projects': [{'id': stem, 'path': directory, 'files': [str(compose)]}],
            'commands': [{'id': 'smoke-echo', 'label': 'Smoke echo', 'scope': 'commands',
                          'argv': ['/bin/echo', 'isolated-command-check'], 'cwd': directory, 'terminal': False}],
            'terminal': ['x-terminal-emulator', '-e']}))
        try:
            network_id = docker('network', 'create', '--label', label, stem)
            volume_name = docker('volume', 'create', '--label', label, stem)
            container_id = docker('create', '--name', stem, '--label', label, '--network', network_id,
                '--mount', f'type=volume,source={volume_name},target=/data', '-e', 'CHECK_SECRET=isolated-fixture',
                'alpine:3.20', 'sh', '-c', 'echo lazyconteiner-check; sleep 600')
            image_id = docker('commit', '--change', f'LABEL dev.axioma.lazyconteiner.check={stem}', container_id, stem + ':check')
            with (work / 'backend.log').open('w') as log:
                backend = subprocess.Popen([str(binary), '--listen', '127.0.0.1:17880'], cwd=ROOT,
                    env={**os.environ, 'LAZYCONTAINER_TOKEN': token, 'LAZYCONTAINER_CONFIG': str(config)}, stdout=log, stderr=log)
                for _ in range(100):
                    if backend.poll() is not None:
                        raise RuntimeError('Isolated test backend failed to start; check port 17880.')
                    try:
                        if api('/api/health')['mode'] == 'live':
                            break
                    except (OSError, RuntimeError):
                        pass
                    time.sleep(0.1)
                else:
                    raise RuntimeError('Isolated test backend did not become ready')
                for kind, target in [('containers', container_id), ('images', image_id), ('volumes', volume_name), ('networks', network_id)]:
                    assert target in {item['id'] for item in api('/api/inventory?kind=' + kind)['items']}
                    checks.append(kind + '/inventory')
                    detail(kind, target, 'inspect')
                action('containers', container_id, 'start')
                detail('containers', container_id, 'logs')
                detail('containers', container_id, 'stats')
                detail('containers', container_id, 'top')
                assert 'isolated-fixture' not in detail('containers', container_id, 'env')
                assert 'isolated-fixture' in detail('containers', container_id, 'env', True)
                action('containers', container_id, 'pause')
                action('containers', container_id, 'unpause')
                action('containers', container_id, 'restart')
                action('containers', container_id, 'stop')
                action('containers', container_id, 'start')
                action('containers', container_id, 'kill')
                action('commands', 'smoke-echo', 'run')
                detail('images', image_id, 'history')
                action('projects', stem, 'up')
                services = [item for item in api('/api/inventory?kind=services')['items'] if item['project'] == stem]
                assert len(services) == 1
                service = services[0]['id']
                detail('services', service, 'logs')
                for op in ['pause', 'unpause', 'restart', 'stop', 'start', 'recreate']:
                    action('services', service, op)
                detail('projects', stem, 'config', reveal=True)
                action('projects', stem, 'down')
                action('containers', container_id, 'remove')
                container_id = None
                action('images', image_id, 'remove')
                image_id = None
                action('volumes', volume_name, 'remove')
                volume_name = None
                action('networks', network_id, 'remove')
                network_id = None
                print(json.dumps({'passed': len(checks), 'checks': checks, 'global_prune': 'not run against user daemon'}, indent=2))
        finally:
            if backend and backend.poll() is None:
                backend.terminate()
                backend.wait(timeout=10)
            docker('compose', '--project-name', stem, '--project-directory', directory, '-f', str(compose), 'down', '--volumes', check=False)
            if container_id:
                docker('rm', '-f', container_id, check=False)
            if image_id:
                docker('image', 'rm', image_id, check=False)
            if volume_name:
                docker('volume', 'rm', volume_name, check=False)
            if network_id:
                docker('network', 'rm', network_id, check=False)


if __name__ == '__main__':
    main()
