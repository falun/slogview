import {
  groupBy,
  groupSelection,
  setGroupSelection,
  getAttr,
} from '../filters';
import type { Record } from '../types';

interface Node {
  value: string;
  count: number;
  children: Map<string, Node>;
}

interface Props {
  records: Record[];
}

// Sentinel shown for records that are missing a value at a given group path.
const MISSING = '∅';

export function GroupTree({ records }: Props) {
  const paths = groupBy.value;
  const sel = groupSelection.value;
  if (paths.length === 0) return null;

  const root = buildTree(records, paths);

  return (
    <div class="group-tree">
      <div class="tree-head">
        <strong>group</strong>
        <span class="muted">{paths.join(' › ')}</span>
        {sel.length > 0 && (
          <button
            type="button"
            class="clear"
            onClick={() => setGroupSelection([])}
          >
            clear
          </button>
        )}
      </div>
      <TreeLevel nodes={root.children} depth={0} sel={sel} />
    </div>
  );
}

function buildTree(records: Record[], paths: string[]): Node {
  const root: Node = { value: '', count: 0, children: new Map() };
  for (const rec of records) {
    let node = root;
    node.count++;
    for (const p of paths) {
      const v = getAttr(rec, p);
      const key = v === undefined ? MISSING : String(v);
      let child = node.children.get(key);
      if (!child) {
        child = { value: key, count: 0, children: new Map() };
        node.children.set(key, child);
      }
      child.count++;
      node = child;
    }
  }
  return root;
}

interface LevelProps {
  nodes: Map<string, Node>;
  depth: number;
  sel: string[];
}

function TreeLevel({ nodes, depth, sel }: LevelProps) {
  const selected = sel[depth];
  // Sort by count descending (most-frequent group first).
  const entries = Array.from(nodes.entries()).sort(
    (a, b) => b[1].count - a[1].count,
  );
  return (
    <ul class="tree-level">
      {entries.map(([key, node]) => {
        const isSel = selected === key;
        return (
          <li key={key}>
            <div
              class={`tree-node ${isSel ? 'sel' : ''}`}
              onClick={() => {
                if (isSel) {
                  // Click selected node → deselect (and anything deeper).
                  setGroupSelection(sel.slice(0, depth));
                } else {
                  setGroupSelection([...sel.slice(0, depth), key]);
                }
              }}
            >
              <span class="val">{key}</span>
              <span class="count">{node.count}</span>
            </div>
            {isSel && node.children.size > 0 && (
              <TreeLevel
                nodes={node.children}
                depth={depth + 1}
                sel={sel}
              />
            )}
          </li>
        );
      })}
    </ul>
  );
}

// applyGroupSelection narrows records to those matching the current selection
// at each group depth. Exported so App can use it without importing the
// component module twice.
export function applyGroupSelection(records: Record[]): Record[] {
  const paths = groupBy.value;
  const sel = groupSelection.value;
  if (paths.length === 0 || sel.length === 0) return records;
  return records.filter((rec) =>
    sel.every((want, d) => {
      const v = getAttr(rec, paths[d]);
      return (v === undefined ? MISSING : String(v)) === want;
    }),
  );
}
