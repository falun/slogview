package webui

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type configResponse struct {
	Level     string        `json:"level"`
	Retention retentionJSON `json:"retention"`
	Stats     statsJSON     `json:"stats"`
}

type retentionJSON struct {
	IdleWindow string `json:"idleWindow"`
	MaxAge     string `json:"maxAge"`
	MaxRecords int    `json:"maxRecords"`
	MaxBytes   int    `json:"maxBytes"`
}

type statsJSON struct {
	Records     int    `json:"records"`
	Bytes       int    `json:"bytes"`
	Subscribers int    `json:"subscribers"`
	Mode        string `json:"mode"`
	Oldest      string `json:"oldest,omitempty"`
	Newest      string `json:"newest,omitempty"`
}

func getConfigHandler(a Adapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st := a.Buffer().Stats()
		resp := configResponse{
			Level: a.Level().String(),
			Retention: retentionJSON{
				IdleWindow: st.Retention.IdleWindow.String(),
				MaxAge:     st.Retention.MaxAge.String(),
				MaxRecords: st.Retention.MaxRecords,
				MaxBytes:   st.Retention.MaxBytes,
			},
			Stats: statsJSON{
				Records:     st.Records,
				Bytes:       st.Bytes,
				Subscribers: st.Subscribers,
				Mode:        st.Mode,
			},
		}
		if !st.Oldest.IsZero() {
			resp.Stats.Oldest = st.Oldest.Format(time.RFC3339Nano)
		}
		if !st.Newest.IsZero() {
			resp.Stats.Newest = st.Newest.Format(time.RFC3339Nano)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func putConfigHandler(a Adapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Level string `json:"level"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.Level != "" {
			var l slog.Level
			if err := l.UnmarshalText([]byte(body.Level)); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			a.SetLevel(l)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
