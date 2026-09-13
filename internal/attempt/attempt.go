package attempt

import (
	"sync"
	"time"
)

// A client retransmits a hello only when nothing acknowledged it, so a repeat is
// the plainest sign the bypass did not work, visible without watching for replies.
type Tracker struct {
	mu     sync.Mutex
	seen   map[key]time.Time
	byPort map[uint16]string
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
	return &Tracker{seen: make(map[key]time.Time), byPort: make(map[uint16]string), forget: forget}
}

func (t *Tracker) Saw(host string, port uint16, seq uint32, now time.Time) Verdict {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.sweep(now)

	t.byPort[port] = host

	k := key{port: port, seq: seq, host: host}

	if _, repeat := t.seen[k]; repeat {
		t.seen[k] = now

		return Again
	}

	t.seen[k] = now

	return First
}

// sweep drops what is too old to be a retransmission, so a long run does not
// grow a map of every connection the machine ever made.
func (t *Tracker) sweep(now time.Time) {
	for k, at := range t.seen {
		if now.Sub(at) > t.forget {
			delete(t.seen, k)
			delete(t.byPort, k.port)
		}
	}
}

func (t *Tracker) Watching() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.seen)
}

// HostOn names the site a local port was used for, so a reply arriving on that
// port can be blamed on the right site. Called from the reply watcher, which
// runs on its own, hence the lock.
func (t *Tracker) HostOn(port uint16) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	host, ok := t.byPort[port]

	return host, ok
}
