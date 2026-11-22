import type { Storage } from './app.contract.d.ts';

export const API = 'http://127.0.0.1:17878';
export const TOKEN_FILE = 'app:/data/session-token';

export async function request(path: string, storage: Storage, method = 'GET', body?: unknown): Promise<unknown> {
  let token: string;
  try {
    token = new TextDecoder().decode(await storage.fs.readFile(TOKEN_FILE)).trim();
  } catch (error) {
    if ((error as { code?: string }).code === 'bake') throw error;
    throw new Error('Session token unavailable. Start the app with make run or make demo.');
  }
  if (!token) throw new Error('Session token missing. Restart the local launcher.');
  const response = await fetch(`${API}${path}`, {
    method,
    headers: { 'X-LazyContainer-Token': token, 'Content-Type': 'application/json' },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    exactTimeout: 30000,
  });
  const value = await response.json() as { error?: unknown };
  if (!response.ok) throw Object.assign(new Error(typeof value?.error === 'string' ? value.error : `Backend returned HTTP ${response.status}.`), { details: value });
  return value;
}

export function message(error: unknown): string {
  if ((error as { code?: string })?.code === 'bake') throw error;
  return error instanceof Error ? error.message : 'The local backend could not complete this request.';
}
