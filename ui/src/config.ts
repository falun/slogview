import { signal } from '@preact/signals';

export interface ConfigResponse {
  level: string;
  retention: { idleWindow: string; maxAge: string; maxRecords: number; maxBytes: number };
  stats: {
    records: number;
    bytes: number;
    subscribers: number;
    mode: string;
    oldest?: string;
    newest?: string;
  };
}

export const config = signal<ConfigResponse | null>(null);

export function startConfigPolling(intervalMs = 3000): () => void {
  let cancelled = false;
  async function poll() {
    try {
      const r = await fetch('/api/config');
      if (r.ok && !cancelled) config.value = await r.json();
    } catch {
      // ignore transient errors; next tick retries
    }
  }
  poll();
  const id = window.setInterval(poll, intervalMs);
  return () => { cancelled = true; clearInterval(id); };
}

export async function setLevel(level: string): Promise<void> {
  await fetch('/api/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ level }),
  });
}
