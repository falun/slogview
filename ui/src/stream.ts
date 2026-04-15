import { signal } from '@preact/signals';
import type { Record, Record$JSON } from './types';

// Client-side cap. Matches the intent of the server-side ring: bound memory.
// Chosen so that 10s of kilo-records at typical message sizes stay well under
// a few MB in the browser.
const MAX_CLIENT_RECORDS = 50_000;

export const records = signal<Record[]>([]);
export const connected = signal(false);

export function connectStream(): () => void {
  const es = new EventSource('/api/stream');
  es.onopen = () => { connected.value = true; };
  es.onerror = () => { connected.value = false; };
  es.onmessage = (ev) => {
    let parsed: Record$JSON;
    try {
      parsed = JSON.parse(ev.data) as Record$JSON;
    } catch {
      return;
    }
    const seq = Number(ev.lastEventId) || 0;
    const rec: Record = {
      seq,
      time: parsed.time ?? '',
      level: parsed.level ?? 'INFO',
      msg: parsed.msg ?? '',
      raw: parsed,
    };
    const list = records.value;
    let next: Record[];
    if (list.length >= MAX_CLIENT_RECORDS) {
      next = list.slice(list.length - MAX_CLIENT_RECORDS + 1);
      next.push(rec);
    } else {
      next = list.concat(rec);
    }
    records.value = next;
  };
  return () => es.close();
}
