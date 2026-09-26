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
