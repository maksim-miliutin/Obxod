package quic

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestProtectReversesUnprotect(t *testing.T) {
	prefix, err := hex.DecodeString("c000000001088394c8f03e5157080000449e7b9aec34d1b1c98dd7689fb8ec11d242b123dc9b")
	if err != nil {
		t.Fatal(err)
	}

	datagram := make([]byte, 1200)
	copy(datagram, prefix)
	original := append([]byte(nil), datagram...)

	head, err := ParseInitial(datagram)
	if err != nil {
		t.Fatal(err)
	}

	keys := ClientInitialKeys(head.DCID)

	if _, err := Unprotect(datagram, head.PNOffset, keys.HP); err != nil {
		t.Fatal(err)
	}

	if datagram[0] == original[0] {
		t.Fatal("unprotect left the first byte unchanged")
	}

	if err := Protect(datagram, head.PNOffset, keys.HP); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(datagram, original) {
		t.Error("protect did not restore the protected header")
	}
}
