import {
  enabledLevels,
  toggleLevel,
  messageQuery,
  attrsQuery,
  parseAttrs,
  groupBy,
  setGroupBy,
} from '../filters';
import { LEVELS, type Record } from '../types';
import { DumpMenu } from './DumpMenu';

interface Props {
  visibleRecords: Record[];
}

export function Toolbar({ visibleRecords }: Props) {
  const set = enabledLevels.value;
  const msg = messageQuery.value;
  const atr = attrsQuery.value;
  const parsed = parseAttrs(atr);

  return (
    <div class="toolbar">
      <div class="chips">
        {LEVELS.map((l) => (
          <button
            key={l}
            type="button"
            class={`chip chip-${l.toLowerCase()} ${set.has(l) ? 'on' : 'off'}`}
            onClick={() => toggleLevel(l)}
          >
            {l}
          </button>
        ))}
      </div>
      <input
        class="search"
        type="text"
        spellcheck={false}
        placeholder="message (substring, or /regex/)"
        value={msg}
        onInput={(e) => {
          messageQuery.value = (e.target as HTMLInputElement).value;
        }}
      />
      <input
        class="search group"
        type="text"
        spellcheck={false}
        placeholder="group by: attr.path[,attr.path]"
        value={groupBy.value.join(',')}
        onInput={(e) => {
          const v = (e.target as HTMLInputElement).value;
          setGroupBy(v.split(',').map((s) => s.trim()).filter(Boolean));
        }}
      />
      <input
        class={`search attrs ${parsed.errors.length ? 'err' : ''}`}
        type="text"
        spellcheck={false}
        placeholder="attrs: key  !key  k=v  k!=v  k~=re  k!~re  k<N  k<=N  k>N  k>=N"
        value={atr}
        title={parsed.errors.map((e) => `${e.token}: ${e.error}`).join('\n') || ''}
        onInput={(e) => {
          attrsQuery.value = (e.target as HTMLInputElement).value;
        }}
      />
      <DumpMenu records={visibleRecords} />
    </div>
  );
}
