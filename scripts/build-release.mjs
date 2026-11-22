// Use the pinned Exact build environment, with its production release profile.
import { spawnSync } from 'node:child_process';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const exact = resolve(process.env.EXACT || resolve(root, '.runtime/exact'));
const { resolveApp, linuxBuild, bakeWatchEnv, developmentBuildEnv } = await import(resolve(exact, 'scripts/app.mjs'));
process.env.EXACT_UPDATE_TRUST = 'production';
const app = resolveApp(resolve(root, 'fleet'));
const [command, ...args] = linuxBuild(app);
args[args.indexOf('--profile') + 1] = 'release';
args.push('--locked');
const result = spawnSync(command, args, {
  cwd: app.workspace ?? app.dir,
  env: { ...developmentBuildEnv(), ...bakeWatchEnv(app), CARGO_TARGET_DIR: app.target },
  stdio: 'inherit',
});
process.exitCode = result.status ?? 1;
