import { useEffect } from 'preact/hooks';
import { connectStream, records } from './stream';
import { startConfigPolling } from './config';
import {
  enabledLevels,
  messageQuery,
  attrsQuery,
  groupBy,
  parseMessage,
  parseAttrs,
  matches,
} from './filters';
import { Toolbar } from './components/Toolbar';
import { LogList } from './components/LogList';
import { Footer } from './components/Footer';
import { GroupTree, applyGroupSelection } from './components/GroupTree';

export function App() {
  useEffect(() => {
    const stopStream = connectStream();
    const stopConfig = startConfigPolling();
    return () => {
      stopStream();
      stopConfig();
    };
  }, []);

  const all = records.value;
  const levels = enabledLevels.value;
  const message = parseMessage(messageQuery.value);
  const attrs = parseAttrs(attrsQuery.value).predicates;

  // Filters apply first (cheap, narrows the working set), then group selection.
  const filtered = all.filter((r) => matches(r, { levels, message, attrs }));
  const visible = applyGroupSelection(filtered);

  const hasGroup = groupBy.value.length > 0;

  return (
    <div class={`app ${hasGroup ? 'has-group' : ''}`}>
      <Toolbar visibleRecords={visible} />
      {hasGroup && <GroupTree records={filtered} />}
      <LogList records={visible} />
      <Footer filteredCount={visible.length} />
    </div>
  );
}
