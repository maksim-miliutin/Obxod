package logbook

import (
	"testing"
)

func TestBookKeepsOnlyTheLastLines(t *testing.T) {
	b := New(3)

	for _, line := range []string{"a", "b", "c", "d", "e"} {
		b.Add(line)
	}

	if got := b.Text(); got != "c\nd\ne" {
		t.Fatalf("Text = %q, want the last three", got)
	}
}

func TestBookIsSafeUnderConcurrentAdd(t *testing.T) {
	b := New(100)

	done := make(chan struct{})
	go func() {
		for range 500 {
			b.Add("line")
		}
		close(done)
	}()

	for range 500 {
		_ = b.Text()
	}
	<-done
}

func TestWorthDropsTheNoisyCounters(t *testing.T) {
	for _, line := range []string{
		"  replies watched: 16201 so far",
		"  rr4---sn-q4fl6ndl.googlevideo.com: asking again",
		"  quic dropped: 151 so far",
	} {
		if Worth(line) {
			t.Errorf("kept noise: %q", line)
		}
	}
}

func TestWorthKeepsRealEvents(t *testing.T) {
	for _, line := range []string{
		"  discord.com on port 61410: the server answered",
		"  i9.ytimg.com: name swapped for yqkl.mail.ru (ttl 4)",
		"  voice: 5 recorded datagrams sent first",
	} {
		if !Worth(line) {
			t.Errorf("dropped a real event: %q", line)
		}
	}
}

func TestClearEmptiesTheBook(t *testing.T) {
	b := New(200)
	b.Add("one")
	b.Add("two")
	b.Clear()

	if b.Text() != "" {
		t.Errorf("after Clear the book still holds %q", b.Text())
	}
}
