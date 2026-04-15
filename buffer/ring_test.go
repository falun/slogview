package buffer

import (
	"fmt"
	"log/slog"
	"testing"
	"time"
)

func mkRecord(seq uint64, t time.Time, payload string) Record {
	return Record{
		Seq:   seq,
		Time:  t,
		Level: slog.LevelInfo,
		Raw:   []byte(payload),
	}
}

func TestRingAppendAndSnapshot(t *testing.T) {
	r := NewRing(Retention{}, nil)
	defer r.Close()

	now := time.Unix(1_700_000_000, 0)
	for i := uint64(1); i <= 5; i++ {
		r.Append(mkRecord(i, now.Add(time.Duration(i)*time.Second), fmt.Sprintf(`{"seq":%d}`, i)))
	}

	got, cur := r.Snapshot(0, 0)
	if len(got) != 5 {
		t.Fatalf("want 5 records, got %d", len(got))
	}
	if cur != 5 {
		t.Fatalf("want cursor 5, got %d", cur)
	}
	got, cur = r.Snapshot(3, 0)
	if len(got) != 2 || got[0].Seq != 4 || cur != 5 {
		t.Fatalf("snapshot since 3: got %d records, cur %d", len(got), cur)
	}
	got, _ = r.Snapshot(0, 2)
	if len(got) != 2 || got[0].Seq != 4 || got[1].Seq != 5 {
		t.Fatalf("snapshot max=2 should return tail: got seqs %d,%d", got[0].Seq, got[1].Seq)
	}
}

func TestRingMaxRecords(t *testing.T) {
	r := NewRing(Retention{MaxRecords: 3}, nil)
	defer r.Close()

	now := time.Unix(1_700_000_000, 0)
	for i := uint64(1); i <= 10; i++ {
		r.Append(mkRecord(i, now, fmt.Sprintf(`{"seq":%d}`, i)))
	}
	st := r.Stats()
	if st.Records != 3 {
		t.Fatalf("want 3 records retained, got %d", st.Records)
	}
	got, _ := r.Snapshot(0, 0)
	if got[0].Seq != 8 || got[2].Seq != 10 {
		t.Fatalf("expected seqs 8..10, got %d..%d", got[0].Seq, got[2].Seq)
	}
}

func TestRingMaxBytes(t *testing.T) {
	r := NewRing(Retention{MaxBytes: 30}, nil)
	defer r.Close()

	now := time.Unix(1_700_000_000, 0)
	// each record is 10 bytes ("xxxxxxxxxx")
	for i := uint64(1); i <= 5; i++ {
		r.Append(mkRecord(i, now, "xxxxxxxxxx"))
	}
	st := r.Stats()
	if st.Records != 3 || st.Bytes != 30 {
		t.Fatalf("want 3 records / 30 bytes, got %d / %d", st.Records, st.Bytes)
	}
}

func TestRingIdleWindowTrimsOnSubscriberCountChange(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }

	r := NewRing(Retention{IdleWindow: 30 * time.Second}, clock)
	defer r.Close()

	// simulate an active subscriber during ingest
	r.SetSubscriberCount(1)

	// t=0..60s, one record per second. With a live subscriber, no idle trim.
	base := now
	for i := 0; i < 60; i++ {
		now = base.Add(time.Duration(i) * time.Second)
		r.Append(mkRecord(uint64(i+1), now, "x"))
	}
	if st := r.Stats(); st.Records != 60 {
		t.Fatalf("active mode should keep all: got %d", st.Records)
	}

	// Subscriber leaves — should trim to 30s window relative to the current clock.
	r.SetSubscriberCount(0)
	st := r.Stats()
	if st.Records == 60 {
		t.Fatalf("idle transition should have trimmed; still have %d", st.Records)
	}
	// The newest record is at base+59s; records older than base+29s go away.
	// cutoff = base+59s - 30s = base+29s. Records with time < base+29s are dropped.
	for _, rec := range must(r.Snapshot(0, 0)) {
		if rec.Time.Before(base.Add(29 * time.Second)) {
			t.Fatalf("record at %v should have been trimmed", rec.Time)
		}
	}
}

func TestRingStatsModeReflectsSubscribers(t *testing.T) {
	r := NewRing(Retention{}, nil)
	defer r.Close()
	if r.Stats().Mode != "idle" {
		t.Fatal("no subs should be idle")
	}
	r.SetSubscriberCount(2)
	if s := r.Stats(); s.Mode != "active" || s.Subscribers != 2 {
		t.Fatalf("expected active/2, got %s/%d", s.Mode, s.Subscribers)
	}
	r.SetSubscriberCount(0)
	if r.Stats().Mode != "idle" {
		t.Fatal("back to idle")
	}
}

// must is a test helper that drops the cursor return value.
func must(recs []Record, _ Cursor) []Record { return recs }

func TestRingMaxAgeAlwaysApplies(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }

	r := NewRing(Retention{MaxAge: 10 * time.Second}, clock)
	defer r.Close()

	// Active subscriber should NOT prevent MaxAge eviction.
	r.SetSubscriberCount(1)

	base := now
	for i := 0; i < 30; i++ {
		now = base.Add(time.Duration(i) * time.Second)
		r.Append(mkRecord(uint64(i+1), now, "x"))
	}
	st := r.Stats()
	for _, rec := range must(r.Snapshot(0, 0)) {
		if rec.Time.Before(now.Add(-10 * time.Second)) {
			t.Fatalf("record at %v older than MaxAge: %d retained", rec.Time, st.Records)
		}
	}
}
