package slogview

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/falun/slogview/buffer"
	"github.com/falun/slogview/stream"
)

// DefaultRetention is used when Options.Retention is the zero value.
var DefaultRetention = buffer.Retention{
	IdleWindow: 30 * time.Second,
	MaxRecords: 0,
	MaxBytes:   64 * 1024 * 1024,
}

// Options configures a new Handler.
type Options struct {
	// Level is the runtime-mutable level gate for records captured into the
	// buffer. Nil means a new LevelVar defaulted to Info. Callers wanting to
	// change the level at runtime should pass their own LevelVar (or call
	// Handler.SetLevel).
	Level *slog.LevelVar

	// Retention controls buffer eviction. Zero value uses DefaultRetention.
	Retention buffer.Retention

	// Next is an optional downstream slog.Handler to tee records to. Typical
	// use: set to a real JSON or text handler writing to stdout so slogview
	// can be layered on top of an existing logging setup.
	Next slog.Handler

	// Clock is used for test injection. Nil means time.Now.
	Clock func() time.Time
}

// Handler is a slog.Handler that captures every record's JSON representation
// into an in-memory buffer and fans it out to live SSE subscribers. It also
// optionally tees records to a downstream handler.
type Handler struct {
	core  *core
	inner slog.Handler // per-instance JSONHandler (carries WithAttrs/WithGroup state)
	next  slog.Handler // per-instance downstream (also cloned by WithAttrs/WithGroup)
}

// core holds state shared across all Handler instances produced by
// WithAttrs/WithGroup from a single New() call.
type core struct {
	level  *slog.LevelVar
	lc     *lineCapture
	buf    buffer.Buffer
	broker *stream.Broker
	seq    atomic.Uint64
	clock  func() time.Time
}

// lineCapture is the io.Writer passed to the inner JSONHandler. Each Handle
// call locks it, resets the buffer, invokes inner.Handle (which writes one
// line), copies the bytes out, and unlocks. The mutex serializes concurrent
// Handle calls through the inner handler. This is probably acceptable for
// now but we may want to retool as pub/sub via channels if it starts to impact
// throughput even in dev scenarios.
type lineCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (lc *lineCapture) Write(p []byte) (int, error) { return lc.buf.Write(p) }

// New constructs a Handler with the given options.
func New(opts Options) *Handler {
	level := opts.Level
	if level == nil {
		level = new(slog.LevelVar)
		level.Set(slog.LevelInfo)
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	retention := opts.Retention
	if retention == (buffer.Retention{}) {
		retention = DefaultRetention
	}

	buf := buffer.NewRing(retention, clock)
	br := stream.NewBroker(buf.SetSubscriberCount)

	lc := &lineCapture{}
	inner := slog.NewJSONHandler(lc, &slog.HandlerOptions{Level: level})

	return &Handler{
		core: &core{
			level:  level,
			lc:     lc,
			buf:    buf,
			broker: br,
			clock:  clock,
		},
		inner: inner,
		next:  opts.Next,
	}
}

// Enabled reports whether we want this record. We accept anything our own
// level allows OR anything our downstream handler accepts, so teeing works
// independently of the buffer's level.
func (h *Handler) Enabled(ctx context.Context, l slog.Level) bool {
	if l >= h.core.level.Level() {
		return true
	}
	if h.next != nil {
		return h.next.Enabled(ctx, l)
	}
	return false
}

// Handle captures the record's JSON via the inner handler, appends to the
// buffer, publishes to the broker, and tees to Next.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if l := h.core.level.Level(); r.Level >= l {
		h.core.lc.mu.Lock()
		h.core.lc.buf.Reset()
		err := h.inner.Handle(ctx, r)
		var raw []byte
		if err == nil {
			b := h.core.lc.buf.Bytes()
			// Drop trailing newline written by JSONHandler.
			for len(b) > 0 && b[len(b)-1] == '\n' {
				b = b[:len(b)-1]
			}
			raw = make([]byte, len(b))
			copy(raw, b)
		}
		h.core.lc.mu.Unlock()
		if err != nil {
			return err
		}
		rec := buffer.Record{
			Seq:   h.core.seq.Add(1),
			Time:  r.Time,
			Level: r.Level,
			Raw:   raw,
		}
		h.core.buf.Append(rec)
		h.core.broker.Publish(rec)
	}

	if h.next != nil && h.next.Enabled(ctx, r.Level) {
		return h.next.Handle(ctx, r)
	}
	return nil
}

// WithAttrs returns a new Handler whose inner and next handlers carry the
// additional attrs. Shared core state is preserved.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var next slog.Handler
	if h.next != nil {
		next = h.next.WithAttrs(attrs)
	}
	return &Handler{
		core:  h.core,
		inner: h.inner.WithAttrs(attrs),
		next:  next,
	}
}

// WithGroup returns a new Handler scoped under the named group.
func (h *Handler) WithGroup(name string) slog.Handler {
	var next slog.Handler
	if h.next != nil {
		next = h.next.WithGroup(name)
	}
	return &Handler{
		core:  h.core,
		inner: h.inner.WithGroup(name),
		next:  next,
	}
}

// SetLevel changes the capture level at runtime.
func (h *Handler) SetLevel(l slog.Level) { h.core.level.Set(l) }

// Level returns the current capture level.
func (h *Handler) Level() slog.Level { return h.core.level.Level() }

// Buffer returns the underlying buffer (used by the web UI server).
func (h *Handler) Buffer() buffer.Buffer { return h.core.buf }

// Broker returns the underlying stream broker (used by the web UI server).
func (h *Handler) Broker() *stream.Broker { return h.core.broker }

// Close releases background resources (idle-trim ticker). The Handler is not
// usable after Close.
func (h *Handler) Close() error { return h.core.buf.Close() }
