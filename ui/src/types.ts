// Levels match slog.Level.String() values.
export const LEVELS = ['DEBUG', 'INFO', 'WARN', 'ERROR'] as const;
export type Level = typeof LEVELS[number];

// Record holds what the UI needs for rendering. `raw` is the parsed JSON
// object produced by slog's stdlib JSONHandler — the source of truth for any
// attribute-based filtering or grouping added later.
export interface Record {
  seq: number;
  time: string;
  level: Level | string;
  msg: string;
  raw: Record$JSON;
}

// Record$JSON is the raw shape from the SSE `data:` line.
export type Record$JSON = {
  time?: string;
  level?: string;
  msg?: string;
  [k: string]: unknown;
};
