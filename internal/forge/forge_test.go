package forge

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"obxod/internal/checksum"
	"obxod/internal/ip"
	"obxod/internal/tcp"
)

func build(protocol byte, ttl byte, payload []byte, trailing int) []byte {
	transport := make([]byte, 20)
	binary.BigEndian.PutUint16(transport[0:2], 54321)
	binary.BigEndian.PutUint16(transport[2:4], 443)
	binary.BigEndian.PutUint32(transport[4:8], 1000)
	transport[12] = 5 << 4
	transport[13] = 0x18

	if protocol == ip.ProtocolUDP {
		transport = make([]byte, 8)
		binary.BigEndian.PutUint16(transport[2:4], 50021)
		binary.BigEndian.PutUint16(transport[4:6], uint16(8+len(payload)))
	}

	transport = append(transport, payload...)

	packet := make([]byte, 20)
	packet[0] = 4<<4 | 5
	packet[8] = ttl
	packet[9] = protocol
	copy(packet[12:16], []byte{192, 168, 1, 2})
	copy(packet[16:20], []byte{93, 184, 216, 34})
	packet = append(packet, transport...)

	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	packet = append(packet, bytes.Repeat([]byte{0xee}, trailing)...)

	return packet
}

func verify(t *testing.T, packet []byte) {
	t.Helper()

	outer, err := ip.Parse(packet)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	if got := checksum.Of(packet[:outer.HeaderLen]); got != 0 {
		t.Errorf("ip checksum checks out as %#04x, want 0", got)
	}

	segment := packet[outer.HeaderLen : outer.HeaderLen+len(outer.Payload)]

	// checksum.TCP reads the field as zero, so it recomputes rather than verifies:
	// compare what stands in the packet against what it should be.
	want := checksum.TCP(outer.Src, outer.Dst, segment)

	if got := binary.BigEndian.Uint16(segment[tcpChecksumAt : tcpChecksumAt+2]); got != want {
		t.Errorf("tcp checksum = %#04x, want %#04x", got, want)
	}
}

func TestCopyChecksums(t *testing.T) {
	for _, size := range []int{1, 5, 40, 517, 1400} {
		packet := build(ip.ProtocolTCP, 64, bytes.Repeat([]byte{0xab}, size), 0)

		copied, err := Copy(packet, Recipe{TTL: 4})
		if err != nil {
			t.Fatalf("Copy: %v", err)
		}

		verify(t, copied)
	}
}

func TestCopySetsTTL(t *testing.T) {
	packet := build(ip.ProtocolTCP, 64, []byte("hello"), 0)

	copied, err := Copy(packet, Recipe{TTL: 3})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if copied[ttlAt] != 3 {
		t.Errorf("ttl = %d, want 3", copied[ttlAt])
	}
}

func TestCopyKeepsTTLWhenNoneAsked(t *testing.T) {
	packet := build(ip.ProtocolTCP, 64, []byte("hello"), 0)

	copied, err := Copy(packet, Recipe{})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if copied[ttlAt] != 64 {
		t.Errorf("ttl = %d, want the original 64", copied[ttlAt])
	}

	verify(t, copied)
}

func TestCopyLeavesTheOriginalAlone(t *testing.T) {
	packet := build(ip.ProtocolTCP, 64, []byte("hello"), 0)
	before := append([]byte(nil), packet...)

	copied, err := Copy(packet, Recipe{TTL: 1})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if !bytes.Equal(packet, before) {
		t.Error("Copy wrote into the packet it was given")
	}

	copied[0] = 0xff

	if packet[0] == 0xff {
		t.Error("the copy shares memory with the original")
	}
}

func TestCopyKeepsTheBytesItDoesNotTouch(t *testing.T) {
	payload := []byte("hello there")
	packet := build(ip.ProtocolTCP, 64, payload, 0)

	copied, err := Copy(packet, Recipe{TTL: 4})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if !bytes.Equal(copied[40:], payload) {
		t.Errorf("payload = %q, want %q", copied[40:], payload)
	}

	if !bytes.Equal(copied[12:20], packet[12:20]) {
		t.Error("the addresses changed")
	}

	if !bytes.Equal(copied[20:24], packet[20:24]) {
		t.Error("the ports changed")
	}
}

func TestCopyIgnoresTrailingBytes(t *testing.T) {
	payload := bytes.Repeat([]byte{0xcd}, 100)

	clean := build(ip.ProtocolTCP, 64, payload, 0)
	padded := build(ip.ProtocolTCP, 64, payload, 16)

	fromClean, err := Copy(clean, Recipe{TTL: 4})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	fromPadded, err := Copy(padded, Recipe{TTL: 4})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if !bytes.Equal(fromClean[:len(clean)], fromPadded[:len(clean)]) {
		t.Error("trailing bytes from offload leaked into the checksums")
	}
}

func TestCopyErrors(t *testing.T) {
	cases := []struct {
		name   string
		packet []byte
		want   error
	}{
		{"empty", nil, ip.ErrTooShort},
		{"version six", append([]byte{6 << 4}, bytes.Repeat([]byte{0}, 40)...), ip.ErrNotIPv4},
		{"udp for now", build(ip.ProtocolUDP, 64, []byte("voice"), 0), ErrNotTCP},
		{"tcp header cut short", build(ip.ProtocolTCP, 64, nil, 0)[:32], tcp.ErrTooShort},
		{"bare ack", build(ip.ProtocolTCP, 64, nil, 0), ErrNoPayload},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Copy(c.packet, Recipe{TTL: 4}); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestCopyFixesAChecksumOffloadLeftWrong(t *testing.T) {
	packet := build(ip.ProtocolTCP, 64, []byte("hello"), 0)

	binary.BigEndian.PutUint16(packet[ipChecksumAt:ipChecksumAt+2], 0x1234)
	binary.BigEndian.PutUint16(packet[20+tcpChecksumAt:20+tcpChecksumAt+2], 0x5678)

	copied, err := Copy(packet, Recipe{TTL: 4})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	verify(t, copied)
}
