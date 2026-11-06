package bypass

import (
	"bytes"
	"testing"

	"obxod/internal/divert"
)

// With rnd the recorded decoy is freshened before each copy, so an inspector cannot
// recognise it by its bytes the way it can a decoy sent identical every time.
func TestRandomizedFakeDiffersEachCopy(t *testing.T) {
	const host = "discord.com"

	recorded := clientHello("www.google.com")
	r := &recorder{}

	e := New(Settings{Rules: setOf(t, host+"=fake,repeats:2,rnd"), Recorded: recorded, Wet: true})

	if _, err := e.forward(r, packet443(clientHello(host)), &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	if len(r.sent) != 2 {
		t.Fatalf("sent %d copies, want 2", len(r.sent))
	}

	if bytes.Equal(r.sent[0], r.sent[1]) {
		t.Error("two randomized decoys are identical")
	}
}
