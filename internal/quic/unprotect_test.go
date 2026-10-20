package quic

import (
	"encoding/hex"
	"testing"
)

func TestUnprotectRevealsRFC9001PacketNumber(t *testing.T) {
	// RFC 9001 A.2: the protected client Initial, header through the first bytes of
	// the ciphertext sample (padded to full length so the header parser accepts it).
	prefix, err := hex.DecodeString("c000000001088394c8f03e5157080000449e7b9aec34d1b1c98dd7689fb8ec11d242b123dc9b")
	if err != nil {
		t.Fatal(err)
	}

	datagram := make([]byte, 1200)
	copy(datagram, prefix)

	head, err := ParseInitial(datagram)
	if err != nil {
		t.Fatal(err)
	}

	keys := ClientInitialKeys(head.DCID)

	got, err := Unprotect(datagram, head.PNOffset, keys.HP)
	if err != nil {
		t.Fatal(err)
	}

	if got.PNLength != 4 {
		t.Errorf("pn length = %d, want 4", got.PNLength)
	}
	if got.Number != 2 {
		t.Errorf("number = %d, want 2", got.Number)
	}
	if got.HeaderLen != 22 {
		t.Errorf("header len = %d, want 22", got.HeaderLen)
	}
	if datagram[0] != 0xc3 {
		t.Errorf("first byte = %#x, want 0xc3", datagram[0])
	}
	if pn := hex.EncodeToString(datagram[head.PNOffset:got.HeaderLen]); pn != "00000002" {
		t.Errorf("packet number bytes = %s, want 00000002", pn)
	}
}

func TestUnprotectRejectsShortSample(t *testing.T) {
	if _, err := Unprotect(make([]byte, 30), 18, make([]byte, 16)); err != ErrShort {
		t.Errorf("err = %v, want %v", err, ErrShort)
	}
}
