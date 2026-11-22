// Seed only this run's isolated Exact stores; never bypass backend authentication.
import { existsSync, mkdirSync, readdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const exact = process.env.EXACT || resolve(root, '.runtime/exact');
const token = process.env.LAZYCONTAINER_TEST_TOKEN;
if (!token) throw new Error('Run native tests through make test so a private demo backend is available.');
const cache = resolve(root, '.runtime/test-cache');
const base = resolve(cache, 'exact/dev.axioma.lazyconteiner/agent');
const seeded = new Set();
const { runTests } = await import(resolve(exact, 'scripts/agent-test.mjs'));
const result = await runTests({
  host: 'linux', app: resolve(root, 'fleet'), file: resolve(root, 'fleet/app.test.contract'),
  only: process.env.NATIVE_TEST_ONLY,
  size: [1440, 900], storage: 'lazytests', env: { XDG_CACHE_HOME: cache },
  atLine({ index }) {
    if (seeded.has(index)) return;
    const stores = existsSync(base) ? readdirSync(base).filter(name =>
      name.startsWith(`lazytests.r${process.pid}-`) && name.endsWith(`.t${index}`)) : [];
    if (stores.length !== 1) throw new Error(`Expected one isolated native store for test ${index}, found ${stores.length}.`);
    const directory = resolve(base, stores[0], 'data');
    mkdirSync(directory, { recursive: true, mode: 0o700 });
    writeFileSync(resolve(directory, 'session-token'), token, { mode: 0o600 });
    seeded.add(index);
  },
  atResult(result) {
    console.log(`${result.failures.length ? 'FAIL' : 'PASS'} ${result.name}`);
    for (const failure of result.failures) console.error(failure);
  },
  async atEnd(session, { index, failures }) {
    if (process.env.NATIVE_CAPTURE) {
      const output = resolve(root, '.runtime/native-screenshots');
      mkdirSync(output, { recursive: true });
      await session.screenshot(resolve(output, `${index}.png`));
      writeFileSync(resolve(output, `${index}.json`), JSON.stringify(await session.tree(), null, 2));
    }
    if (!failures.length) return;
    const output = resolve(root, '.runtime/native-failures');
    mkdirSync(output, { recursive: true });
    writeFileSync(resolve(output, `${index}.json`), JSON.stringify(await session.tree(), null, 2));
    writeFileSync(resolve(output, `${index}.logs.json`), JSON.stringify(await session.logs(), null, 2));
  },
});
console.log(`${result.passed} passed, ${result.failed} failed, ${result.skipped} skipped`);
process.exitCode = result.failed || result.skipped ? 1 : 0;
