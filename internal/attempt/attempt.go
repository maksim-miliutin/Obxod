package attempt

import "time"

// A client retransmits a hello only when nothing acknowledged it, so a repeat is
// the plainest sign the bypass did not work, visible without watching for replies.
type Tracker struct {
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
	t.sweep(now)

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
		}
	}
}

func (t *Tracker) Watching() int {
	return len(t.seen)
}
