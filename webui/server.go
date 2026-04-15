// Package webui serves the slogview web UI and its supporting HTTP API.
//
// Endpoints:
//
//	GET  /api/stream  SSE; id: is the record Seq so EventSource reconnects
//	                  resume automatically. ?since=N picks an explicit
//	                  starting cursor.
//	GET  /api/config  Returns current level, retention policy, and stats.
//	PUT  /api/config  Accepts {"level":"DEBUG|INFO|WARN|ERROR"} to change
//	                  the capture level at runtime. TODO:
//	/*                Serves the embedded UI assets.
package webui

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/falun/slogview/buffer"
	"github.com/falun/slogview/stream"
)

// Adapter is the subset of *slogview.Handler the server needs. Exists to prevent
// import cycles
type Adapter interface {
	Buffer() buffer.Buffer
	Broker() *stream.Broker
	SetLevel(slog.Level)
	Level() slog.Level
}

//go:embed assets
var assetsFS embed.FS

// NewServer returns an http.Handler serving the slogview API and UI.
func NewServer(a Adapter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/stream", streamHandler(a))
	mux.HandleFunc("GET /api/config", getConfigHandler(a))
	mux.HandleFunc("PUT /api/config", putConfigHandler(a))

	sub, err := fs.Sub(assetsFS, "assets")
	if err == nil {
		mux.Handle("/", http.FileServer(http.FS(sub)))
	}
	return mux
}
