import type { Answer, Result, Sources, Storage } from './app.contract.d.ts';

export const API = 'http://127.0.0.1:17878';
const TOKEN_FILE = 'app:/data/session-token';
export const grants = `net.fetch ${API}\nfs.read ${TOKEN_FILE}`;

type Dashboard = Result<'dashboard'>;
type Reply = Result<'containerOp'>;
const selected = new Set<string>();
const dismissed = new Set<string>();
const cleanup = new Map<string, boolean>();
let snapshot: Dashboard | undefined;
const ok = (): Reply => ({ ok: true, error: '' });

async function loadDashboard(): Promise<Dashboard> {
  const response = await fetch(`${API}/api/dashboard`, { exactTimeout: 12000 });
  if (!response.ok) throw new Error('Docker backend unavailable. Check its log and socket permissions.');
  const body = await response.json() as Dashboard;
  // Exact validates the complete returned shape; check the fields we traverse first.
  if (!body || !Array.isArray(body.stacks) || !Array.isArray(body.alerts) || !Array.isArray(body.cleanup) ||
      !body.stacks.every(s => s && Array.isArray(s.containers) && s.containers.every(c => c && typeof c.id === 'string' && typeof c.name === 'string' && typeof c.state === 'string')) ||
      !body.alerts.every(a => a && typeof a.id === 'string') || !body.cleanup.every(c => c && typeof c.id === 'string')) {
    throw new Error('Invalid dashboard response.');
  }
  snapshot = body;
  const available = new Set(body.stacks.flatMap(s => s.containers.map(c => c.id)));
  for (const id of selected) if (!available.has(id)) selected.delete(id);
  return body;
}

async function operate(id: string, op: string, storage: Storage): Promise<Reply> {
  if (!['start', 'stop', 'restart'].includes(op)) return { ok: false, error: 'Unsupported operation.' };
  const ids = id === '__selected' ? [...selected] : [id];
  if (!ids.length) return { ok: false, error: 'Select at least one container.' };
  let token: string;
  try {
    token = new TextDecoder().decode(await storage.fs.readFile(TOKEN_FILE)).trim();
    if (!token) return { ok: false, error: 'Session token missing. Restart with the local launcher.' };
  } catch (error) {
    if ((error as { code?: string }).code === 'bake') throw error;
    return { ok: false, error: 'Session token unavailable. Restart with the local launcher.' };
  }
  let completed = 0;
  for (const target of ids) {
    try {
      const response = await fetch(`${API}/api/containers/${encodeURIComponent(target)}/${op}`, {
        method: 'POST', headers: { 'X-LazyContainer-Token': token, 'Content-Type': 'application/json' }, body: '{}', exactTimeout: 30000,
      });
      if (!response.ok) return { ok: false, error: `${completed} of ${ids.length} completed. Backend refused ${op} (HTTP ${response.status}); refresh before retrying.` };
      completed++;
    } catch (error) {
      if ((error as { code?: string }).code === 'bake') throw error;
      return { ok: false, error: `${completed} of ${ids.length} confirmed. Connection lost; refresh to check state before retrying.` };
    }
  }
  selected.clear();
  return ok();
}

const sources: Sources = {
  dashboard: async ([filter, query, showDismissed]) => {
    const data = await loadDashboard();
    const term = query.trim().toLowerCase();
    return {
      ...data,
      selectedCount: selected.size,
      alerts: data.alerts.filter(a => showDismissed || !dismissed.has(a.id)),
      cleanup: data.cleanup.map(item => ({ ...item, checked: cleanup.get(item.id) ?? item.checked })),
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
  containerOp: ([id, op], _store, storage) => operate(id, op, storage),
  previewCleanup: () => {
    if (!snapshot) return { ok: false, error: 'Refresh the dashboard before previewing cleanup.' };
    const containers = snapshot.stacks.flatMap(s => s.containers).filter(c => c.state === 'exited');
    const include = cleanup.get('containers') ?? snapshot.cleanup.find(c => c.id === 'containers')?.checked ?? false;
    const names = include ? containers.map(c => c.name).join(', ') || 'none' : 'not selected';
    return { ok: true, error: `Inventory preview only. Stopped containers: ${names}. Image, volume and cache removal candidates are not verified. Nothing was removed.` };
  },
};
export const answer: Answer = (source, args, store, storage, native) => sources[source](args, store, storage, native);
