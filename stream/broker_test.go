package stream

import (
	"sync"
	"testing"
	"time"

	"github.com/falun/slogview/buffer"
)

func TestBrokerFanOut(t *testing.T) {
	b := NewBroker(nil)
	s1 := b.Subscribe(4)
	s2 := b.Subscribe(4)

	b.Publish(buffer.Record{Seq: 1})
	b.Publish(buffer.Record{Seq: 2})

	for _, s := range []*Subscriber{s1, s2} {
		for want := uint64(1); want <= 2; want++ {
			select {
			case r := <-s.C():
				if r.Seq != want {
					t.Fatalf("want seq %d, got %d", want, r.Seq)
				}
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for record")
			}
		}
	}
}

func TestBrokerPresenceCallback(t *testing.T) {
	var mu sync.Mutex
	var counts []int
	b := NewBroker(func(n int) {
		mu.Lock()
		counts = append(counts, n)
		mu.Unlock()
	})

	s1 := b.Subscribe(1)
	s2 := b.Subscribe(1)
	b.Unsubscribe(s1)
	b.Unsubscribe(s2)

	mu.Lock()
	defer mu.Unlock()
	want := []int{1, 2, 1, 0}
	if len(counts) != len(want) {
		t.Fatalf("want %v, got %v", want, counts)
	}
	for i, c := range counts {
		if c != want[i] {
			t.Fatalf("counts[%d] = %d, want %d", i, c, want[i])
		}
	}
}

func TestBrokerDropsOnSlowSubscriber(t *testing.T) {
	b := NewBroker(nil)
	s := b.Subscribe(2) // buffer size 2
	for i := uint64(0); i < 100; i++ {
		b.Publish(buffer.Record{Seq: i})
	}
	// Subscriber should have at most 2 records buffered; publishes did not block.
	got := 0
	drain := time.After(50 * time.Millisecond)
loop:
	for {
		select {
		case <-s.C():
			got++
		case <-drain:
			break loop
		}
	}
	if got > 2 {
		t.Fatalf("expected <=2 buffered records, got %d", got)
	}
	b.Unsubscribe(s)
}

func TestBrokerUnsubscribeIdempotent(t *testing.T) {
	b := NewBroker(nil)
	s := b.Subscribe(1)
	b.Unsubscribe(s)
	b.Unsubscribe(s) // must not panic
}
