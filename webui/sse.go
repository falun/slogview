package webui

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/falun/slogview/buffer"
)

// streamHandler serves the live record stream. It subscribes first (so no
// records are missed between snapshot and tail), replays snapshot-since-cursor,
// then forwards live records. The record's Seq is used as the SSE event id so
// EventSource's automatic reconnect with Last-Event-ID works.
func streamHandler(a Adapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		since := startCursor(r)

		sub := a.Broker().Subscribe(0)
		defer a.Broker().Unsubscribe(sub)

		snap, lastSeq := a.Buffer().Snapshot(since, 0)
		for _, rec := range snap {
			writeEvent(w, rec)
		}
		flusher.Flush()

		ctx := r.Context()
		ch := sub.C()
		for {
			select {
			case <-ctx.Done():
				return
			case rec, ok := <-ch:
				if !ok {
					return
				}
				// Skip records covered by the snapshot (can happen because
				// we subscribed before snapshotting).
				if rec.Seq <= uint64(lastSeq) {
					continue
				}
				writeEvent(w, rec)
				flusher.Flush()
			}
		}
	}
}

func startCursor(r *http.Request) buffer.Cursor {
	if s := r.URL.Query().Get("since"); s != "" {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
			return buffer.Cursor(n)
		}
	}
	if s := r.Header.Get("Last-Event-ID"); s != "" {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
			return buffer.Cursor(n)
		}
	}
	return 0
}

func writeEvent(w http.ResponseWriter, rec buffer.Record) {
	fmt.Fprintf(w, "id: %d\ndata: %s\n\n", rec.Seq, rec.Raw)
}
