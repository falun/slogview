package slogview

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"testing/slogtest"

	"github.com/falun/slogview/buffer"
)

func TestSlogtestConformance(t *testing.T) {
	h := New(Options{Level: levelVar(slog.LevelDebug)})
	defer h.Close()

	err := slogtest.TestHandler(h, func() []map[string]any {
		recs, _ := h.Buffer().Snapshot(0, 0)
		out := make([]map[string]any, 0, len(recs))
		for _, r := range recs {
			var m map[string]any
			if err := json.Unmarshal(r.Raw, &m); err != nil {
				t.Fatalf("bad raw json: %v", err)
			}
			out = append(out, m)
		}
		return out
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCapturesRecord(t *testing.T) {
	h := New(Options{})
	defer h.Close()
	logger := slog.New(h)
	logger.Info("hello", "k", "v")

	recs, _ := h.Buffer().Snapshot(0, 0)
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d", len(recs))
	}
	if recs[0].Level != slog.LevelInfo {
		t.Fatalf("level: got %v", recs[0].Level)
	}
	if !bytes.Contains(recs[0].Raw, []byte(`"msg":"hello"`)) {
		t.Fatalf("raw missing msg: %s", recs[0].Raw)
	}
	if !bytes.Contains(recs[0].Raw, []byte(`"k":"v"`)) {
		t.Fatalf("raw missing attr: %s", recs[0].Raw)
	}
	// No trailing newline.
	if bytes.HasSuffix(recs[0].Raw, []byte("\n")) {
		t.Fatal("raw should not end with newline")
	}
}

func TestTeeToNext(t *testing.T) {
	var sink bytes.Buffer
	next := slog.NewJSONHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := New(Options{Next: next})
	defer h.Close()
	logger := slog.New(h)
	logger.Info("tee test", "n", 42)

	recs, _ := h.Buffer().Snapshot(0, 0)
	if len(recs) != 1 {
		t.Fatalf("buffer: want 1, got %d", len(recs))
	}
	if !strings.Contains(sink.String(), `"msg":"tee test"`) {
		t.Fatalf("next did not receive record: %q", sink.String())
	}
}

func TestWithAttrsPropagatesToInnerAndNext(t *testing.T) {
	var sink bytes.Buffer
	next := slog.NewJSONHandler(&sink, nil)

	h := New(Options{Next: next})
	defer h.Close()
	logger := slog.New(h).With("service", "api")
	logger.Info("hi")

	recs, _ := h.Buffer().Snapshot(0, 0)
	if !bytes.Contains(recs[0].Raw, []byte(`"service":"api"`)) {
		t.Fatalf("buffer missing service attr: %s", recs[0].Raw)
	}
	if !strings.Contains(sink.String(), `"service":"api"`) {
		t.Fatalf("next missing service attr: %s", sink.String())
	}
}

func TestSetLevelGatesCapture(t *testing.T) {
	h := New(Options{Level: levelVar(slog.LevelWarn)})
	defer h.Close()
	logger := slog.New(h)

	logger.Info("should drop")
	logger.Warn("should capture")
	recs, _ := h.Buffer().Snapshot(0, 0)
	if len(recs) != 1 || !bytes.Contains(recs[0].Raw, []byte("should capture")) {
		t.Fatalf("unexpected recs: %+v", recs)
	}

	h.SetLevel(slog.LevelInfo)
	logger.Info("now captured")
	recs, _ = h.Buffer().Snapshot(0, 0)
	if len(recs) != 2 {
		t.Fatalf("want 2 records after SetLevel, got %d", len(recs))
	}
}

func TestPublishesToBroker(t *testing.T) {
	h := New(Options{})
	defer h.Close()

	sub := h.Broker().Subscribe(4)
	defer h.Broker().Unsubscribe(sub)

	slog.New(h).Info("broadcast")

	select {
	case rec := <-sub.C():
		if !bytes.Contains(rec.Raw, []byte("broadcast")) {
			t.Fatalf("bad raw: %s", rec.Raw)
		}
	default:
		t.Fatal("subscriber did not receive record")
	}
}

func TestEnabledRespectsNext(t *testing.T) {
	var sink bytes.Buffer
	next := slog.NewJSONHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := New(Options{Level: levelVar(slog.LevelError), Next: next})
	defer h.Close()

	if !h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("should be Enabled when Next accepts it")
	}
	// Record captured by next, not our buffer.
	logger := slog.New(h)
	logger.Debug("trace")
	if sink.Len() == 0 {
		t.Fatal("next should have received the record")
	}
	recs, _ := h.Buffer().Snapshot(0, 0)
	if _, ok := any(recs).([]buffer.Record); !ok || len(recs) != 0 {
		t.Fatalf("buffer should be empty: %+v", recs)
	}
}

func levelVar(l slog.Level) *slog.LevelVar {
	v := new(slog.LevelVar)
	v.Set(l)
	return v
}
