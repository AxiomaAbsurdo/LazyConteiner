#!/usr/bin/env python3
"""Collect license texts from the pinned native and Go dependency graphs."""
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent


def output(command, cwd):
    return subprocess.check_output(command, cwd=cwd, text=True)


def documents(directory):
    return sorted({p for pattern in ('LICENSE*', 'LICENCE*', 'COPYING*', 'NOTICE*', 'UNLICENSE*')
                   for p in directory.glob(pattern) if p.is_file()})


def main():
    doc = Path(sys.argv[1])
    licenses = doc / 'licenses'
    licenses.mkdir(exist_ok=True)
    records = []
    missing = []
    fallback_root = ROOT / 'packaging/licenses'
    fallbacks = json.loads((fallback_root / 'sources.json').read_text())

    def collect(name, version, directory, declared='See included license text', search_parents=False):
        directory = Path(directory)
        files = documents(directory)
        if search_parents:
            for parent in directory.parents:
                if files or parent == Path.home() or parent == ROOT.parent:
                    break
                files = documents(parent)
        key = re.sub(r'[^A-Za-z0-9._-]', '_', name + '-' + version)
        upstream = []
        if not files:
            upstream = fallbacks.get(key, [])
            for entry in upstream:
                source = fallback_root / entry['file']
                if hashlib.sha256(source.read_bytes()).hexdigest() != entry['sha256']:
                    raise RuntimeError(f'License checksum mismatch: {source}')
                files.append(source)
        paths = []
        for source in files:
            target = licenses / (key + '-' + source.name)
            shutil.copyfile(source, target)
            paths.append(str(target.relative_to(doc)))
        records.append({'name': name, 'version': version, 'license': declared, 'notices': paths,
                        **({'upstream': upstream} if upstream else {})})
        if not files:
            missing.append(name)

    cargo = json.loads(output(['cargo', 'metadata', '--format-version=1', '--locked', '--offline', '--filter-platform=x86_64-unknown-linux-gnu'], ROOT / 'fleet/.exact/shells'))
    graph = {node['id']: node for node in cargo['resolve']['nodes']}
    pending = [package['id'] for package in cargo['packages'] if package['name'] == 'lazyconteiner-linux']
    included = set()
    while pending:
        package_id = pending.pop()
        if package_id in included:
            continue
        included.add(package_id)
        pending.extend(edge['pkg'] for edge in graph[package_id]['deps']
                       if any(kind['kind'] != 'dev' for kind in edge['dep_kinds']))
    for package in cargo['packages']:
        if package['id'] not in included:
            continue
        if package['name'] == 'lazyconteiner-linux':
            continue
        collect(package['name'], package['version'], Path(package['manifest_path']).parent,
                package.get('license') or 'See included license text', search_parents=True)
    # Go emits a stream of JSON objects rather than an array.
    data = output(['go', 'list', '-deps', '-json', '.'], ROOT / 'backend')
    decoder = json.JSONDecoder()
    modules = {}
    while data.strip():
        package, end = decoder.raw_decode(data.lstrip())
        data = data.lstrip()[end:]
        module = package.get('Module', {})
        if module.get('Dir') and not module.get('Main'):
            modules[module['Path']] = module
    for name, module in sorted(modules.items()):
        collect(name, module['Version'], module['Dir'])
    # Hermes/ICU are prebuilt static libraries; their bundle ships its own notices.
    native = set()
    for build in (ROOT / 'fleet/target/release/build').glob('hermes-lean-sys-*/output'):
        for line in build.read_text().splitlines():
            if 'rustc-link-search=native=' in line:
                directory = Path(line.split('rustc-link-search=native=', 1)[1])
                native.update(documents(directory))
                native.update(documents(directory.parent))
    for source in sorted(native):
        target = licenses / ('native-' + hashlib.sha256(source.read_bytes()).hexdigest()[:12] + '-' + source.name)
        shutil.copyfile(source, target)
    if not any('hermes' in p.name.lower() for p in native):
        raise RuntimeError('The release Hermes bundle license was not found; refusing to omit it.')
    (doc / 'dependency-notices.json').write_text(json.dumps(records, indent=2) + '\n')
    (doc / 'THIRD-PARTY.txt').write_text(
        'Pinned Cargo graph (including build dependencies) and compiled Go module notices.\n'
        'See dependency-notices.json for versions, declared licenses and local notice paths.\n'
        'Prebuilt Hermes/ICU bundle notices are in licenses/native-*.\n'
        'drm-fourcc upstream provides MIT metadata but no standalone license file; its original\n'
        'Cargo.toml and README preserve the declaration and contributor attribution.\n'
        + ('Missing source license files: ' + ', '.join(missing) + '\n' if missing else 'All enumerated packages have license texts or upstream declarations.\n'))
    print(f'Collected notices for {len(records)} dependencies and {len(native)} native license files.')
    if missing:
        raise RuntimeError('Missing license files: ' + ', '.join(missing))


if __name__ == '__main__':
    main()
