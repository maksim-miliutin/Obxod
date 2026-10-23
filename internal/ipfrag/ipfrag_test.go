package ipfrag

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sort"
	"testing"

	"obxod/internal/checksum"
	"obxod/internal/ip"
)

// flags is the 16-bit field at bytes 6-7: don't-fragment, more-fragments, offset.
func packetWith(id uint16, flags uint16, payload []byte) []byte {
	packet := make([]byte, 20)
	packet[0] = 4<<4 | 5
	binary.BigEndian.PutUint16(packet[4:6], id)
	binary.BigEndian.PutUint16(packet[6:8], flags)
	packet[8] = 64
	packet[9] = ip.ProtocolTCP
	copy(packet[12:16], []byte{192, 168, 1, 2})
	copy(packet[16:20], []byte{93, 184, 216, 34})
	packet = append(packet, payload...)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))

	return packet
}

func field(t *testing.T, fragment []byte) uint16 {
	t.Helper()

	return binary.BigEndian.Uint16(fragment[6:8])
}

// reassemble is what the receiver does: order the pieces by offset and lay their
// payloads down, which is how a fragmented datagram becomes whole again.
func reassemble(t *testing.T, fragments ...[]byte) []byte {
	t.Helper()

	type piece struct {
		at   int
		data []byte
	}

	ordered := make([]piece, 0, len(fragments))

	for _, fragment := range fragments {
		outer, err := ip.Parse(fragment)
		if err != nil {
			t.Fatalf("ip.Parse: %v", err)
		}

		ordered = append(ordered, piece{int(field(t, fragment)&offsetMask) * 8, outer.Payload})
	}

	sort.Slice(ordered, func(i, j int) bool { return ordered[i].at < ordered[j].at })

	var out []byte
	for _, p := range ordered {
		out = append(out, p.data...)
	}

	return out
}

func TestAtReassemblesToThePayload(t *testing.T) {
	payload := bytes.Repeat([]byte{0xab}, 64)
	packet := packetWith(0x1234, 0, payload)

	first, second, err := At(packet, 24)
	if err != nil {
		t.Fatalf("At: %v", err)
	}

	outer, err := ip.Parse(first)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	if got := len(outer.Payload); got != 24 {
		t.Errorf("first fragment carries %d bytes, want 24", got)
	}

	// Out of order on purpose: the receiver uses the offsets, not arrival order.
	if joined := reassemble(t, second, first); !bytes.Equal(joined, payload) {
		t.Error("the fragments do not reassemble to the payload")
	}
}

func TestAtSetsTheFragmentFields(t *testing.T) {
	payload := bytes.Repeat([]byte{0xcd}, 80)

	first, second, err := At(packetWith(0x1234, 0, payload), 40)
	if err != nil {
		t.Fatalf("At: %v", err)
	}

	if field(t, first)&flagMore == 0 {
		t.Error("the first fragment does not say more follow")
	}

	if field(t, first)&offsetMask != 0 {
		t.Error("the first fragment does not start at offset 0")
	}

	if field(t, second)&flagMore != 0 {
		t.Error("the last fragment still says more follow")
	}

	if got := field(t, second) & offsetMask; got != 40/8 {
		t.Errorf("the last fragment offset is %d eight-byte units, want %d", got, 40/8)
	}
}

// Both fragments share the original id, or the receiver cannot tell they belong to
// the same datagram, and both carry a header checksum that checks out.
func TestAtKeepsTheIDAndSealsTheHeader(t *testing.T) {
	first, second, err := At(packetWith(0xbeef, 0, bytes.Repeat([]byte{1}, 48)), 16)
	if err != nil {
		t.Fatalf("At: %v", err)
	}

	for _, fragment := range [][]byte{first, second} {
		if id := binary.BigEndian.Uint16(fragment[4:6]); id != 0xbeef {
			t.Errorf("fragment id = %#x, want the original 0xbeef", id)
		}

		if got := checksum.Of(fragment[:20]); got != 0 {
			t.Errorf("header checksum checks out as %#04x, want 0", got)
		}

		outer, err := ip.Parse(fragment)
		if err != nil {
			t.Fatalf("ip.Parse: %v", err)
		}

		if outer.TotalLen != len(fragment) {
			t.Errorf("total length says %d, fragment is %d bytes", outer.TotalLen, len(fragment))
		}
	}
}

// A packet marked don't-fragment cannot be fragmented by a router; we are doing it
// ourselves, so the bit has to come off both pieces.
func TestAtClearsDontFragment(t *testing.T) {
	const dontFragment = 0x4000

	first, second, err := At(packetWith(0x1234, dontFragment, bytes.Repeat([]byte{7}, 32)), 8)
	if err != nil {
		t.Fatalf("At: %v", err)
	}

	for _, fragment := range [][]byte{first, second} {
		if field(t, fragment)&dontFragment != 0 {
			t.Error("the don't-fragment bit survived into a fragment")
		}
	}
}

func TestAtLeavesTheOriginalAlone(t *testing.T) {
	packet := packetWith(0x1234, 0, bytes.Repeat([]byte{9}, 40))
	before := append([]byte(nil), packet...)

	if _, _, err := At(packet, 16); err != nil {
		t.Fatalf("At: %v", err)
	}

	if !bytes.Equal(packet, before) {
		t.Error("At wrote into the packet it was given")
	}
}

func TestAtErrors(t *testing.T) {
	payload := bytes.Repeat([]byte{0xab}, 64)

	cases := []struct {
		name   string
		packet []byte
		point  int
		want   error
	}{
		{"not ipv4", append([]byte{0x60}, make([]byte, 39)...), 8, ip.ErrNotIPv4},
		{"no payload", packetWith(1, 0, nil), 8, ErrNoPayload},
		{"at the start", packetWith(1, 0, payload), 0, ErrBadPoint},
		{"negative", packetWith(1, 0, payload), -8, ErrBadPoint},
		{"at the end", packetWith(1, 0, payload), len(payload), ErrBadPoint},
		{"past the end", packetWith(1, 0, payload), len(payload) + 8, ErrBadPoint},
		{"not a multiple of eight", packetWith(1, 0, payload), 20, ErrNotAligned},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := At(c.packet, c.point); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}
