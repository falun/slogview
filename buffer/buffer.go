// Package buffer stores slog records in memory with configurable retention.
//
// Records are kept in sequence order (monotonically increasing Seq). The
// buffer is "subscriber-aware": when no subscribers are connected, records
// older than Retention.IdleWindow are evicted so idle processes do not hold
// unbounded memory. Hard caps on count and bytes always apply.
package buffer

import (
	"log/slog"
	"time"
)

// Record is the in-memory representation of a single slog record.
// Raw is the JSON produced by slog's stdlib JSONHandler (one line, no trailing
// newline) so filtering/grouping can happen client-side without re-serializing.
type Record struct {
	Seq   uint64
	Time  time.Time
	Level slog.Level
	Raw   []byte
}

// Cursor is a position in the buffer keyed by Record.Seq. Snapshot returns
// records strictly after the cursor; the zero Cursor returns everything.
type Cursor uint64

// Retention configures eviction policy. Zero fields mean "unbounded" for the
// hard caps and "no time-based trim" for IdleWindow / MaxAge.
type Retention struct {
	// IdleWindow: when no subscribers are present, records older than this
	// are evicted. Zero disables idle trimming.
	IdleWindow time.Duration

	// MaxAge: records older than this are always evicted (regardless of
	// subscriber presence). Zero disables age-based trimming.
	MaxAge time.Duration

	// MaxRecords: hard cap on record count. Zero means unlimited.
	MaxRecords int

	// MaxBytes: hard cap on sum of len(Raw). Zero means unlimited.
	MaxBytes int
}

// Stats is a point-in-time snapshot of buffer state, surfaced via /api/config.
type Stats struct {
	Records     int
	Bytes       int
	Oldest      time.Time
	Newest      time.Time
	Subscribers int
	Mode        string // "active" (subscribers present) or "idle"
	Retention   Retention
}

// Buffer stores records and supports snapshot-then-tail for SSE clients.
// All methods are safe for concurrent use.
type Buffer interface {
	// Append adds a record. Eviction is applied synchronously.
	Append(r Record)

	// Snapshot returns records with Seq strictly greater than since, up to
	// max records (0 = no limit). The returned cursor is the Seq of the last
	// record returned, or since if none were returned.
	Snapshot(since Cursor, max int) ([]Record, Cursor)

	// Stats returns current state.
	Stats() Stats

	// SetSubscriberCount is called by the stream broker when subscriber
	// presence changes. The buffer uses this to enable/disable idle trim.
	SetSubscriberCount(n int)

	// Close releases any background resources (e.g. idle trim ticker).
	Close() error
}
