import { useVirtualizer } from '@tanstack/react-virtual';
import { useRef, useEffect, useState, useLayoutEffect } from 'preact/hooks';
import type { Record } from '../types';
import { RecordRow } from './RecordRow';

interface Props {
  records: Record[];
}

// Virtualized log pane. Autoscroll stays on while the user is at the bottom.
// Scrolling up by more than THRESHOLD_PX disengages it; a "jump to live"
// pill re-engages.
const THRESHOLD_PX = 40;

export function LogList({ records }: Props) {
  const parentRef = useRef<HTMLDivElement>(null);
  const [autoscroll, setAutoscroll] = useState(true);

  const virtualizer = useVirtualizer({
    count: records.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 24,
    overscan: 20,
  });

  // Pin to the bottom when new records arrive and autoscroll is on.
  useLayoutEffect(() => {
    if (autoscroll && records.length > 0) {
      virtualizer.scrollToIndex(records.length - 1, { align: 'end' });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [records.length, autoscroll]);

  useEffect(() => {
    const el = parentRef.current;
    if (!el) return;
    function onScroll() {
      const nearBottom =
        el!.scrollHeight - el!.scrollTop - el!.clientHeight < THRESHOLD_PX;
      setAutoscroll((prev) => (prev !== nearBottom ? nearBottom : prev));
    }
    el.addEventListener('scroll', onScroll);
    return () => el.removeEventListener('scroll', onScroll);
  }, []);

  const items = virtualizer.getVirtualItems();

  return (
    <div class="log-list" ref={parentRef}>
      <div
        style={{
          height: `${virtualizer.getTotalSize()}px`,
          position: 'relative',
          width: '100%',
        }}
      >
        {items.map((v) => (
          <div
            key={v.key}
            data-index={v.index}
            ref={virtualizer.measureElement}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              transform: `translateY(${v.start}px)`,
            }}
          >
            <RecordRow record={records[v.index]} />
          </div>
        ))}
      </div>
      {!autoscroll && (
        <button
          type="button"
          class="jump-live"
          onClick={() => {
            setAutoscroll(true);
            if (records.length > 0) {
              virtualizer.scrollToIndex(records.length - 1, { align: 'end' });
            }
          }}
        >
          ▼ live
        </button>
      )}
    </div>
  );
}
