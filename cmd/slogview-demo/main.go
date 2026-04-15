// Command slogview-demo runs a slogview-backed logger that emits synthetic
// log records at a steady rate, so you can point a browser (or curl) at the
// UI and watch records flow through.
//
// Usage:
//
//	go run ./cmd/slogview-demo
//	# in another terminal:
//	curl -N http://127.0.0.1:8787/api/stream
//	curl    http://127.0.0.1:8787/api/config
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/falun/slogview"
	"github.com/falun/slogview/webui"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address for the slogview UI/API")
	rate := flag.Duration("rate", 500*time.Millisecond, "interval between synthetic log records")
	flag.Parse()

	h := slogview.New(slogview.Options{
		Next: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
	})
	defer h.Close()

	logger := slog.New(h)
	slog.SetDefault(logger)

	server := &http.Server{
		Addr:    *addr,
		Handler: webui.NewServer(h),
	}
	go func() {
		slog.Info("slogview listening", "addr", "http://"+server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	emit(*rate)
}

func emit(interval time.Duration) {
	levels := []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelInfo, slog.LevelInfo, slog.LevelWarn, slog.LevelError}
	messages := []string{"request received", "cache miss", "db query", "user action", "background job", "outbound call"}
	services := []string{"api", "worker", "billing"}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		l := levels[rand.Intn(len(levels))]
		m := messages[rand.Intn(len(messages))]
		slog.Log(context.Background(), l, m,
			"service", services[rand.Intn(len(services))],
			"request_id", fmt.Sprintf("req-%04d", rand.Intn(1000)),
			"user_id", fmt.Sprintf("u-%d", rand.Intn(20)),
			"latency_ms", rand.Intn(500),
		)
	}
}
