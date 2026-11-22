import { afterEach, expect, test } from 'bun:test';
import { workbenchSources, invalidateInventory } from './workbench';

const originalFetch = globalThis.fetch;
const storage = { fs: { readFile: async () => new TextEncoder().encode('fixture-token') } };
const call = (source: keyof typeof workbenchSources, args: unknown[]) => (workbenchSources[source] as Function)(args, {}, storage);
const inventory = { kind: 'containers', items: [{ id: 'sample', name: 'Sample', state: 'running', detail: '', project: '', service: '' }], actions: [{ id: 'stop', label: 'Stop', destructive: false, input: '' }], views: [{ id: 'logs', label: 'Logs' }], mode: 'demo', error: '' };
afterEach(() => { globalThis.fetch = originalFetch; invalidateInventory(); });

test('inventory cache avoids duplicate requests until refresh', async () => {
  let reads = 0;
  globalThis.fetch = (async (_url: unknown, init: RequestInit) => {
    reads++;
    expect((init.headers as Record<string, string>)['X-LazyContainer-Token']).toBe('fixture-token');
    return Response.json(inventory);
  }) as typeof fetch;
  expect((await call('inventory', ['containers', 0])).items.length).toBe(1);
  await call('inventory', ['containers', 0]);
  expect(reads).toBe(1);
  await call('inventory', ['containers', 1]);
  expect(reads).toBe(2);
});

test('preview rejects malformed arguments before any HTTP request', async () => {
  let requests = 0;
  globalThis.fetch = (async () => { requests++; return Response.json({}); }) as typeof fetch;
  const reply = await call('previewAction', ['containers', ['sample'], 'exec', '{"shell":"bad"}']);
  expect(reply.confirmation).toBe('');
  expect(reply.error).toContain('JSON array');
  expect(requests).toBe(0);
});

test('execution only sends the server confirmation and retains partial failure output', async () => {
  globalThis.fetch = (async (url: unknown, init: RequestInit) => {
    expect(String(url)).toEndWith('/api/action');
    expect(JSON.parse(init.body as string)).toEqual({ confirmation: 'one-use-confirmation' });
    return Response.json({ count: 1, text: 'First target stopped', error: 'Second target failed' }, { status: 502 });
  }) as typeof fetch;
  const reply = await call('executeAction', ['one-use-confirmation']);
  expect(reply.count).toBe(1);
  expect(reply.text).toBe('First target stopped');
  expect(reply.error).toBe('Second target failed');
});

test('detail identity prevents stale content when selecting another target', async () => {
  globalThis.fetch = (async () => Response.json({ title: 'Sample', text: 'masked', mode: 'demo' })) as typeof fetch;
  const result = await call('detail', ['containers', 'sample', 'inspect', false, 0]);
  expect(result.owner).toBe('containers/sample/inspect/false');
  expect(result.text).toBe('masked');
});

test('invalid configuration never reaches backend', async () => {
  let writes = 0;
  globalThis.fetch = (async () => { writes++; return Response.json({}); }) as typeof fetch;
  expect((await call('saveConfig', ['[]'])).ok).toBe(false);
  expect(writes).toBe(0);
});

test('transient job read failure retains output and continues the same job', async () => {
  let reads = 0;
  globalThis.fetch = (async (url: unknown) => {
    expect(String(url)).toEndWith('/api/jobs/retry-job');
    reads++;
    if (reads === 2) throw new Error('temporary network failure');
    return Response.json({ id: 'retry-job', status: reads === 1 ? 'running' : 'succeeded', text: reads === 1 ? 'Starting service' : 'Service ready', error: '' });
  }) as typeof fetch;
  expect((await call('job', ['retry-job', 0])).status).toBe('running');
  const unavailable = await call('job', ['retry-job', 1]);
  expect(unavailable.status).toBe('unknown');
  expect(unavailable.text).toBe('Starting service');
  expect(unavailable.error).toContain('Retrying the same job');
  expect((await call('job', ['retry-job', 2])).status).toBe('succeeded');
  expect(reads).toBe(3);
});

test('all workbench responses omit backend fields outside closed Exact shapes', async () => {
  const metric = { label: 'CPU', value: '12%', percent: 12, history: [12], internal: true };
  globalThis.fetch = (async (url: unknown) => {
    const path = String(url);
    if (path.includes('/inventory?')) return Response.json({ ...inventory, serverVersion: 2, items: inventory.items.map(item => ({ ...item, version: 'private-proof' })), actions: inventory.actions.map(action => ({ ...action, internal: true })), views: inventory.views.map(view => ({ ...view, internal: true })) });
    if (path.includes('/detail/')) return Response.json({ title: 'Sample', text: 'stats', mode: 'demo', metrics: [metric], points: [12] });
    if (path.endsWith('/action/preview')) return Response.json({ confirmation: 'proof', summary: 'Start Sample', expiresAt: 'later', targets: [{ ...inventory.items[0], version: 'private-proof' }], internal: true });
    if (path.endsWith('/action')) return Response.json({ count: 1, mode: 'demo', text: 'Started', jobId: '', internal: true });
    if (path.includes('/jobs/')) return Response.json({ id: 'closed-shape', status: 'succeeded', text: 'Done', error: '', count: 1 });
    return Response.json({ text: '{}', path: '/config', internal: true });
  }) as typeof fetch;
  expect(await call('inventory', ['containers', 100])).toEqual(inventory);
  expect(await call('detail', ['containers', 'sample', 'stats', false, 100])).toEqual({ title: 'Sample', text: 'stats', mode: 'demo', owner: 'containers/sample/stats/false', error: '', metrics: [{ label: 'CPU', value: '12%', percent: 12, history: [12] }] });
  expect(await call('previewAction', ['containers', ['sample'], 'start', '[]'])).toEqual({ confirmation: 'proof', summary: 'Start Sample', expiresAt: 'later', targets: [{ id: 'sample', name: 'Sample' }], error: '' });
  expect(await call('executeAction', ['proof'])).toEqual({ count: 1, mode: 'demo', text: 'Started', jobId: '', error: '' });
  expect(await call('job', ['closed-shape', 0])).toEqual({ id: 'closed-shape', status: 'succeeded', text: 'Done', error: '' });
  expect(await call('loadConfig', [])).toEqual({ text: '{}', path: '/config', error: '' });
});

test('inventory response started before invalidation cannot repopulate cache', async () => {
  let release: (response: Response) => void = () => {};
  let began: () => void = () => {};
  const started = new Promise<void>(resolve => { began = resolve; });
  let requests = 0;
  globalThis.fetch = (async () => {
    requests++;
    if (requests === 1) {
      began();
      return new Promise<Response>(resolve => { release = resolve; });
    }
    return Response.json({ ...inventory, items: [] });
  }) as typeof fetch;
  const oldRead = call('inventory', ['containers', 0]);
  await started;
  invalidateInventory();
  release(Response.json(inventory));
  await oldRead;
  const fresh = await call('inventory', ['containers', 0]);
  expect(fresh.items).toHaveLength(0);
  expect(requests).toBe(2);
});
