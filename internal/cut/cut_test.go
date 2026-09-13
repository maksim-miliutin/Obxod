package cut

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"obxod/internal/checksum"
	"obxod/internal/clienthello"
	"obxod/internal/ip"
	"obxod/internal/tcp"
)

func hello(host string) []byte {
	name := []byte(host)

	list := binary.BigEndian.AppendUint16(nil, uint16(len(name)+3))
	list = append(list, 0x00)
	list = binary.BigEndian.AppendUint16(list, uint16(len(name)))
	list = append(list, name...)

	sni := binary.BigEndian.AppendUint16(nil, 0x0000)
	sni = binary.BigEndian.AppendUint16(sni, uint16(len(list)))
	sni = append(sni, list...)

	extensions := binary.BigEndian.AppendUint16(nil, uint16(len(sni)))
	extensions = append(extensions, sni...)

	body := []byte{0x03, 0x03}
	body = append(body, bytes.Repeat([]byte{0xab}, 32)...)
	body = append(body, 0x00, 0x00, 0x02, 0x13, 0x01, 0x01, 0x00)
	body = append(body, extensions...)

	handshake := []byte{0x01, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}
	handshake = append(handshake, body...)

	record := []byte{0x16, 0x03, 0x01, 0x00, 0x00}
	binary.BigEndian.PutUint16(record[3:5], uint16(len(handshake)))

	return append(record, handshake...)
}

func packetWith(seq uint32, payload []byte) []byte {
	segment := make([]byte, 20)
	binary.BigEndian.PutUint16(segment[0:2], 54321)
	binary.BigEndian.PutUint16(segment[2:4], 443)
	binary.BigEndian.PutUint32(segment[4:8], seq)
	segment[12] = 5 << 4
	segment[13] = 0x18
	segment = append(segment, payload...)

	packet := make([]byte, 20)
	packet[0] = 4<<4 | 5
	packet[8] = 64
	packet[9] = ip.ProtocolTCP
	copy(packet[12:16], []byte{192, 168, 1, 2})
	copy(packet[16:20], []byte{93, 184, 216, 34})
	packet = append(packet, segment...)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))

	return packet
}

func payloadOf(t *testing.T, packet []byte) []byte {
	t.Helper()

	outer, err := ip.Parse(packet)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		t.Fatalf("tcp.Parse: %v", err)
	}

	return segment.Payload
}

func seqOf(t *testing.T, packet []byte) uint32 {
	t.Helper()

	outer, err := ip.Parse(packet)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		t.Fatalf("tcp.Parse: %v", err)
	}

	return segment.Seq
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
	want := checksum.TCP(outer.Src, outer.Dst, segment)

	if got := binary.BigEndian.Uint16(segment[tcpChecksumAt : tcpChecksumAt+2]); got != want {
		t.Errorf("tcp checksum = %#04x, want %#04x", got, want)
	}
}

func TestHalvesRebuildTheOriginal(t *testing.T) {
	payload := hello("updates.discord.com")

	for _, point := range []int{1, 2, 10, 40, len(payload) / 2, len(payload) - 1} {
		first, second, err := At(packetWith(1000, payload), point)
		if err != nil {
			t.Fatalf("point %d: At: %v", point, err)
		}

		joined := append(append([]byte(nil), payloadOf(t, first)...), payloadOf(t, second)...)

		if !bytes.Equal(joined, payload) {
			t.Errorf("point %d: the halves do not add up to the original", point)
		}
	}
}

func TestSecondHalfContinuesTheStream(t *testing.T) {
	payload := hello("updates.discord.com")

	for _, point := range []int{1, 17, 60} {
		first, second, err := At(packetWith(5000, payload), point)
		if err != nil {
			t.Fatalf("At: %v", err)
		}

		if got := seqOf(t, first); got != 5000 {
			t.Errorf("first seq = %d, want 5000", got)
		}

		// No gap, no overlap: the second half starts exactly where the first ended.
		if got := seqOf(t, second); got != 5000+uint32(point) {
			t.Errorf("second seq = %d, want %d", got, 5000+uint32(point))
		}

		if got := len(payloadOf(t, first)); got != point {
			t.Errorf("first half carries %d bytes, want %d", got, point)
		}
	}
}

func TestBothHalvesAreWellFormed(t *testing.T) {
	payload := hello("updates.discord.com")

	first, second, err := At(packetWith(1000, payload), 30)
	if err != nil {
		t.Fatalf("At: %v", err)
	}

	verify(t, first)
	verify(t, second)

	for _, half := range [][]byte{first, second} {
		outer, err := ip.Parse(half)
		if err != nil {
			t.Fatalf("ip.Parse: %v", err)
		}

		if outer.TotalLen != len(half) {
			t.Errorf("total length says %d, packet is %d bytes", outer.TotalLen, len(half))
		}
	}
}

// The whole point: an inspector reading either packet on its own finds no host name.
func TestNameIsBrokenAcrossTheHalves(t *testing.T) {
	const host = "updates.discord.com"

	payload := hello(host)

	parsed, err := clienthello.Parse(payload)
	if err != nil {
		t.Fatalf("clienthello.Parse: %v", err)
	}

	name, err := parsed.ServerName()
	if err != nil {
		t.Fatalf("ServerName: %v", err)
	}

	point := name.Offset + len(host)/2

	first, second, err := At(packetWith(1000, payload), point)
	if err != nil {
		t.Fatalf("At: %v", err)
	}

	for _, half := range [][]byte{first, second} {
		if bytes.Contains(half, []byte(host)) {
			t.Errorf("a half still carries the whole name %q", host)
		}
	}

	joined := append(append([]byte(nil), payloadOf(t, first)...), payloadOf(t, second)...)

	if !bytes.Contains(joined, []byte(host)) {
		t.Error("the name did not survive being put back together")
	}
}

func TestLeavesTheOriginalAlone(t *testing.T) {
	packet := packetWith(1000, hello("updates.discord.com"))
	before := append([]byte(nil), packet...)

	if _, _, err := At(packet, 25); err != nil {
		t.Fatalf("At: %v", err)
	}

	if !bytes.Equal(packet, before) {
		t.Error("At wrote into the packet it was given")
	}
}

func TestErrors(t *testing.T) {
	payload := hello("updates.discord.com")

	cases := []struct {
		name   string
		packet []byte
		point  int
		want   error
	}{
		{"empty", nil, 5, ip.ErrTooShort},
		{"udp", udpPacket(), 5, ErrNotTCP},
		{"tcp header cut short", packetWith(1000, nil)[:32], 5, tcp.ErrTooShort},
		{"bare ack", packetWith(1000, nil), 5, ErrNoPayload},
		{"point past the payload", packetWith(1000, payload), len(payload) + 1, ErrBadPoint},
		{"negative point", packetWith(1000, payload), -1, ErrBadPoint},
		{"nothing on the left", packetWith(1000, payload), 0, ErrNoRoomLeft},
		{"nothing on the right", packetWith(1000, payload), len(payload), ErrNoRoomLeft},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := At(c.packet, c.point); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func udpPacket() []byte {
	datagram := make([]byte, 8)
	binary.BigEndian.PutUint16(datagram[2:4], 50021)
	binary.BigEndian.PutUint16(datagram[4:6], 8+5)
	datagram = append(datagram, []byte("voice")...)

	packet := make([]byte, 20)
	packet[0] = 4<<4 | 5
	packet[8] = 64
	packet[9] = ip.ProtocolUDP
	packet = append(packet, datagram...)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))

	return packet
}

// rebuilt is what a server sees: it counts from where it left off, so anything
// before that sequence number is behind its window and dropped.
func rebuilt(t *testing.T, from uint32, halves ...[]byte) []byte {
	t.Helper()

	out := make([]byte, 0, 4096)
	next := from

	for _, half := range halves {
		outer, err := ip.Parse(half)
		if err != nil {
			t.Fatalf("ip.Parse: %v", err)
		}

		segment, err := tcp.Parse(outer.Payload)
		if err != nil {
			t.Fatalf("tcp.Parse: %v", err)
		}

		payload := segment.Payload
		seq := segment.Seq

		if seq < next {
			behind := int(next - seq)
			if behind >= len(payload) {
				continue
			}

			payload = payload[behind:]
			seq = next
		}

		if seq != next {
			t.Fatalf("a gap in the stream: next is %d, half starts at %d", next, seq)
		}

		out = append(out, payload...)
		next += uint32(len(payload))
	}

	return out
}

// stacked is what a lazy inspector sees: payloads in the order they arrived,
// with no attention paid to sequence numbers.
func stacked(t *testing.T, halves ...[]byte) []byte {
	t.Helper()

	out := make([]byte, 0, 4096)

	for _, half := range halves {
		out = append(out, payloadOf(t, half)...)
	}

	return out
}

func TestOverlapServerRebuildsTheRealHello(t *testing.T) {
	const host = "gateway.discord.gg"

	payload := hello(host)
	pattern := hello("www.4pda.to")

	for _, point := range []int{1, 2, 30, len(payload) - 1} {
		first, second, err := Overlap(packetWith(1000, payload), pattern, point)
		if err != nil {
			t.Fatalf("point %d: Overlap: %v", point, err)
		}

		got := rebuilt(t, 1000, first, second)

		if !bytes.Equal(got, payload) {
			t.Errorf("point %d: the server would rebuild %d bytes, want the original %d", point, len(got), len(payload))
		}
	}
}

func TestOverlapInspectorReadsTheRecordedHello(t *testing.T) {
	const host = "gateway.discord.gg"
	const decoy = "www.4pda.to"

	pattern := hello(decoy)

	first, second, err := Overlap(packetWith(1000, hello(host)), pattern, 1)
	if err != nil {
		t.Fatalf("Overlap: %v", err)
	}

	seen := stacked(t, first, second)

	if !bytes.HasPrefix(seen, pattern) {
		t.Error("the recorded hello is not what comes first in the stream")
	}

	if !bytes.Contains(seen, []byte(decoy)) {
		t.Error("the decoy name never reaches an inspector")
	}

	// The real name still travels, but only behind a whole hello for another site.
	at := bytes.Index(seen, []byte(host))
	if at >= 0 && at < len(pattern) {
		t.Error("the real name shows up before the recorded hello is over")
	}
}

func TestOverlapSequenceNumbers(t *testing.T) {
	pattern := hello("www.4pda.to")

	first, second, err := Overlap(packetWith(5000, hello("gateway.discord.gg")), pattern, 3)
	if err != nil {
		t.Fatalf("Overlap: %v", err)
	}

	if got := seqOf(t, first); got != 5000-uint32(len(pattern)) {
		t.Errorf("first seq = %d, want %d", got, 5000-uint32(len(pattern)))
	}

	if got := seqOf(t, second); got != 5003 {
		t.Errorf("second seq = %d, want 5003", got)
	}

	if got := len(payloadOf(t, first)); got != len(pattern)+3 {
		t.Errorf("first half carries %d bytes, want %d", got, len(pattern)+3)
	}
}

func TestOverlapHalvesAreWellFormed(t *testing.T) {
	first, second, err := Overlap(packetWith(1000, hello("gateway.discord.gg")), hello("www.4pda.to"), 2)
	if err != nil {
		t.Fatalf("Overlap: %v", err)
	}

	verify(t, first)
	verify(t, second)

	for _, half := range [][]byte{first, second} {
		outer, err := ip.Parse(half)
		if err != nil {
			t.Fatalf("ip.Parse: %v", err)
		}

		if outer.TotalLen != len(half) {
			t.Errorf("total length says %d, packet is %d bytes", outer.TotalLen, len(half))
		}
	}
}

func TestOverlapLeavesTheOriginalAlone(t *testing.T) {
	packet := packetWith(1000, hello("gateway.discord.gg"))
	before := append([]byte(nil), packet...)

	if _, _, err := Overlap(packet, hello("www.4pda.to"), 2); err != nil {
		t.Fatalf("Overlap: %v", err)
	}

	if !bytes.Equal(packet, before) {
		t.Error("Overlap wrote into the packet it was given")
	}
}

func TestOverlapErrors(t *testing.T) {
	payload := hello("gateway.discord.gg")
	pattern := hello("www.4pda.to")

	cases := []struct {
		name    string
		packet  []byte
		pattern []byte
		point   int
		want    error
	}{
		{"no pattern", packetWith(1000, payload), nil, 2, ErrNoPattern},
		{"empty packet", nil, pattern, 2, ip.ErrTooShort},
		{"udp", udpPacket(), pattern, 2, ErrNotTCP},
		{"bare ack", packetWith(1000, nil), pattern, 2, ErrNoPayload},
		{"point past the payload", packetWith(1000, payload), pattern, len(payload) + 1, ErrBadPoint},
		{"nothing on the left", packetWith(1000, payload), pattern, 0, ErrNoRoomLeft},
		{"nothing on the right", packetWith(1000, payload), pattern, len(payload), ErrNoRoomLeft},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := Overlap(c.packet, c.pattern, c.point); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}
