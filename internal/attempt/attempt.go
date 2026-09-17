package attempt

import (
	"sync"
	"time"
)

// A client retransmits only when nothing acknowledged it.
type Tracker struct {
	mu     sync.Mutex
	seen   map[key]time.Time
	forget time.Duration
}

type key struct {
	port uint16
	seq  uint32
	host string
}

type Verdict int

const (
	First Verdict = iota
	Again
)

func New(forget time.Duration) *Tracker {
	return &Tracker{seen: make(map[key]time.Time), forget: forget}
}

func (t *Tracker) Saw(host string, port uint16, seq uint32, now time.Time) Verdict {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sweep(now)

	k := key{port: port, seq: seq, host: host}

	if _, repeat := t.seen[k]; repeat {
		t.seen[k] = now

		return Again
	}

	t.seen[k] = now

	return First
}

func (t *Tracker) sweep(now time.Time) {
	for k, at := range t.seen {
		if now.Sub(at) > t.forget {
			delete(t.seen, k)
		}
	}
}

// The map has to stay small over a long run, and this is how a test can see that.
func (t *Tracker) Watching() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.seen)
}
