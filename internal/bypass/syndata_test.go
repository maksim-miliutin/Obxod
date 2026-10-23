package bypass

import (
	"bytes"
	"encoding/binary"
	"testing"

	"obxod/internal/ip"
	"obxod/internal/tcp"
)

func synPacket(seq uint32) []byte {
	segment := make([]byte, 20)
	binary.BigEndian.PutUint16(segment[0:2], 51234)
	binary.BigEndian.PutUint16(segment[2:4], 443)
	binary.BigEndian.PutUint32(segment[4:8], seq)
	segment[12] = 5 << 4
	segment[13] = tcp.FlagSYN

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

func TestIsClientSYN(t *testing.T) {
	if !isClientSYN(synPacket(1000)) {
		t.Error("an opening syn was not recognised")
	}

	synAck := synPacket(1000)

	outer, err := ip.Parse(synAck)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	synAck[outer.HeaderLen+13] |= tcp.FlagACK // 13 is the flags byte within the segment

	if isClientSYN(synAck) {
		t.Error("a syn-ack was taken for an opening syn")
	}

	if isClientSYN(packet443(clientHello("discord.com"))) {
		t.Error("a data packet was taken for an opening syn")
	}

	if isClientSYN(datagramTo(443)) {
		t.Error("a udp datagram was taken for an opening syn")
	}
}

// Wet syndata sends one syn carrying a payload and drops the empty original, so the
// dpi reads a fake first packet while the handshake itself still completes.
func TestRunSynDataReplacesTheEmptySyn(t *testing.T) {
	syn := synPacket(1000)

	wire := runOver(t, Settings{SynData: true, Wet: true}, syn)

	if len(wire.sent) != 1 {
		t.Fatalf("sent %d packets, want the one syn carrying data", len(wire.sent))
	}

	if bytes.Equal(wire.sent[0], syn) {
		t.Error("the empty original syn was sent, not the one carrying data")
	}

	if !isClientSYN(wire.sent[0]) {
		t.Error("what went out is no longer an opening syn")
	}

	if len(payloadOf(t, wire.sent[0])) == 0 {
		t.Error("the syn carries no data")
	}
}

// The sequence number field stays the syn's own: it numbers the phantom byte the
// syn flag consumes, so moving it would open a hole in the stream.
func TestSynDataKeepsTheSequenceNumber(t *testing.T) {
	const seq = 0x11223344

	wire := runOver(t, Settings{SynData: true, Wet: true}, synPacket(seq))

	if len(wire.sent) != 1 {
		t.Fatalf("sent %d packets, want one", len(wire.sent))
	}

	if got := seqOf(wire.sent[0]); got != seq {
		t.Errorf("seq = %#x, want the syn's own %#x", got, seq)
	}
}

// A dry run alters nothing: the empty syn passes untouched, so a look costs the
// connection nothing.
func TestSynDataDryKeepsTheOriginal(t *testing.T) {
	syn := synPacket(1000)

	wire := runOver(t, Settings{SynData: true}, syn)

	if len(wire.sent) != 1 || !bytes.Equal(wire.sent[0], syn) {
		t.Errorf("a dry run sent %d packets and changed the syn", len(wire.sent))
	}
}

// A recorded hello makes a more convincing blob than the default zeros, so when one
// is loaded the syn carries it.
func TestSynDataCarriesTheRecordedHello(t *testing.T) {
	recorded := clientHello("www.google.com")

	wire := runOver(t, Settings{SynData: true, Recorded: recorded, Wet: true}, synPacket(1000))

	if len(wire.sent) != 1 {
		t.Fatalf("sent %d packets, want one", len(wire.sent))
	}

	if !bytes.Contains(payloadOf(t, wire.sent[0]), []byte("www.google.com")) {
		t.Error("the syn does not carry the recorded hello")
	}
}

// Without a recording the default is zapret's: sixteen zero bytes, enough for the
// dpi to see data where a bare syn has none.
func TestSynDataDefaultsToSixteenZeros(t *testing.T) {
	wire := runOver(t, Settings{SynData: true, Wet: true}, synPacket(1000))

	if payload := payloadOf(t, wire.sent[0]); !bytes.Equal(payload, make([]byte, 16)) {
		t.Errorf("default syn payload = %x, want sixteen zero bytes", payload)
	}
}

// syndata only touches opening syns; a hello and everything else keep their usual
// path, so a run with no rule passes them through unchanged.
func TestSynDataLeavesHellosAlone(t *testing.T) {
	hello := packet443(clientHello("example.org"))

	wire := runOver(t, Settings{SynData: true, Wet: true}, hello)

	if len(wire.sent) != 1 || !bytes.Equal(wire.sent[0], hello) {
		t.Error("syndata changed a hello it should have left to the ways")
	}
}
