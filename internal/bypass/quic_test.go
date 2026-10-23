package bypass

import (
	"bytes"
	"encoding/binary"
	"testing"

	"obxod/internal/checksum"
	"obxod/internal/divert"
	"obxod/internal/ip"
	"obxod/internal/quic"
)

func quicInitialPacket(t *testing.T) []byte {
	t.Helper()

	decoy, err := quic.Decoy("example.com")
	if err != nil {
		t.Fatal(err)
	}

	body := make([]byte, 8)
	binary.BigEndian.PutUint16(body[0:2], 51234)
	binary.BigEndian.PutUint16(body[2:4], 443)
	binary.BigEndian.PutUint16(body[4:6], uint16(8+len(decoy)))
	body = append(body, decoy...)

	packet := make([]byte, 20)
	packet[0] = 4<<4 | 5
	packet[8] = 64
	packet[9] = ip.ProtocolUDP
	copy(packet[12:16], []byte{192, 168, 1, 2})
	copy(packet[16:20], []byte{142, 250, 72, 206})
	packet = append(packet, body...)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))

	return packet
}

func TestFakeQUICSendsDecoysAhead(t *testing.T) {
	e := New(Settings{FakeQUIC: "www.google.com", Wet: true})
	r := &recorder{}

	if err := e.fakeInitial(r, quicInitialPacket(t), &divert.Addr{}); err != nil {
		t.Fatalf("fakeInitial: %v", err)
	}

	if len(r.sent) != quicCopies {
		t.Fatalf("sent %d decoys, want %d", len(r.sent), quicCopies)
	}

	for i, one := range r.sent {
		outer, err := ip.Parse(one)
		if err != nil {
			t.Fatalf("decoy %d does not parse: %v", i, err)
		}

		if !quic.IsInitial(outer.Payload[8:]) {
			t.Errorf("decoy %d does not read as a quic initial", i)
		}
	}
}

// The decoy has to die before the server, so its udp checksum is wrong on purpose.
func TestFakeQUICDecoyDiesAtTheServer(t *testing.T) {
	e := New(Settings{FakeQUIC: "www.google.com", Wet: true})
	r := &recorder{}

	if err := e.fakeInitial(r, quicInitialPacket(t), &divert.Addr{}); err != nil {
		t.Fatalf("fakeInitial: %v", err)
	}

	outer, err := ip.Parse(r.sent[0])
	if err != nil {
		t.Fatalf("decoy does not parse: %v", err)
	}

	var src, dst [4]byte

	copy(src[:], r.sent[0][12:16])
	copy(dst[:], r.sent[0][16:20])

	stored := binary.BigEndian.Uint16(outer.Payload[udpChecksumField : udpChecksumField+2])
	if stored == checksum.UDP(src, dst, outer.Payload) {
		t.Error("the decoy's udp checksum checks out, so the server would accept it")
	}
}

const udpChecksumField = 6

func TestFakeQUICPassesTheRealInitial(t *testing.T) {
	packet := quicInitialPacket(t)

	wire := runOver(t, Settings{FakeQUIC: "www.google.com", Wet: true}, packet)

	if len(wire.sent) != quicCopies+1 {
		t.Fatalf("sent %d packets, want %d decoys and the real one", len(wire.sent), quicCopies+1)
	}

	if !bytes.Equal(wire.sent[len(wire.sent)-1], packet) {
		t.Error("the real initial came out changed or never followed its decoys")
	}
}

func TestFakeQUICSendsNothingWhileDry(t *testing.T) {
	packet := quicInitialPacket(t)

	wire := runOver(t, Settings{FakeQUIC: "www.google.com"}, packet)

	if len(wire.sent) != 1 {
		t.Fatalf("sent %d packets, want only the real one passing", len(wire.sent))
	}

	if !bytes.Equal(wire.sent[0], packet) {
		t.Error("the real initial was altered in a dry run")
	}
}

// Fake mode must not touch the later packets of a connection, only the Initial.
func TestFakeQUICLeavesLaterPacketsAlone(t *testing.T) {
	wire := runOver(t, Settings{FakeQUIC: "www.google.com", Wet: true}, datagramTo(443))

	if len(wire.sent) != 1 {
		t.Fatalf("sent %d packets, want the lone non-initial datagram to pass", len(wire.sent))
	}
}
