import type { Record } from '../types';

interface Props {
  records: Record[];
}

// Browser-download dump of the currently-visible records (post-filter,
// post-group-selection). JSONL is one record's raw JSON per line; text is a
// human-friendly format. Both produce a Blob that the browser saves.
export function DumpMenu({ records }: Props) {
  function dump(format: 'jsonl' | 'text') {
    const text = format === 'jsonl' ? toJSONL(records) : toText(records);
    const ext = format === 'jsonl' ? 'jsonl' : 'txt';
    const ts = new Date().toISOString().replace(/[:.]/g, '-');
    const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `slogview-${ts}.${ext}`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }
  return (
    <div class="dump">
      <button type="button" onClick={() => dump('jsonl')} title="download visible records as JSONL">
        ↓ jsonl
      </button>
      <button type="button" onClick={() => dump('text')} title="download visible records as text">
        ↓ text
      </button>
    </div>
  );
}

function toJSONL(records: Record[]): string {
  if (records.length === 0) return '';
  return records.map((r) => JSON.stringify(r.raw)).join('\n') + '\n';
}

function toText(records: Record[]): string {
  if (records.length === 0) return '';
  return (
    records
      .map((r) => {
        const t = r.time || '';
        const lvl = String(r.level).padEnd(5);
        const extras = Object.entries(r.raw)
          .filter(([k]) => k !== 'time' && k !== 'level' && k !== 'msg')
          .map(([k, v]) => `${k}=${typeof v === 'string' ? v : JSON.stringify(v)}`)
          .join(' ');
        return `${t} ${lvl} ${r.msg}${extras ? ' ' + extras : ''}`;
      })
      .join('\n') + '\n'
  );
}
