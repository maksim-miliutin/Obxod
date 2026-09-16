package link

import (
	"sync"
	"time"
)

type Health struct {
	mu    sync.Mutex
	links map[uint16]*state
}

type state struct {
	host     string
	began    time.Time
	packets  int
	bytes    int
	lastData time.Time
	reported bool
	closed   bool
}

type Report struct {
	Host    string
	Port    uint16
	Packets int
	Bytes   int
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

func (h *Health) Data(port uint16, bytes int, now time.Time) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	s, known := h.links[port]
	if !known {
		return "", false
	}

	s.packets++
	s.bytes += bytes
	s.lastData = now

	return s.host, s.packets == 1
}

// A finished request goes quiet exactly like a killed one; only a fin tells them apart.
func (h *Health) Closed(port uint16) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if s, known := h.links[port]; known {
		s.closed = true
	}
}

// Reset drops the link and says whose it was: a connection the other side reset
// must not also be reported as one that quietly died.
func (h *Health) Reset(port uint16) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	s, known := h.links[port]
	if !known {
		return "", false
	}

	delete(h.links, port)

	return s.host, true
}

func (h *Health) WentQuiet(now time.Time, after time.Duration) []Report {
	h.mu.Lock()
	defer h.mu.Unlock()

	var out []Report

	for port, s := range h.links {
		if s.packets == 0 || s.reported || s.closed || now.Sub(s.lastData) < after {
			continue
		}

		s.reported = true

		out = append(out, Report{
			Host:    s.host,
			Port:    port,
			Packets: s.packets,
			Bytes:   s.bytes,
			Silence: now.Sub(s.lastData),
		})
	}

	return out
}
