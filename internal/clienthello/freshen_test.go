package clienthello

import (
	"bytes"
	"testing"
)

func TestFreshenedChangesTheRandomOnly(t *testing.T) {
	in := build("ya.ru")
	keep := make([]byte, len(in))
	copy(keep, in)

	out := Freshened(in)

	if len(out) != len(in) {
		t.Fatalf("length changed: %d vs %d", len(out), len(in))
	}

	if !bytes.Equal(in, keep) {
		t.Error("Freshened mutated its input")
	}

	at := recordHeaderLen + handshakeHeaderLen + clientRandomAt
	if bytes.Equal(out[at:at+clientRandomLen], keep[at:at+clientRandomLen]) {
		t.Error("the random field was not freshened")
	}

	if !bytes.Contains(out, []byte("ya.ru")) {
		t.Error("Freshened dropped the server name")
	}

	if bytes.Equal(out, Freshened(in)) {
		t.Error("two freshenings produced the same bytes")
	}
}
