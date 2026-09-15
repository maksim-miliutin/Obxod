package seal

import (
	"bytes"
	"encoding/binary"
	"testing"

	"obxod/internal/checksum"
	"obxod/internal/ip"
	"obxod/internal/tcp"
)

func packetWith(payload []byte, trailing int) []byte {
	segment := make([]byte, 20)
	binary.BigEndian.PutUint16(segment[0:2], 51000)
	binary.BigEndian.PutUint16(segment[2:4], 443)
	binary.BigEndian.PutUint32(segment[4:8], 1000)
	segment[12] = 5 << 4
	segment[13] = 0x18
	segment = append(segment, payload...)

	out := make([]byte, 20)
	out[0] = 4<<4 | 5
	out[8] = 64
	out[9] = ip.ProtocolTCP
	copy(out[12:16], []byte{192, 168, 1, 2})
	copy(out[16:20], []byte{93, 184, 216, 34})
	out = append(out, segment...)
	binary.BigEndian.PutUint16(out[2:4], uint16(len(out)))

	// Offload hands the driver more bytes than the header claims.
	return append(out, bytes.Repeat([]byte{0xee}, trailing)...)
}

// A sealed header sums to zero when the checksum field is counted in. Goes
// through Of rather than the functions under test, so it is not circular.
func sumsAgree(t *testing.T, packet []byte) (bool, bool) {
	t.Helper()

	outer, err := ip.Parse(packet)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	segment := packet[outer.HeaderLen : outer.HeaderLen+len(outer.Payload)]

	pseudo := make([]byte, 12)
	copy(pseudo[0:4], outer.Src[:])
	copy(pseudo[4:8], outer.Dst[:])
	pseudo[9] = ip.ProtocolTCP
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(segment)))

	return checksum.Of(packet[:outer.HeaderLen]) == 0, checksum.Of(append(pseudo, segment...)) == 0
}

func TestSumsSealsBothHeaders(t *testing.T) {
	packet := packetWith([]byte("hello there"), 0)
	packet[8] = 4

	if err := Sums(packet, false); err != nil {
		t.Fatalf("Sums: %v", err)
	}

	outerOK, segmentOK := sumsAgree(t, packet)
	if !outerOK || !segmentOK {
		t.Errorf("after sealing: ip sum right = %v, tcp sum right = %v", outerOK, segmentOK)
	}
}

// The trap this guards: trailing bytes from offload belong to no segment, and
// summing them leaves a checksum the server throws away.
func TestSumsIgnoresWhatOffloadAddsPastTheHeader(t *testing.T) {
	packet := packetWith([]byte("hello there"), 12)

	if err := Sums(packet, false); err != nil {
		t.Fatalf("Sums: %v", err)
	}

	if _, segmentOK := sumsAgree(t, packet); !segmentOK {
		t.Error("the tcp sum covered bytes past the claimed length")
	}
}

// badsum must leave a wrong sum even where offload left the field blank, so it
// flips the right answer instead of skipping the work.
func TestBadSumLeavesAWrongOne(t *testing.T) {
	for _, blank := range []bool{false, true} {
		packet := packetWith([]byte("hello there"), 0)

		if !blank {
			binary.BigEndian.PutUint16(packet[20+16:20+18], 0x1234)
		}

		if err := Sums(packet, true); err != nil {
			t.Fatalf("Sums: %v", err)
		}

		if _, segmentOK := sumsAgree(t, packet); segmentOK {
			t.Errorf("badsum with a blank field = %v left a sum the server accepts", blank)
		}
	}
}

func TestRemadeCarriesTheNewPayload(t *testing.T) {
	packet := packetWith([]byte("the original hello"), 0)

	out, err := Remade(packet, []byte("a recorded one"), 5000)
	if err != nil {
		t.Fatalf("Remade: %v", err)
	}

	outer, err := ip.Parse(out)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		t.Fatalf("tcp.Parse: %v", err)
	}

	if string(segment.Payload) != "a recorded one" {
		t.Errorf("payload = %q, want the recorded one", segment.Payload)
	}

	if segment.Seq != 5000 {
		t.Errorf("seq = %d, want 5000", segment.Seq)
	}

	if segment.DstPort != 443 {
		t.Errorf("dst port = %d, want the original 443", segment.DstPort)
	}
}

// The trap this guards: a payload of a different length leaves the ip total
// length claiming the old one, and everything downstream reads the wrong bytes.
func TestRemadeFixesTheLength(t *testing.T) {
	packet := packetWith([]byte("the original hello"), 0)

	for _, payload := range [][]byte{nil, []byte("x"), bytes.Repeat([]byte{0xaa}, 700)} {
		out, err := Remade(packet, payload, 1000)
		if err != nil {
			t.Fatalf("Remade: %v", err)
		}

		if got := binary.BigEndian.Uint16(out[2:4]); int(got) != len(out) {
			t.Errorf("total length says %d, the packet is %d bytes", got, len(out))
		}

		outerOK, segmentOK := sumsAgree(t, out)
		if !outerOK || !segmentOK {
			t.Errorf("payload of %d bytes: ip sum right = %v, tcp sum right = %v", len(payload), outerOK, segmentOK)
		}
	}
}

func TestRemadeDropsWhatOffloadAdded(t *testing.T) {
	packet := packetWith([]byte("the original hello"), 12)

	out, err := Remade(packet, []byte("short"), 1000)
	if err != nil {
		t.Fatalf("Remade: %v", err)
	}

	if len(out) != 20+20+len("short") {
		t.Errorf("the remade packet is %d bytes, want headers plus the payload alone", len(out))
	}
}

func TestOnlyTCPIsSealed(t *testing.T) {
	packet := packetWith([]byte("hello"), 0)
	packet[9] = ip.ProtocolUDP

	if err := Sums(packet, false); err != ErrNotTCP {
		t.Errorf("Sums on udp gave %v, want ErrNotTCP", err)
	}

	if _, err := Remade(packet, []byte("x"), 1); err != ErrNotTCP {
		t.Errorf("Remade on udp gave %v, want ErrNotTCP", err)
	}
}

func numbers(t *testing.T, packet []byte) (uint32, uint32) {
	t.Helper()

	outer, err := ip.Parse(packet)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	segment := packet[outer.HeaderLen:]

	return binary.BigEndian.Uint32(segment[4:8]), binary.BigEndian.Uint32(segment[8:12])
}

func TestShiftMovesEachNumberOnItsOwn(t *testing.T) {
	packet := packetWith([]byte("hello"), 0)
	binary.BigEndian.PutUint32(packet[20+8:20+12], 70000)

	if err := Shift(packet, 0, 0); err != nil {
		t.Fatalf("Shift: %v", err)
	}

	if seq, ack := numbers(t, packet); seq != 1000 || ack != 70000 {
		t.Errorf("asking for no shift moved them to %d and %d", seq, ack)
	}

	if err := Shift(packet, 2, 0); err != nil {
		t.Fatalf("Shift: %v", err)
	}

	if seq, ack := numbers(t, packet); seq != 1002 || ack != 70000 {
		t.Errorf("a seq shift gave %d and %d, want 1002 and 70000", seq, ack)
	}
}

// Backwards is the useful direction: an old acknowledgement is ignored, while a
// future one makes the server answer instead of staying quiet.
func TestShiftTakesTheAckBackwards(t *testing.T) {
	packet := packetWith([]byte("hello"), 0)
	binary.BigEndian.PutUint32(packet[20+8:20+12], 70000)

	if err := Shift(packet, 0, -66000); err != nil {
		t.Fatalf("Shift: %v", err)
	}

	if _, ack := numbers(t, packet); ack != 4000 {
		t.Errorf("ack = %d, want 4000", ack)
	}
}

// The trap this guards: tcp numbers wrap, and a shift past zero must wrap with
// them rather than clamp.
func TestShiftWrapsLikeTCPDoes(t *testing.T) {
	packet := packetWith([]byte("hello"), 0)
	binary.BigEndian.PutUint32(packet[20+8:20+12], 100)

	if err := Shift(packet, 0, -200); err != nil {
		t.Fatalf("Shift: %v", err)
	}

	if _, ack := numbers(t, packet); ack != 0xffffff9c {
		t.Errorf("ack = %d, want it wrapped to 4294967196", ack)
	}
}
