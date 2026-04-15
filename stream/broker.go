// Package stream fans out buffer records to live subscribers (SSE clients).
//
// The broker uses a bounded per-subscriber channel. Slow subscribers have
// records dropped on the floor rather than blocking the logger — this is a
// dev tool and a stuck browser tab must not wedge the application.
package stream

import (
	"sync"

	"github.com/falun/slogview/buffer"
)

// Broker is a simple pub/sub over buffer.Record.
type Broker struct {
	mu            sync.Mutex
	subs          map[*Subscriber]struct{}
	onCountChange func(int)
}

// Subscriber is a handle returned to the caller of Subscribe.
type Subscriber struct {
	ch chan buffer.Record
}

// NewBroker creates a broker. onCountChange, if non-nil, is invoked (outside
// the broker lock) whenever the subscriber count changes — typically wired to
// Buffer.SetSubscriberCount to drive idle-trim behavior.
func NewBroker(onCountChange func(int)) *Broker {
	return &Broker{
		subs:          make(map[*Subscriber]struct{}),
		onCountChange: onCountChange,
	}
}

// Subscribe registers a new subscriber with a buffered channel of the given
// size. If bufferSize <= 0 a default is used.
func (b *Broker) Subscribe(bufferSize int) *Subscriber {
	if bufferSize <= 0 {
		bufferSize = 256
	}
	s := &Subscriber{ch: make(chan buffer.Record, bufferSize)}
	b.mu.Lock()
	b.subs[s] = struct{}{}
	n := len(b.subs)
	b.mu.Unlock()
	if b.onCountChange != nil {
		b.onCountChange(n)
	}
	return s
}

// Unsubscribe removes a subscriber and closes its channel. Safe to call twice.
func (b *Broker) Unsubscribe(s *Subscriber) {
	b.mu.Lock()
	_, ok := b.subs[s]
	if ok {
		delete(b.subs, s)
		close(s.ch)
	}
	n := len(b.subs)
	b.mu.Unlock()
	if ok && b.onCountChange != nil {
		b.onCountChange(n)
	}
}

// Publish sends a record to every subscriber non-blockingly. Full channels
// drop the record.
func (b *Broker) Publish(r buffer.Record) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs {
		select {
		case s.ch <- r:
		default:
		}
	}
}

// Count returns the current number of subscribers.
func (b *Broker) Count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

// C returns the subscriber's receive channel.
func (s *Subscriber) C() <-chan buffer.Record { return s.ch }
