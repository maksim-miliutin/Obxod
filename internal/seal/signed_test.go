package seal

import (
	"bytes"
	"encoding/binary"
	"testing"

	"obxod/internal/checksum"
	"obxod/internal/ip"
	"obxod/internal/tcp"
)

func sound(t *testing.T, packet []byte) {
	t.Helper()

	outer, err := ip.Parse(packet)
	if err != nil {
		t.Fatalf("the signed packet does not parse: %v", err)
	}

	if int(binary.BigEndian.Uint16(packet[2:4])) != len(packet) {
		t.Errorf("ip length says %d, the packet is %d",
			binary.BigEndian.Uint16(packet[2:4]), len(packet))
	}

	if got := binary.BigEndian.Uint16(packet[10:12]); got != checksum.IPv4(packet[:outer.HeaderLen]) {
		t.Error("the ip checksum was not counted again")
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		t.Fatalf("the signed segment does not parse: %v", err)
	}

	if segment.HeaderLen%4 != 0 {
		t.Errorf("the header is %d bytes, which is not whole words", segment.HeaderLen)
	}

	var src, dst [4]byte

	copy(src[:], packet[12:16])
	copy(dst[:], packet[16:20])

	if got := binary.BigEndian.Uint16(outer.Payload[16:18]); got != checksum.TCP(src, dst, outer.Payload) {
		t.Error("the tcp checksum was not counted again")
	}
}

func TestSignedStaysSound(t *testing.T) {
	made, err := Signed(packetWith([]byte("a client hello"), 0))
	if err != nil {
		t.Fatalf("Signed: %v", err)
	}

	sound(t, made)
}

func TestSignedCarriesTheOption(t *testing.T) {
	packet := packetWith([]byte("a client hello"), 0)

	made, err := Signed(packet)
	if err != nil {
		t.Fatalf("Signed: %v", err)
	}

	outer, _ := ip.Parse(made)

	options := outer.Payload[20 : 20+20]
	if options[0] != signatureKind || options[1] != signatureLen {
		t.Errorf("the option is kind %d length %d, want %d and %d",
			options[0], options[1], signatureKind, signatureLen)
	}

	// Padded to a whole word, or everything after it is read at the wrong offset.
	if options[18] != optionNop || options[19] != optionNop {
		t.Error("the option was not padded out to a whole word")
	}
}

func TestSignedKeepsThePayload(t *testing.T) {
	want := []byte("a client hello that must arrive whole")

	made, err := Signed(packetWith(want, 0))
	if err != nil {
		t.Fatalf("Signed: %v", err)
	}

	outer, _ := ip.Parse(made)

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		t.Fatalf("tcp.Parse: %v", err)
	}

	if !bytes.Equal(segment.Payload, want) {
		t.Errorf("the payload came out as %q", segment.Payload)
	}
}

func TestEverySignatureIsDifferent(t *testing.T) {
	seen := map[string]bool{}

	for range 20 {
		made, err := Signed(packetWith([]byte("x"), 0))
		if err != nil {
			t.Fatalf("Signed: %v", err)
		}

		seen[string(made[40:58])] = true
	}

	if len(seen) < 2 {
		t.Error("twenty signatures came out the same")
	}
}

// The data offset counts words in four bits, so sixty bytes is the whole header a
// segment can ever have, and twenty of them cannot always be found.
func TestSignedRefusesAFullHeader(t *testing.T) {
	crowded := bytes.Repeat([]byte{optionNop}, 24)

	if _, err := Signed(withOptions(crowded)); err != ErrNoRoomToSign {
		t.Errorf("Signed on a header with no room gave %v, want ErrNoRoomToSign", err)
	}
}

func TestSignedTakesAHeaderWithRoom(t *testing.T) {
	made, err := Signed(withOptions(timestamps(5000, 77)))
	if err != nil {
		t.Fatalf("Signed: %v", err)
	}

	sound(t, made)
}
