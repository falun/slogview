package buffer

import (
	"sync"
	"time"
)

// Ring is the default Buffer implementation. It stores records in a slice in
// Seq order and evicts oldest-first when retention limits are exceeded.
type Ring struct {
	mu        sync.Mutex
	records   []Record
	bytes     int
	subs      int
	retention Retention
	clock     func() time.Time

	stopTrim chan struct{}
}

// NewRing creates a Ring with the given retention policy. clock may be nil
// (defaults to time.Now). If IdleWindow > 0, a background trimmer runs while
// no subscribers are connected.
func NewRing(retention Retention, clock func() time.Time) *Ring {
	if clock == nil {
		clock = time.Now
	}
	r := &Ring{retention: retention, clock: clock}
	// subscribers start at 0; if either time-based policy is set we need a
	// background ticker so the trim happens even when no Append is firing.
	if retention.IdleWindow > 0 || retention.MaxAge > 0 {
		r.startIdleTrim()
	}
	return r
}

func (r *Ring) Append(rec Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
	r.bytes += len(rec.Raw)
	r.enforce(r.clock())
}

// enforce applies all retention rules. Caller must hold r.mu.
func (r *Ring) enforce(now time.Time) {
	if r.retention.MaxRecords > 0 {
		for len(r.records) > r.retention.MaxRecords {
			r.evictHead()
		}
	}
	if r.retention.MaxBytes > 0 {
		for r.bytes > r.retention.MaxBytes && len(r.records) > 0 {
			r.evictHead()
		}
	}
	if r.retention.MaxAge > 0 {
		cutoff := now.Add(-r.retention.MaxAge)
		for len(r.records) > 0 && r.records[0].Time.Before(cutoff) {
			r.evictHead()
		}
	}
	if r.subs == 0 && r.retention.IdleWindow > 0 {
		cutoff := now.Add(-r.retention.IdleWindow)
		for len(r.records) > 0 && r.records[0].Time.Before(cutoff) {
			r.evictHead()
		}
	}
	// Periodically reallocate the backing array so repeatedly evicting the
	// head doesn't leak memory. Trigger when the slice is mostly empty.
	if cap(r.records) > 1024 && len(r.records)*4 < cap(r.records) {
		fresh := make([]Record, len(r.records))
		copy(fresh, r.records)
		r.records = fresh
	}
}

func (r *Ring) evictHead() {
	r.bytes -= len(r.records[0].Raw)
	r.records = r.records[1:]
}

func (r *Ring) Snapshot(since Cursor, max int) ([]Record, Cursor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Records are in Seq order; find the first one with Seq > since.
	// Callers usually want recent data, so scan from the tail backwards.
	start := 0
	for i := len(r.records) - 1; i >= 0; i-- {
		if r.records[i].Seq <= uint64(since) {
			start = i + 1
			break
		}
	}
	end := len(r.records)
	if max > 0 && end-start > max {
		start = end - max
	}
	out := make([]Record, end-start)
	copy(out, r.records[start:end])
	newest := since
	if end > 0 {
		newest = Cursor(r.records[end-1].Seq)
	}
	return out, newest
}

func (r *Ring) Stats() Stats {
	r.mu.Lock()
	defer r.mu.Unlock()
	var oldest, newest time.Time
	if len(r.records) > 0 {
		oldest = r.records[0].Time
		newest = r.records[len(r.records)-1].Time
	}
	mode := "active"
	if r.subs == 0 {
		mode = "idle"
	}
	return Stats{
		Records:     len(r.records),
		Bytes:       r.bytes,
		Oldest:      oldest,
		Newest:      newest,
		Subscribers: r.subs,
		Mode:        mode,
		Retention:   r.retention,
	}
}

func (r *Ring) SetSubscriberCount(n int) {
	r.mu.Lock()
	prev := r.subs
	r.subs = n
	// On transitions we manage the trim ticker. We need a ticker whenever
	// any time-based policy is active. If MaxAge is set we keep ticking
	// regardless of subs; if only IdleWindow is set we tick only while idle
	// so it doesn't fire pointlessly under load.
	needTickerWhenIdle := r.retention.IdleWindow > 0 || r.retention.MaxAge > 0
	needTickerAlways := r.retention.MaxAge > 0
	if needTickerWhenIdle && prev > 0 && n == 0 {
		r.startIdleTrimLocked()
	} else if !needTickerAlways && prev == 0 && n > 0 {
		r.stopIdleTrimLocked()
	}
	if n == 0 {
		r.enforce(r.clock())
	}
	r.mu.Unlock()
}

func (r *Ring) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopIdleTrimLocked()
	return nil
}

func (r *Ring) startIdleTrim() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.startIdleTrimLocked()
}

func (r *Ring) startIdleTrimLocked() {
	if r.stopTrim != nil {
		return
	}
	// Pick the tightest interval among configured time-based policies.
	candidates := []time.Duration{r.retention.IdleWindow, r.retention.MaxAge}
	var smallest time.Duration
	for _, c := range candidates {
		if c > 0 && (smallest == 0 || c < smallest) {
			smallest = c
		}
	}
	interval := smallest / 3
	if interval < time.Second {
		interval = time.Second
	}
	stop := make(chan struct{})
	r.stopTrim = stop
	go r.trimLoop(stop, interval)
}

func (r *Ring) stopIdleTrimLocked() {
	if r.stopTrim != nil {
		close(r.stopTrim)
		r.stopTrim = nil
	}
}

func (r *Ring) trimLoop(stop <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			r.mu.Lock()
			r.enforce(r.clock())
			r.mu.Unlock()
		}
	}
}
