package logbook

import (
	"strings"
	"sync"
)

type Book struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func New(max int) *Book {
	return &Book{max: max}
}

func (b *Book) Add(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = append(b.lines, line)

	// Keep only the last max lines: the engine's log runs to thousands.
	if len(b.lines) > b.max {
		keep := make([]string, b.max)
		copy(keep, b.lines[len(b.lines)-b.max:])
		b.lines = keep
	}
}

func (b *Book) Text() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return strings.Join(b.lines, "\n")
}

// Worth drops the engine's high-frequency counter and retry lines, which
// otherwise bury the useful events under thousands of near-identical entries.
func Worth(line string) bool {
	for _, noise := range []string{"replies watched", "asking again", "quic dropped"} {
		if strings.Contains(line, noise) {
			return false
		}
	}

	return true
}
