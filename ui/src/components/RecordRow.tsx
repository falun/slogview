import { useState } from 'preact/hooks';
import type { Record } from '../types';
import {
  pinnedAttrs,
  togglePin,
  getAttr,
  flattenAttrs,
} from '../filters';

interface Props {
  record: Record;
}

// Renders a single log record. Collapsed shows time · level · msg + pinned
// attr badges. Clicking expands to a full attr listing where each row has a
// pin toggle.
export function RecordRow({ record }: Props) {
  const [open, setOpen] = useState(false);
  const pins = pinnedAttrs.value;
  const t = formatTime(record.time);
  const levelClass = `level-${String(record.level).toLowerCase()}`;

  const pinnedView = pins
    .map((path) => ({ path, value: getAttr(record, path) }))
    .filter((x) => x.value !== undefined);

  return (
    <div class={`row ${levelClass}`} onClick={() => setOpen((o) => !o)}>
      <div class="row-head">
        <span class="time">{t}</span>
        <span class="level">{record.level}</span>
        <span class="msg">{record.msg}</span>
        {pinnedView.map(({ path, value }) => (
          <span key={path} class="pin-badge">
            <span class="pk">{path}</span>={formatValue(value)}
          </span>
        ))}
      </div>
      {open && (
        <div class="attrs" onClick={(e) => e.stopPropagation()}>
          {flattenAttrs(record.raw).map(({ path, value }) => {
            const isPinned = pins.includes(path);
            return (
              <div key={path} class={`attr-row ${isPinned ? 'pinned' : ''}`}>
                <button
                  type="button"
                  class="pin-btn"
                  title={isPinned ? 'unpin' : 'pin in collapsed view'}
                  onClick={() => togglePin(path)}
                >
                  {isPinned ? '★' : '☆'}
                </button>
                <span class="ap">{path}</span>
                <span class="av">{formatValue(value)}</span>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

function formatTime(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const pad = (n: number, w = 2) => String(n).padStart(w, '0');
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`;
}

function formatValue(v: unknown): string {
  if (v === null) return 'null';
  if (v === undefined) return '';
  if (typeof v === 'string') return v;
  return JSON.stringify(v);
}
