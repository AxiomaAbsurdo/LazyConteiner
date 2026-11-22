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

test('mutation requires a session token and reports server refusal', async () => {
  const absent = { fs: { readFile: async () => { throw new Error('missing'); } } };
  expect((await call('containerOp', ['live', 'restart'], absent)).ok).toBe(false);
  let requests = 0;
  globalThis.fetch = (async (_url: unknown, init: RequestInit) => {
    requests++;
    expect((init.headers as Record<string, string>)['X-LazyContainer-Token']).toBe('test-only');
    return new Response('{}', { status: 409 });
  }) as typeof fetch;
  const storage = { fs: { readFile: async () => new TextEncoder().encode('test-only\n') } };
  const result = await call('containerOp', ['live', 'restart'], storage);
  expect(result.error).toContain('HTTP 409');
  expect(requests).toBe(1);
});

test('cleanup preview never sends a mutation', async () => {
  let requests = 0;
  globalThis.fetch = (async () => { requests++; return Response.json(dashboard); }) as typeof fetch;
  await call('dashboard', ['all', '', false]);
  const result = await call('previewCleanup', []);
  expect(result.error).toContain('Nothing was removed.');
  expect(requests).toBe(1);
});
