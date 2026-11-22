import type { Result, Sources } from './app.contract.d.ts';
import { request, message } from './api';

type Inventory = Result<'inventory'>;
type Preview = Result<'previewAction'>;
type Execution = Result<'executeAction'>;
type Configuration = Result<'loadConfig'>;
const inventoryCache = new Map<string, { revision: number; generation: number; value: Inventory }>();
let generation = 0;
const observedJobs = new Map<string, Result<'job'>>();
export const invalidateInventory = () => { generation++; };

function inventoryBody(value: unknown, kind: string): Inventory {
  const data = value as Inventory;
  if (!data || data.kind !== kind || !Array.isArray(data.items) || !Array.isArray(data.actions) || !Array.isArray(data.views) ||
      !data.items.every(item => item && [item.id, item.name, item.state, item.detail, item.project, item.service].every(field => typeof field === 'string')) ||
      !data.actions.every(action => action && typeof action.id === 'string' && typeof action.label === 'string' && typeof action.destructive === 'boolean') ||
      !data.views.every(view => view && typeof view.id === 'string' && typeof view.label === 'string')) throw new Error('Invalid inventory response.');
  return {
    kind, mode: data.mode, error: data.error ?? '',
    items: data.items.map(({ id, name, state, detail, project, service }) => ({ id, name, state, detail, project, service })),
    actions: data.actions.map(({ id, label, destructive, input }) => ({ id, label, destructive, input: input ?? '' })),
    views: data.views.map(({ id, label }) => ({ id, label })),
  };
}

export const workbenchSources: Pick<Sources, 'inventory' | 'detail' | 'previewAction' | 'executeAction' | 'job' | 'loadConfig' | 'saveConfig'> = {
  inventory: async ([kind, revision], _store, storage) => {
    const cached = inventoryCache.get(kind);
    if (cached?.revision === revision && cached.generation === generation) return cached.value;
    const requestedGeneration = generation;
    try {
      const value = inventoryBody(await request(`/api/inventory?kind=${encodeURIComponent(kind)}`, storage), kind);
      if (requestedGeneration === generation) inventoryCache.set(kind, { revision, generation: requestedGeneration, value });
      return value;
    } catch (error) {
      return { kind, items: [], actions: [], views: [], mode: '', error: message(error) };
    }
  },
  detail: async ([kind, id, tab, reveal], _store, storage) => {
    const owner = `${kind}/${id}/${tab}/${reveal}`;
    try {
      const body = await request(`/api/detail/${encodeURIComponent(kind)}/${encodeURIComponent(id)}?tab=${encodeURIComponent(tab)}&reveal=${reveal ? '1' : '0'}`, storage) as Result<'detail'>;
      if (!body || typeof body.text !== 'string' || typeof body.title !== 'string') throw new Error('Invalid detail response.');
      const metrics = body.metrics ?? [];
      if (!Array.isArray(metrics) || !metrics.every(metric => metric && typeof metric.label === 'string' && typeof metric.value === 'string' && Number.isFinite(metric.percent) && Array.isArray(metric.history) && metric.history.every(point => Number.isFinite(point) && point >= 0 && point <= 100))) throw new Error('Invalid metrics response.');
      return { title: body.title, text: body.text, mode: body.mode, metrics: metrics.map(({ label, value, percent, history }) => ({ label, value, percent, history })), owner, error: '' };
    } catch (error) {
      return { title: '', text: '', metrics: [], mode: '', owner, error: message(error) };
    }
  },
  previewAction: async ([kind, ids, op, argumentsText], _store, storage) => {
    try {
      const args: unknown = JSON.parse(argumentsText);
      if (!Array.isArray(args) || !args.every(arg => typeof arg === 'string') || args.length > 128) throw new Error('Arguments must be a JSON array of at most 128 strings.');
      const value = await request('/api/action/preview', storage, 'POST', { kind, ids, op, args }) as Preview;
      if (!value || typeof value.confirmation !== 'string' || typeof value.summary !== 'string' || typeof value.expiresAt !== 'string' || !Array.isArray(value.targets) || !value.targets.every(target => target && typeof target.id === 'string' && typeof target.name === 'string')) throw new Error('Invalid confirmation response.');
      return { confirmation: value.confirmation, summary: value.summary, expiresAt: value.expiresAt, targets: value.targets.map(({ id, name }) => ({ id, name })), error: '' };
    } catch (error) {
      return { confirmation: '', summary: '', targets: [], expiresAt: '', error: message(error) };
    }
  },
  executeAction: async ([confirmation], _store, storage) => {
    try {
      if (!confirmation) throw new Error('Review the operation targets before executing.');
      const value = await request('/api/action', storage, 'POST', { confirmation }) as Execution;
      if (!value || typeof value.count !== 'number' || typeof value.text !== 'string') throw new Error('Invalid operation response. Refresh to check the actual resource state.');
      invalidateInventory();
      return { count: value.count, mode: value.mode, text: value.text, jobId: value.jobId ?? '', error: '' };
    } catch (error) {
      const details = (error as { details?: Partial<Execution> })?.details;
      return { count: typeof details?.count === 'number' ? details.count : 0, text: typeof details?.text === 'string' ? details.text : '', jobId: '', mode: '', error: message(error) };
    }
  },
  job: async ([id], _store, storage) => {
    try {
      const value = await request(`/api/jobs/${encodeURIComponent(id)}`, storage) as Result<'job'>;
      if (!value || typeof value.status !== 'string' || typeof value.text !== 'string') throw new Error('Invalid job response.');
      if (!['running', 'succeeded', 'failed'].includes(value.status)) throw new Error('Unrecognized job status.');
      if (value.status === 'succeeded' || value.status === 'failed') invalidateInventory();
      const observed = { id: value.id, status: value.status, text: value.text, error: value.error ?? '' };
      observedJobs.set(id, observed);
      return observed;
    } catch (error) {
      return { id, status: 'unknown', text: observedJobs.get(id)?.text ?? '', error: `Operation status unavailable; it may still be running. Retrying the same job. ${message(error)}` };
    }
  },
  loadConfig: async (_args, _store, storage) => {
    try {
      const value = await request('/api/config', storage) as Configuration;
      if (!value || typeof value.text !== 'string' || typeof value.path !== 'string') throw new Error('Invalid configuration response.');
      return { text: value.text, path: value.path, error: '' };
    } catch (error) {
      return { text: '', path: '', error: message(error) };
    }
  },
  saveConfig: async ([text], _store, storage) => {
    try {
      const value: unknown = JSON.parse(text);
      if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Configuration must be a JSON object.');
      await request('/api/config', storage, 'POST', { text });
      invalidateInventory();
      return { ok: true, error: '' };
    } catch (error) {
      return { ok: false, error: message(error) };
    }
  },
};
