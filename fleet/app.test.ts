import { afterEach, expect, test } from 'bun:test';
import { answer } from './app';

const originalFetch = globalThis.fetch;
const dashboard = {
  warnings: [], mode: 'demo', error: '', updatedAt: '', env: { id: 'local', label: 'local', connected: true, detail: 'fixture' }, envs: [],
  totals: { containers: 2, running: 1, failed: 1, cleanExit: 0, images: 0, imagesSize: '0 B', imagesUnused: 0, volumes: 0, volumesUnused: 0, networks: 0, networksDetail: '' },
  stacks: [{ id: 'project', name: 'Project', isCompose: true, running: 1, total: 2, status: 'Partial', containers: [
    { id: 'live', name: 'web', stack: 'project', state: 'running', exitCode: -1, ports: '', cpu: '—', selected: false },
    { id: 'failed', name: 'worker', stack: 'project', state: 'exited', exitCode: 137, ports: '', cpu: '—', selected: false },
  ] }], alerts: [], cleanup: [], selectedCount: 0, host: [], serviceActive: false, consumers: [], events: [], unread: 0, attention: 1,
};
// These tests exercise the data boundary without an Exact host or Docker daemon.
const call = (source: string, args: unknown[], storage: unknown = {}) => (answer as Function)(source, args, {}, storage, {});
afterEach(async () => { globalThis.fetch = originalFetch; await call('clearSelected', []); });

test('filters and selection preserve global totals', async () => {
  globalThis.fetch = (async () => Response.json(dashboard)) as typeof fetch;
  await call('setSelected', ['failed', true]);
  const result = await call('dashboard', ['running', '', false]);
  expect(result.stacks[0].containers.map((c: { id: string }) => c.id)).toEqual(['live']);
  expect(result.selectedCount).toBe(1);
  expect(result.totals.containers).toBe(2);
  expect((await call('dashboard', ['all', 'WORKER', false])).stacks[0].containers[0].selected).toBe(true);
});

test('HTTP failure never becomes fabricated dashboard data', async () => {
  globalThis.fetch = (async () => new Response('{}', { status: 503 })) as typeof fetch;
  await expect(call('dashboard', ['all', '', false])).rejects.toThrow('Docker backend unavailable');
});

test('invalid traversal input is rejected', async () => {
  globalThis.fetch = (async () => Response.json({ stacks: [{}], alerts: [], cleanup: [] })) as typeof fetch;
  await expect(call('dashboard', ['all', '', false])).rejects.toThrow('Invalid dashboard');
});

test('quick operation requires a token and shows backend preview targets', async () => {
  const absent = { fs: { readFile: async () => { throw new Error('missing'); } } };
  expect((await call('prepareContainer', ['live', 'restart'], absent)).confirmation).toBe('');
  globalThis.fetch = (async (_url: unknown, init: RequestInit) => {
    expect((init.headers as Record<string, string>)['X-LazyContainer-Token']).toBe('test-only');
    return Response.json({ confirmation: 'one-use', targets: [{ id: 'live', name: 'web' }] });
  }) as typeof fetch;
  const storage = { fs: { readFile: async () => new TextEncoder().encode('test-only') } };
  const result = await call('prepareContainer', ['live', 'restart'], storage);
  expect(result.names).toBe('web · live');
  expect(result.confirmation).toBe('one-use');
});

test('cleanup preview never executes a Docker operation', async () => {
  const urls: string[] = [];
  globalThis.fetch = (async (url: unknown) => {
    urls.push(String(url));
    return Response.json(String(url).endsWith('/api/dashboard') ? { ...dashboard, cleanup: [{ id: 'containers', label: 'Stopped', size: '', checked: true, dangerous: false }] } : { targets: [{ name: 'worker' }] });
  }) as typeof fetch;
  await call('dashboard', ['all', '', false]);
  const storage = { fs: { readFile: async () => new TextEncoder().encode('test-only') } };
  const result = await call('previewCleanup', [], storage);
  expect(result.error).toContain('Nothing was removed.');
  expect(urls.filter(url => url.endsWith('/api/action'))).toHaveLength(0);
  expect(urls.filter(url => url.endsWith('/api/action/preview'))).toHaveLength(1);
});

test('dashboard filters and checkbox changes reuse the current snapshot', async () => {
  let requests = 0;
  globalThis.fetch = (async () => { requests++; return Response.json(dashboard); }) as typeof fetch;
  await call('dashboard', ['all', '', false, 50]);
  await call('dashboard', ['running', 'web', false, 50]);
  await call('setSelected', ['live', true]);
  await call('dashboard', ['all', '', false, 50]);
  expect(requests).toBe(1);
  await call('dashboard', ['all', '', false, 51]);
  expect(requests).toBe(2);
});

test('dashboard quickJob retries after a transient read failure', async () => {
  let reads = 0;
  const storage = { fs: { readFile: async () => new TextEncoder().encode('test-only') } };
  globalThis.fetch = (async (url: unknown) => {
    expect(String(url)).toEndWith('/api/jobs/dashboard-retry');
    reads++;
    if (reads === 1) return Response.json({ error: 'temporarily unavailable' }, { status: 503 });
    return Response.json({ id: 'dashboard-retry', status: 'succeeded', text: 'Container restarted', error: '' });
  }) as typeof fetch;
  expect((await call('quickJob', ['dashboard-retry', 0], storage)).status).toBe('unknown');
  const done = await call('quickJob', ['dashboard-retry', 1], storage);
  expect(done.status).toBe('succeeded');
  expect(done.text).toBe('Container restarted');
  expect(reads).toBe(2);
});

test('dashboard strips backend bookkeeping from nested Contract objects', async () => {
  const body = { ...dashboard, internal: true, env: { ...dashboard.env, internal: true }, totals: { ...dashboard.totals, internal: true }, stacks: dashboard.stacks.map(stack => ({ ...stack, internal: true, containers: stack.containers.map(container => ({ ...container, version: 'private-proof' })) })) };
  globalThis.fetch = (async () => Response.json(body)) as typeof fetch;
  const result = await call('dashboard', ['all', '', false]);
  expect(result).not.toHaveProperty('internal');
  expect(result.env).not.toHaveProperty('internal');
  expect(result.totals).not.toHaveProperty('internal');
  expect(result.stacks[0]).not.toHaveProperty('internal');
  expect(result.stacks[0].containers[0]).not.toHaveProperty('version');
});
