import { config } from '../config';
import { connected, records } from '../stream';

interface Props {
  filteredCount: number;
}

export function Footer({ filteredCount }: Props) {
  const c = config.value;
  const live = connected.value;
  const clientCount = records.value.length;
  if (!c) {
    return (
      <div class="footer">
        <span class={live ? 'ok' : 'warn'}>{live ? 'connected' : 'connecting…'}</span>
      </div>
    );
  }
  const mb = (c.stats.bytes / (1024 * 1024)).toFixed(2);
  return (
    <div class="footer">
      <span class={live ? 'ok' : 'warn'}>
        {live ? '●' : '○'} {live ? 'live' : 'reconnecting'}
      </span>
      <span>server: {c.stats.records} rec / {mb} MiB</span>
      <span>mode: {c.stats.mode}</span>
      <span>idle-window: {c.retention.idleWindow}</span>
      <span>level: {c.level}</span>
      <span>
        client: {filteredCount}/{clientCount}
      </span>
    </div>
  );
}
