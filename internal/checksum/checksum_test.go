package checksum

import (
	"encoding/binary"
	"math/rand"
	"testing"
)

func TestOfMatchesRFC1071(t *testing.T) {
	// The worked example from RFC 1071, section 3.
	data := []byte{0x00, 0x01, 0xf2, 0x03, 0xf4, 0xf5, 0xf6, 0xf7}

	if got := Of(data); got != 0x220d {
		t.Errorf("Of = %#04x, want 0x220d", got)
	}
}

func TestIPv4MatchesAKnownHeader(t *testing.T) {
	header := []byte{
		0x45, 0x00, 0x00, 0x73, 0x00, 0x00, 0x40, 0x00,
		0x40, 0x11, 0xb8, 0x61, 0xc0, 0xa8, 0x00, 0x01,
		0xc0, 0xa8, 0x00, 0xc7,
	}

	if got := IPv4(header); got != 0xb861 {
		t.Errorf("IPv4 = %#04x, want 0xb861", got)
	}
}

func TestIPv4IgnoresWhateverSitsInTheField(t *testing.T) {
	header := []byte{
		0x45, 0x00, 0x00, 0x73, 0x00, 0x00, 0x40, 0x00,
		0x40, 0x11, 0x00, 0x00, 0xc0, 0xa8, 0x00, 0x01,
		0xc0, 0xa8, 0x00, 0xc7,
	}

	want := IPv4(header)

	for _, junk := range []uint16{0x0000, 0xffff, 0xb861, 0x1234} {
		binary.BigEndian.PutUint16(header[ipv4At:ipv4At+2], junk)

		if got := IPv4(header); got != want {
			t.Errorf("with %#04x in the field IPv4 = %#04x, want %#04x", junk, got, want)
		}
	}
}

func TestIPv4HeaderVerifiesToZero(t *testing.T) {
	header := []byte{
		0x45, 0x00, 0x00, 0x3c, 0x1c, 0x46, 0x40, 0x00,
		0x40, 0x06, 0x00, 0x00, 0xac, 0x10, 0x0a, 0x63,
		0xac, 0x10, 0x0a, 0x0c,
	}

	binary.BigEndian.PutUint16(header[ipv4At:ipv4At+2], IPv4(header))

	// A header carrying its own checksum sums to all ones, so the complement is zero.
	if got := Of(header); got != 0 {
		t.Errorf("filled header checks out as %#04x, want 0", got)
	}
}

func segment(payload []byte) []byte {
	s := make([]byte, 20+len(payload))

	binary.BigEndian.PutUint16(s[0:2], 54321)
	binary.BigEndian.PutUint16(s[2:4], 443)
	binary.BigEndian.PutUint32(s[4:8], 1000)
	s[12] = 5 << 4
	s[13] = 0x18
	copy(s[20:], payload)

	return s
}

func datagram(payload []byte) []byte {
	d := make([]byte, 8+len(payload))

	binary.BigEndian.PutUint16(d[0:2], 51234)
	binary.BigEndian.PutUint16(d[2:4], 50021)
	binary.BigEndian.PutUint16(d[4:6], uint16(8+len(payload)))
	copy(d[8:], payload)

	return d
}

func TestFilledSegmentsVerifyToZero(t *testing.T) {
	src := [4]byte{192, 168, 1, 2}
	dst := [4]byte{93, 184, 216, 34}

	random := rand.New(rand.NewSource(1071))

	for size := 0; size < 200; size++ {
		payload := make([]byte, size)
		random.Read(payload)

		s := segment(payload)
		binary.BigEndian.PutUint16(s[tcpAt:tcpAt+2], TCP(src, dst, s))

		if got := fold(pseudo(src, dst, protocolTCP, len(s)) + sum(s, skipNothing)); got != 0 {
			t.Fatalf("tcp payload of %d bytes checks out as %#04x, want 0", size, got)
		}

		d := datagram(payload)
		binary.BigEndian.PutUint16(d[udpAt:udpAt+2], UDP(src, dst, d))

		if got := fold(pseudo(src, dst, protocolUDP, len(d)) + sum(d, skipNothing)); got != 0 {
			t.Fatalf("udp payload of %d bytes checks out as %#04x, want 0", size, got)
		}
	}
}

func TestOddLengthKeepsTheLastByte(t *testing.T) {
	odd := []byte{0x12, 0x34, 0x56}
	even := []byte{0x12, 0x34, 0x56, 0x00}

	if Of(odd) != Of(even) {
		t.Errorf("Of(odd) = %#04x, Of(padded) = %#04x, want them equal", Of(odd), Of(even))
	}

	if Of(odd) == Of([]byte{0x12, 0x34}) {
		t.Error("the last byte of an odd length fell out of the sum")
	}
}

func TestUDPNeverReturnsZero(t *testing.T) {
	src := [4]byte{0, 0, 0, 0}
	dst := [4]byte{0, 0, 0, 0}

	found := false

	for candidate := 0; candidate <= 0xffff && !found; candidate++ {
		d := make([]byte, 8)
		binary.BigEndian.PutUint16(d[0:2], uint16(candidate))
		d[4] = 0x00
		d[5] = 0x08

		bare := fold(pseudo(src, dst, protocolUDP, len(d)) + sum(d, udpAt))
		if bare != 0 {
			continue
		}

		found = true

		if got := UDP(src, dst, d); got != 0xffff {
			t.Errorf("UDP = %#04x for a datagram that sums to zero, want 0xffff", got)
		}
	}

	if !found {
		t.Fatal("no datagram summing to zero was found, so the rule went untested")
	}
}

func TestPseudoHeaderCountsTheAddresses(t *testing.T) {
	s := segment([]byte("hello"))

	first := TCP([4]byte{192, 168, 1, 2}, [4]byte{93, 184, 216, 34}, s)
	second := TCP([4]byte{192, 168, 1, 3}, [4]byte{93, 184, 216, 34}, s)

	if first == second {
		t.Error("changing the source address left the checksum alone")
	}
}
