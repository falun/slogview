package webui_test

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/falun/slogview"
	"github.com/falun/slogview/webui"
)

func newTestServer(t *testing.T, opts slogview.Options) (*slogview.Handler, *httptest.Server) {
	t.Helper()
	h := slogview.New(opts)
	t.Cleanup(func() { _ = h.Close() })
	s := httptest.NewServer(webui.NewServer(h))
	t.Cleanup(s.Close)
	return h, s
}

func TestGetConfig(t *testing.T) {
	h, s := newTestServer(t, slogview.Options{})
	_ = h
	resp, err := http.Get(s.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["level"] != "INFO" {
		t.Fatalf("level: got %v", got["level"])
	}
	stats := got["stats"].(map[string]any)
	if stats["mode"] != "idle" {
		t.Fatalf("mode: got %v", stats["mode"])
	}
}

func TestPutConfigChangesLevel(t *testing.T) {
	h, s := newTestServer(t, slogview.Options{})
	req, _ := http.NewRequest(http.MethodPut, s.URL+"/api/config", strings.NewReader(`{"level":"DEBUG"}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if h.Level() != slog.LevelDebug {
		t.Fatalf("level not updated: %v", h.Level())
	}
}

func TestSSEStream(t *testing.T) {
	h, s := newTestServer(t, slogview.Options{})

	// Pre-populate one record so we can verify snapshot replay.
	slog.New(h).Info("first", "k", 1)

	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type: %q", got)
	}

	events := make(chan string, 4)
	go readEvents(resp.Body, events)

	// Snapshot replay of "first".
	got := expectEvent(t, events, time.Second)
	if !strings.Contains(got, `"msg":"first"`) {
		t.Fatalf("snapshot event: %q", got)
	}

	// Live record.
	slog.New(h).Info("second", "k", 2)
	got = expectEvent(t, events, time.Second)
	if !strings.Contains(got, `"msg":"second"`) {
		t.Fatalf("live event: %q", got)
	}
}

func TestSSESubscriberFlipsMode(t *testing.T) {
	h, s := newTestServer(t, slogview.Options{})
	if h.Buffer().Stats().Mode != "idle" {
		t.Fatalf("start mode: %v", h.Buffer().Stats().Mode)
	}

	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	// Give the server a moment to install the subscription.
	for i := 0; i < 20; i++ {
		if h.Buffer().Stats().Mode == "active" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if h.Buffer().Stats().Mode != "active" {
		t.Fatalf("mode should be active after subscribe, got %v", h.Buffer().Stats().Mode)
	}
	resp.Body.Close()

	for i := 0; i < 20; i++ {
		if h.Buffer().Stats().Mode == "idle" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if h.Buffer().Stats().Mode != "idle" {
		t.Fatalf("mode should be idle after client disconnect, got %v", h.Buffer().Stats().Mode)
	}
}

func readEvents(body io.Reader, out chan<- string) {
	r := bufio.NewReader(body)
	var dataLines []string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			close(out)
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(dataLines) > 0 {
				out <- strings.Join(dataLines, "\n")
				dataLines = dataLines[:0]
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
}

func expectEvent(t *testing.T, ch <-chan string, d time.Duration) string {
	t.Helper()
	select {
	case e, ok := <-ch:
		if !ok {
			t.Fatal("event channel closed")
		}
		return e
	case <-time.After(d):
		t.Fatalf("timed out waiting for event")
		return ""
	}
}
