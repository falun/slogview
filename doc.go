// Package slogview is a dev-time log handler that buffers records in
// memory and serves a web UI for live viewing, filtering, and grouping.
//
// It is intended for local development, not production. Allow me to
// restate: DO NOT SHIP TO PROD ENABLED. Ahem.
//
// The handler wraps an inner slog.JSONHandler for formatting and can
// optionally forward records to a downstream handler. It's relatively
// trivial to add into your logging stack in most cases:
//
// ```go
//
//	// grab the original handler that we're tapping into
//	originalHandler := slog.Default().Handler()
//
//	// construct a slogview handler that forwards to the original
//	svHandler := slogview.New(slogview.Options{Next: originalHandler})
//	// replace the default *slog.Logger with the now-wrapped version
//	slog.SetDefault(slog.New(svHandler))
//
//	// Wire up the handler to a webserver for debugging
//	slogHttpHandler := slogui.NewServer(svHandler)
//	go http.ListenAndServe("localhost:1312", slogHttpHandler)
//
// ```
package slogview
