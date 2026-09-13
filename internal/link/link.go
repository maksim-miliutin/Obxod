package link

import (
	"sync"
	"time"
)

// Health follows single connections rather than sites: a site can have one
// connection answering and the next one dying, and a per-site view hides that.
type Health struct {
	mu    sync.Mutex
	links map[uint16]*state
}

type state struct {
	host     string
	began    time.Time
	packets  int
	lastData time.Time
	reported bool
}

type Report struct {
	Host    string
	Port    uint16
	Packets int
	Silence time.Duration
}

func New() *Health {
	return &Health{links: make(map[uint16]*state)}
}

func (h *Health) Hello(host string, port uint16, now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.links[port] = &state{host: host, began: now}
}

// Data reports whether this is the first answer on the connection, so the caller
// can say so once instead of on every packet.
func (h *Health) Data(port uint16, now time.Time) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	s, known := h.links[port]
	if !known {
		return "", false
	}

	s.packets++
	s.lastData = now

	return s.host, s.packets == 1
}

func (h *Health) Forget(port uint16) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.links, port)
}

// WentQuiet names connections that answered and then stopped, which is what a
// killed connection looks like when nothing bothers to send a reset.
func (h *Health) WentQuiet(now time.Time, after time.Duration) []Report {
	h.mu.Lock()
	defer h.mu.Unlock()

	var out []Report

	for port, s := range h.links {
		if s.packets == 0 || s.reported || now.Sub(s.lastData) < after {
			continue
		}

		s.reported = true

		out = append(out, Report{
			Host:    s.host,
			Port:    port,
			Packets: s.packets,
			Silence: now.Sub(s.lastData),
		})
	}

	return out
}
