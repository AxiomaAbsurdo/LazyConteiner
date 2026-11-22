import type { Answer, Result, Sources } from './app.contract.d.ts';

import { API, TOKEN_FILE, request, message } from './api';
import { workbenchSources } from './workbench';
export { API } from './api';
export const grants = `net.fetch ${API}\nfs.read ${TOKEN_FILE}`;

type Dashboard = Result<'dashboard'>;
type Reply = Result<'clearSelected'>;
const selected = new Set<string>();
const dismissed = new Set<string>();
const cleanup = new Map<string, boolean>();
let snapshot: Dashboard | undefined;
let snapshotRevision: number | undefined;
const ok = (): Reply => ({ ok: true, error: '' });

async function loadDashboard(revision?: number): Promise<Dashboard> {
  if (snapshot && revision !== undefined && snapshotRevision === revision) return snapshot;
  const response = await fetch(`${API}/api/dashboard`, { exactTimeout: 12000 });
  if (!response.ok) throw new Error('Docker backend unavailable. Check its log and socket permissions.');
  const body = await response.json() as Dashboard;
  // Exact validates the complete returned shape; check the fields we traverse first.
  if (!body || !Array.isArray(body.stacks) || !Array.isArray(body.alerts) || !Array.isArray(body.cleanup) ||
      !body.stacks.every(s => s && Array.isArray(s.containers) && s.containers.every(c => c && typeof c.id === 'string' && typeof c.name === 'string' && typeof c.state === 'string')) ||
      !body.alerts.every(a => a && typeof a.id === 'string') || !body.cleanup.every(c => c && typeof c.id === 'string')) {
    throw new Error('Invalid dashboard response.');
  }
  // Exact shapes are closed: omit backend bookkeeping at every object boundary.
  const dashboard: Dashboard = {
    mode: body.mode, error: body.error, updatedAt: body.updatedAt,
    selectedCount: 0, selectedNames: '', serviceActive: body.serviceActive, unread: body.unread, attention: body.attention, warnings: body.warnings,
    env: { id: body.env.id, label: body.env.label, connected: body.env.connected, detail: body.env.detail },
    envs: body.envs.map(({ id, label, connected, detail }) => ({ id, label, connected, detail })),
    totals: {
      containers: body.totals.containers, running: body.totals.running, failed: body.totals.failed, cleanExit: body.totals.cleanExit,
      images: body.totals.images, imagesSize: body.totals.imagesSize, imagesUnused: body.totals.imagesUnused,
      volumes: body.totals.volumes, volumesUnused: body.totals.volumesUnused, networks: body.totals.networks, networksDetail: body.totals.networksDetail,
    },
    alerts: body.alerts.map(({ id, title, hint, cta, ctaAction, tone, items }) => ({ id, title, hint, cta, ctaAction, tone, items: items.map(({ id, name, exitCode }) => ({ id, name, exitCode })) })),
    stacks: body.stacks.map(({ id, name, isCompose, running, total, status, containers }) => ({ id, name, isCompose, running, total, status,
      containers: containers.map(({ id, name, stack, state, exitCode, ports, cpu }) => ({ id, name, stack, state, exitCode, ports, cpu, selected: false })),
    })),
    host: body.host.map(({ label, valueText, percent, warn }) => ({ label, valueText, percent, warn })),
    consumers: body.consumers.map(({ id, name, cpu }) => ({ id, name, cpu })),
    events: body.events.map(({ id, name, what, tone, at }) => ({ id, name, what, tone, at })),
    cleanup: body.cleanup.map(({ id, label, size, checked, dangerous }) => ({ id, label, size, checked, dangerous })),
  };
  snapshot = dashboard;
  snapshotRevision = revision;
  const available = new Set(body.stacks.flatMap(s => s.containers.map(c => c.id)));
  for (const id of selected) if (!available.has(id)) selected.delete(id);
  return dashboard;
}

const sources: Sources = {
  ...workbenchSources,
  dashboard: async ([filter, query, showDismissed, revision]) => {
    const data = await loadDashboard(revision);
    const term = query.trim().toLowerCase();
    return {
      ...data,
      selectedCount: selected.size,
      selectedNames: data.stacks.flatMap(stack => stack.containers).filter(container => selected.has(container.id)).map(container => container.name).join(', '),
      alerts: data.alerts.filter(a => showDismissed || !dismissed.has(a.id)),
      cleanup: data.cleanup.filter(item => item.id !== 'cache').map(item => ({ ...item, checked: cleanup.get(item.id) ?? item.checked })),
      stacks: data.stacks.map(stack => ({
        ...stack,
        containers: stack.containers.filter(c =>
          (filter === 'all' || (filter === 'running' ? c.state === 'running' : c.state !== 'running')) &&
          (!term || c.name.toLowerCase().includes(term) || stack.name.toLowerCase().includes(term)),
        ).map(c => ({ ...c, selected: selected.has(c.id) })),
      })).filter(stack => stack.containers.length > 0),
    };
  },
  setSelected: ([id, on]) => { if (on) selected.add(id); else selected.delete(id); return ok(); },
  clearSelected: () => { selected.clear(); return ok(); },
  dismissAlert: ([id]) => { dismissed.add(id); return ok(); },
  setCleanup: ([id, on]) => { cleanup.set(id, on); return ok(); },
  prepareContainer: async ([id, op], _store, storage) => {
    try {
      if (!['start', 'stop', 'restart'].includes(op)) throw new Error('Unsupported quick operation.');
      const ids = id === '__selected' ? [...selected] : [id];
      if (!ids.length) throw new Error('Select at least one container.');
      const preview = await request('/api/action/preview', storage, 'POST', { kind: 'containers', ids, op, args: [] }) as { confirmation: string; targets: { id: string; name: string }[] };
      if (!preview || typeof preview.confirmation !== 'string' || !Array.isArray(preview.targets)) throw new Error('Invalid confirmation response.');
      return { confirmation: preview.confirmation, names: preview.targets.map(target => `${target.name} · ${target.id}`).join('\n'), error: '' };
    } catch (error) {
      return { confirmation: '', names: '', error: message(error) };
    }
  },
  executeQuick: async ([confirmation], store, storage, native) => {
    const result = await workbenchSources.executeAction([confirmation], store, storage, native);
    snapshotRevision = undefined;
    if (!result.error) selected.clear();
    return { text: result.text, jobId: result.jobId, error: result.error };
  },
  quickJob: async ([id, revision], store, storage, native) => {
    const result = await workbenchSources.job([id, revision], store, storage, native);
    if (result.status === 'succeeded' || result.status === 'failed') snapshotRevision = undefined;
    return { status: result.status, text: result.text, error: result.error };
  },
  previewCleanup: async (_args, _store, storage) => {
    if (!snapshot) return { ok: false, error: 'Refresh the dashboard before previewing cleanup.' };
    try {
      const selectedKinds = snapshot.cleanup.filter(item => item.id !== 'cache' && (cleanup.get(item.id) ?? item.checked)).map(item => item.id);
      if (!selectedKinds.length) return { ok: false, error: 'Choose at least one cleanup category.' };
      const previews = [];
      for (const kind of selectedKinds) {
        const result = await request('/api/action/preview', storage, 'POST', { kind, ids: [], op: 'prune', args: [] }) as { targets: { name: string }[]; summary: string };
        if (!result || !Array.isArray(result.targets)) throw new Error('Invalid cleanup preview.');
        previews.push(`${kind}: ${result.targets.map(target => target.name).join(', ') || 'no candidates'}`);
      }
      return { ok: true, error: `Verified candidates — ${previews.join('; ')}. Nothing was removed. Open the resource page to review and execute cleanup.` };
    } catch (error) {
      return { ok: false, error: message(error) };
    }
  },
};
export const answer: Answer = (source, args, store, storage, native) => sources[source](args, store, storage, native);
