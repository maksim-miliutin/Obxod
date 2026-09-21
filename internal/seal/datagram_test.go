package seal

import (
	"bytes"
	"encoding/binary"
	"testing"

	"obxod/internal/checksum"
	"obxod/internal/ip"
	"obxod/internal/udp"
)

func datagramTo(port uint16, payload []byte) []byte {
	out := make([]byte, 20)
	out[0] = 4<<4 | 5
	out[8] = 64
	out[9] = ip.ProtocolUDP
	copy(out[12:16], []byte{192, 168, 1, 2})
	copy(out[16:20], []byte{162, 159, 130, 234})

	body := make([]byte, 8)
	binary.BigEndian.PutUint16(body[0:2], 51000)
	binary.BigEndian.PutUint16(body[2:4], port)
	binary.BigEndian.PutUint16(body[4:6], uint16(8+len(payload)))
	body = append(body, payload...)

	out = append(out, body...)
	binary.BigEndian.PutUint16(out[2:4], uint16(len(out)))
	binary.BigEndian.PutUint16(out[10:12], checksum.IPv4(out[:20]))

	return out
}

func checked(t *testing.T, packet []byte) {
	t.Helper()

	outer, err := ip.Parse(packet)
	if err != nil {
		t.Fatalf("the made packet does not parse: %v", err)
	}

	if int(binary.BigEndian.Uint16(packet[2:4])) != len(packet) {
		t.Errorf("ip length says %d, the packet is %d",
			binary.BigEndian.Uint16(packet[2:4]), len(packet))
	}

	// The sum skips its own field, so the honest check is to count it again and
	// compare with what is written there.
	if got, want := binary.BigEndian.Uint16(packet[10:12]), checksum.IPv4(packet[:outer.HeaderLen]); got != want {
		t.Errorf("ip checksum is %#04x, counted again it is %#04x", got, want)
	}

	datagram, err := udp.Parse(outer.Payload)
	if err != nil {
		t.Fatalf("the made datagram does not parse: %v", err)
	}

	if datagram.Length != len(outer.Payload) {
		t.Errorf("udp length says %d, the datagram is %d", datagram.Length, len(outer.Payload))
	}

	var src, dst [4]byte

	copy(src[:], packet[12:16])
	copy(dst[:], packet[16:20])

	if got, want := binary.BigEndian.Uint16(outer.Payload[6:8]), checksum.UDP(src, dst, outer.Payload); got != want {
		t.Errorf("udp checksum is %#04x, counted again it is %#04x", got, want)
	}
}

// Whatever comes out has to be a datagram the other side will accept: the two
// lengths true and both sums checking out.
func TestDatagramIsSealedWhole(t *testing.T) {
	for _, size := range []int{1, 8, 100, 1200, 1400} {
		packet := datagramTo(50001, bytes.Repeat([]byte{0xab}, 40))

		made, err := Datagram(packet, bytes.Repeat([]byte{0xcd}, size))
		if err != nil {
			t.Fatalf("Datagram: %v", err)
		}

		if len(made) != 20+8+size {
			t.Fatalf("a %d byte payload made a %d byte packet", size, len(made))
		}

		checked(t, made)
	}
}

// The recorded datagram replaces the payload and nothing else: same addresses,
// same ports, so it reaches the same place.
func TestDatagramKeepsWhereItGoes(t *testing.T) {
	packet := datagramTo(50001, []byte("real"))

	made, err := Datagram(packet, []byte("a recorded discord datagram"))
	if err != nil {
		t.Fatalf("Datagram: %v", err)
	}

	if !bytes.Equal(made[12:20], packet[12:20]) {
		t.Error("the addresses changed")
	}

	if !bytes.Equal(made[20:24], packet[20:24]) {
		t.Error("the ports changed")
	}

	if bytes.Contains(made, []byte("real")) {
		t.Error("the real payload is still in there")
	}
}

func TestDatagramRefusesWhatIsNotUDP(t *testing.T) {
	packet := datagramTo(50001, []byte("x"))
	packet[9] = ip.ProtocolTCP

	if _, err := Datagram(packet, []byte("y")); err != ErrNotUDP {
		t.Errorf("Datagram on a tcp packet gave %v, want ErrNotUDP", err)
	}
}

// The payload is what was recorded, byte for byte: a datagram is not reassembled
// from pieces, so anything lost here is lost on the wire.
func TestDatagramCarriesThePayloadUntouched(t *testing.T) {
	want := bytes.Repeat([]byte{0x00, 0xff, 0x7f, 0x80}, 300)

	made, err := Datagram(datagramTo(19300, []byte("real")), want)
	if err != nil {
		t.Fatalf("Datagram: %v", err)
	}

	if !bytes.Equal(made[28:], want) {
		t.Error("the payload came out different from what went in")
	}
}
