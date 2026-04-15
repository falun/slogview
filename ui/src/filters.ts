import { signal, effect } from '@preact/signals';
import { LEVELS, type Level, type Record } from './types';

// Filter state — all URL-synced, per-tab. No localStorage (values are
// typically bug-specific and shouldn't leak between sessions).

export type AttrOp =
  | 'present'
  | 'absent'
  | 'eq'
  | 'ne'
  | 'regex'
  | 'not-regex'
  | 'lt'
  | 'le'
  | 'gt'
  | 'ge';

export interface AttrPredicate {
  key: string;
  op: AttrOp;
  value?: string;
  numValue?: number;
  re?: RegExp;
}

export interface ParsedAttrs {
  predicates: AttrPredicate[];
  errors: { token: string; error: string }[];
}

export interface MessageFilter {
  kind: 'substring' | 'regex';
  value: string;
  re?: RegExp;
}

export const enabledLevels = signal<Set<Level>>(readLevels());
export const messageQuery = signal<string>(readParam('q') ?? '');
export const attrsQuery = signal<string>(readParam('attrs') ?? '');
export const groupBy = signal<string[]>(readList('group'));
export const groupSelection = signal<string[]>(readList('gsel'));
export const pinnedAttrs = signal<string[]>(readList('pin'));

function readParam(key: string): string | null {
  return new URL(location.href).searchParams.get(key);
}

function readList(key: string): string[] {
  const raw = readParam(key);
  return raw ? raw.split(',').filter(Boolean) : [];
}

function readLevels(): Set<Level> {
  const raw = readParam('levels');
  if (!raw) return new Set(LEVELS);
  const out = new Set<Level>();
  for (const p of raw.split(',')) {
    if ((LEVELS as readonly string[]).includes(p)) out.add(p as Level);
  }
  return out;
}

// Mirror signal state to the URL.
effect(() => {
  const url = new URL(location.href);
  const setOrDel = (k: string, v: string | null) => {
    if (v == null || v === '') url.searchParams.delete(k);
    else url.searchParams.set(k, v);
  };
  const lv = enabledLevels.value;
  setOrDel('levels', lv.size === LEVELS.length ? null : Array.from(lv).join(','));
  setOrDel('q', messageQuery.value || null);
  setOrDel('attrs', attrsQuery.value || null);
  setOrDel('group', groupBy.value.length ? groupBy.value.join(',') : null);
  setOrDel('gsel', groupSelection.value.length ? groupSelection.value.join(',') : null);
  setOrDel('pin', pinnedAttrs.value.length ? pinnedAttrs.value.join(',') : null);
  history.replaceState(null, '', url);
});

export function togglePin(path: string) {
  const cur = pinnedAttrs.value;
  pinnedAttrs.value = cur.includes(path)
    ? cur.filter((p) => p !== path)
    : [...cur, path];
}

export function clearPins() {
  pinnedAttrs.value = [];
}

// flattenAttrs walks the raw record JSON, emitting one entry per leaf value.
// Top-level time/level/msg are skipped because they appear in the row header.
const TOP_SKIP = new Set(['time', 'level', 'msg']);
export function flattenAttrs(
  obj: unknown,
  prefix = '',
): Array<{ path: string; value: unknown }> {
  const out: Array<{ path: string; value: unknown }> = [];
  if (obj === null || typeof obj !== 'object') return out;
  for (const [k, v] of Object.entries(obj as Record$Obj)) {
    if (prefix === '' && TOP_SKIP.has(k)) continue;
    const path = prefix ? `${prefix}.${k}` : k;
    if (v !== null && typeof v === 'object' && !Array.isArray(v)) {
      out.push(...flattenAttrs(v, path));
    } else {
      out.push({ path, value: v });
    }
  }
  return out;
}

export function setGroupBy(paths: string[]) {
  groupBy.value = paths;
  // Truncate any selection that extends beyond the new depth.
  if (groupSelection.value.length > paths.length) {
    groupSelection.value = groupSelection.value.slice(0, paths.length);
  }
}

export function setGroupSelection(sel: string[]) {
  groupSelection.value = sel;
}

export function toggleLevel(l: Level) {
  const next = new Set(enabledLevels.value);
  if (next.has(l)) next.delete(l);
  else next.add(l);
  enabledLevels.value = next;
}

// --- message parser ----------------------------------------------------

// Substring by default (case-insensitive). `/pattern/` triggers regex
// (case-insensitive). Empty query means "no filter".
export function parseMessage(query: string): MessageFilter | null {
  if (!query) return null;
  if (query.length >= 2 && query.startsWith('/') && query.endsWith('/')) {
    const pat = query.slice(1, -1);
    try {
      return { kind: 'regex', value: query, re: new RegExp(pat, 'i') };
    } catch {
      return null;
    }
  }
  return { kind: 'substring', value: query };
}

// --- attr parser -------------------------------------------------------

// Operators in specificity order — two-char operators must be checked first
// so `!=` doesn't get mistaken for a bare `!`-prefixed key or `=` operator.
const OPS: Array<[string, AttrOp]> = [
  ['!=', 'ne'],
  ['!~', 'not-regex'],
  ['<=', 'le'],
  ['>=', 'ge'],
  ['~=', 'regex'],
  ['=', 'eq'],
  ['<', 'lt'],
  ['>', 'gt'],
];

export function parseAttrs(query: string): ParsedAttrs {
  const out: ParsedAttrs = { predicates: [], errors: [] };
  if (!query.trim()) return out;
  for (const tok of query.split(/\s+/).filter(Boolean)) {
    try {
      out.predicates.push(parseOne(tok));
    } catch (e) {
      out.errors.push({ token: tok, error: (e as Error).message });
    }
  }
  return out;
}

function parseOne(tok: string): AttrPredicate {
  // `!key` (absent) — only when no other operator symbol appears.
  if (tok.startsWith('!') && !hasAnyOp(tok)) {
    return { key: tok.slice(1), op: 'absent' };
  }
  for (const [sym, op] of OPS) {
    const idx = tok.indexOf(sym);
    if (idx <= 0) continue;
    const key = tok.slice(0, idx);
    const val = tok.slice(idx + sym.length);
    if (op === 'regex' || op === 'not-regex') {
      try {
        return { key, op, value: val, re: new RegExp(val) };
      } catch (e) {
        throw new Error(`invalid regex: ${(e as Error).message}`);
      }
    }
    if (op === 'lt' || op === 'le' || op === 'gt' || op === 'ge') {
      const n = Number(val);
      if (Number.isNaN(n)) throw new Error('expected number');
      return { key, op, value: val, numValue: n };
    }
    return { key, op, value: val };
  }
  return { key: tok, op: 'present' };
}

function hasAnyOp(tok: string): boolean {
  return /[=<>~]/.test(tok);
}

// --- evaluator ---------------------------------------------------------

// getAttr walks a dotted path through the record's raw JSON (slog emits
// groups as nested objects, so `http.request.id` naturally resolves).
export function getAttr(rec: Record, path: string): unknown {
  const parts = path.split('.');
  let v: unknown = rec.raw;
  for (const p of parts) {
    if (v !== null && typeof v === 'object' && p in (v as Record$Obj)) {
      v = (v as Record$Obj)[p];
    } else {
      return undefined;
    }
  }
  return v;
}
type Record$Obj = { [k: string]: unknown };

export interface CompiledFilter {
  levels: Set<Level>;
  message: MessageFilter | null;
  attrs: AttrPredicate[];
}

export function matches(rec: Record, f: CompiledFilter): boolean {
  if (!f.levels.has(rec.level as Level)) return false;
  if (f.message) {
    if (f.message.kind === 'regex') {
      if (!f.message.re || !f.message.re.test(rec.msg)) return false;
    } else {
      if (!rec.msg.toLowerCase().includes(f.message.value.toLowerCase())) return false;
    }
  }
  for (const p of f.attrs) {
    if (!evalPred(rec, p)) return false;
  }
  return true;
}

function evalPred(rec: Record, p: AttrPredicate): boolean {
  const v = getAttr(rec, p.key);
  switch (p.op) {
    case 'present': return v !== undefined;
    case 'absent':  return v === undefined;
    case 'eq':      return v !== undefined && String(v) === (p.value ?? '');
    case 'ne':      return String(v) !== (p.value ?? '');
    case 'regex':   return p.re !== undefined && p.re.test(String(v ?? ''));
    case 'not-regex': return p.re !== undefined && !p.re.test(String(v ?? ''));
    case 'lt': case 'le': case 'gt': case 'ge': {
      if (p.numValue === undefined) return false;
      const n = typeof v === 'number' ? v : Number(v);
      if (Number.isNaN(n)) return false;
      switch (p.op) {
        case 'lt': return n < p.numValue;
        case 'le': return n <= p.numValue;
        case 'gt': return n > p.numValue;
        case 'ge': return n >= p.numValue;
      }
      return false;
    }
  }
  return false;
}
