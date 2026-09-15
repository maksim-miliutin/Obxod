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

const (
	ipChecksumAt  = 10
	tcpChecksumAt = 16
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

func seqOf(packet []byte) uint32 {
	return binary.BigEndian.Uint32(packet[20+tcpSeqAt : 20+tcpSeqAt+4])
}

func TestCopyShiftsSequence(t *testing.T) {
	packet := build(ip.ProtocolTCP, 64, []byte("hello"), 0)
	before := seqOf(packet)

	copied, err := Copy(packet, Recipe{SeqDelta: 100000})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if got := seqOf(copied); got != before+100000 {
		t.Errorf("seq = %d, want %d", got, before+100000)
	}

	// The point of badseq: the number is wrong for the stream, yet the checksum
	// is right for the packet, so it travels intact until the server rejects it.
	verify(t, copied)
}

func TestCopyKeepsSequenceWhenNoneAsked(t *testing.T) {
	packet := build(ip.ProtocolTCP, 64, []byte("hello"), 0)
	before := seqOf(packet)

	copied, err := Copy(packet, Recipe{TTL: 4})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if got := seqOf(copied); got != before {
		t.Errorf("seq = %d, want the original %d", got, before)
	}
}

func TestCopySequenceWraps(t *testing.T) {
	packet := build(ip.ProtocolTCP, 64, []byte("hello"), 0)
	binary.BigEndian.PutUint32(packet[20+tcpSeqAt:20+tcpSeqAt+4], 0xfffffff0)

	copied, err := Copy(packet, Recipe{SeqDelta: 0x20})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	// 0xfffffff0 + 0x20 wraps to 0x10; the field is 32 bits and must roll over.
	if got := seqOf(copied); got != 0x10 {
		t.Errorf("seq = %#x, want 0x10", got)
	}

	verify(t, copied)
}

func TestCopyTTLAndSeqTogether(t *testing.T) {
	packet := build(ip.ProtocolTCP, 64, []byte("hello there"), 0)
	before := seqOf(packet)

	copied, err := Copy(packet, Recipe{TTL: 3, SeqDelta: 50})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if copied[ttlAt] != 3 {
		t.Errorf("ttl = %d, want 3", copied[ttlAt])
	}

	if got := seqOf(copied); got != before+50 {
		t.Errorf("seq = %d, want %d", got, before+50)
	}

	verify(t, copied)
}

func tcpSumField(packet []byte) uint16 {
	return binary.BigEndian.Uint16(packet[20+tcpChecksumAt : 20+tcpChecksumAt+2])
}

func TestCopyBadSumIsWrong(t *testing.T) {
	for _, size := range []int{1, 5, 40, 517, 1400} {
		packet := build(ip.ProtocolTCP, 64, bytes.Repeat([]byte{0xab}, size), 0)

		good, err := Copy(packet, Recipe{})
		if err != nil {
			t.Fatalf("Copy good: %v", err)
		}

		bad, err := Copy(packet, Recipe{BadSum: true})
		if err != nil {
			t.Fatalf("Copy bad: %v", err)
		}

		right := tcpSumField(good)
		wrong := tcpSumField(bad)

		if wrong == right {
			t.Errorf("size %d: badsum left the right checksum %#04x", size, right)
		}

		// A bad checksum must never read as 0x0000: that means "no checksum" in TCP,
		// which some stacks accept, so the copy could slip through to the server.
		if wrong == 0 {
			t.Errorf("size %d: badsum produced 0x0000, which reads as no checksum", size)
		}
	}
}

func TestCopyBadSumStillShiftsSequence(t *testing.T) {
	packet := build(ip.ProtocolTCP, 64, []byte("hello"), 0)
	before := seqOf(packet)

	copied, err := Copy(packet, Recipe{SeqDelta: 77, BadSum: true})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if got := seqOf(copied); got != before+77 {
		t.Errorf("seq = %d, want %d", got, before+77)
	}

	// badsum touches only the TCP checksum: the IP header must still verify.
	outer, err := ip.Parse(copied)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	if got := checksum.Of(copied[:outer.HeaderLen]); got != 0 {
		t.Errorf("ip checksum checks out as %#04x, want 0", got)
	}
}

func TestCopyBadSumLeavesIPHeaderValid(t *testing.T) {
	packet := build(ip.ProtocolTCP, 64, bytes.Repeat([]byte{0xcd}, 200), 0)

	copied, err := Copy(packet, Recipe{BadSum: true})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	outer, err := ip.Parse(copied)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	if got := checksum.Of(copied[:outer.HeaderLen]); got != 0 {
		t.Errorf("ip header checks out as %#04x, want 0", got)
	}
}

func helloWithName(host string) []byte {
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

func TestCopyWearsTheDecoyName(t *testing.T) {
	const real = "updates.discord.com"
	const decoy = "xxxxxxxx.google.com"

	payload := helloWithName(real)
	at := bytes.Index(payload, []byte(real))

	packet := build(ip.ProtocolTCP, 64, payload, 0)

	copied, err := Copy(packet, Recipe{TTL: 4, Name: decoy, NameAt: at})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if bytes.Contains(copied, []byte(real)) {
		t.Error("the copy still carries the blocked name")
	}

	if !bytes.Contains(copied, []byte(decoy)) {
		t.Error("the copy does not carry the decoy name")
	}

	if len(copied) != len(packet) {
		t.Errorf("copy is %d bytes, original %d: a decoy must not change the length", len(copied), len(packet))
	}

	verify(t, copied)
}

func TestCopyLeavesTheRealHelloAlone(t *testing.T) {
	const real = "updates.discord.com"

	payload := helloWithName(real)
	at := bytes.Index(payload, []byte(real))
	packet := build(ip.ProtocolTCP, 64, payload, 0)
	before := append([]byte(nil), packet...)

	if _, err := Copy(packet, Recipe{Name: "xxxxxxxx.google.com", NameAt: at}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if !bytes.Equal(packet, before) {
		t.Error("the real hello was rewritten; only the copy may carry a decoy")
	}
}

func TestCopyDecoyMustFit(t *testing.T) {
	payload := helloWithName("updates.discord.com")
	at := bytes.Index(payload, []byte("updates.discord.com"))
	packet := build(ip.ProtocolTCP, 64, payload, 0)

	cases := []struct {
		name   string
		decoy  string
		nameAt int
	}{
		{"past the end", "xxxxxxxx.google.com", len(payload) - 3},
		{"negative offset", "xxxxxxxx.google.com", -1},
		{"longer than the payload", bytes.NewBuffer(bytes.Repeat([]byte("a"), 5000)).String(), at},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Copy(packet, Recipe{Name: c.decoy, NameAt: c.nameAt}); !errors.Is(err, ErrNameSpace) {
				t.Errorf("err = %v, want %v", err, ErrNameSpace)
			}
		})
	}
}
