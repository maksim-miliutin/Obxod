package replies

import (
	"encoding/binary"
	"errors"
	"testing"

	"obxod/internal/divert"
	"obxod/internal/ip"
	"obxod/internal/tcp"
)

func reply(srcPort, dstPort uint16, flags byte, payload []byte) []byte {
	segment := make([]byte, 20)
	binary.BigEndian.PutUint16(segment[0:2], srcPort)
	binary.BigEndian.PutUint16(segment[2:4], dstPort)
	segment[12] = 5 << 4
	segment[13] = flags
	segment = append(segment, payload...)

	packet := make([]byte, 20)
	packet[0] = 4<<4 | 5
	packet[8] = 64
	packet[9] = ip.ProtocolTCP
	packet = append(packet, segment...)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))

	return packet
}

type feed struct {
	packets [][]byte
	at      int
}

var errDone = errors.New("done")

func (f *feed) Recv(buf []byte) (int, divert.Addr, error) {
	if f.at >= len(f.packets) {
		return 0, divert.Addr{}, errDone
	}

	n := copy(buf, f.packets[f.at])
	f.at++

	return n, divert.Addr{}, nil
}

func TestRunReportsResetsAndData(t *testing.T) {
	f := &feed{packets: [][]byte{
		reply(443, 54321, tcp.FlagRST|tcp.FlagACK, nil),
		reply(443, 54322, tcp.FlagPSH|tcp.FlagACK, []byte{0x16, 0x03, 0x03}),
		reply(443, 54323, tcp.FlagACK, nil),
		reply(443, 54325, tcp.FlagRST, nil),
		reply(443, 54326, tcp.FlagFIN|tcp.FlagACK, nil),
	}}

	var resets, data, closed []uint16

	w := Watch{
		Reset:  func(port uint16) { resets = append(resets, port) },
		Data:   func(port uint16) { data = append(data, port) },
		Closed: func(port uint16) { closed = append(closed, port) },
	}

	if err := w.Run(f); !errors.Is(err, errDone) {
		t.Fatalf("Run: %v", err)
	}

	if len(resets) != 2 || resets[0] != 54321 || resets[1] != 54325 {
		t.Errorf("resets = %v, want 54321 and 54325", resets)
	}

	if len(data) != 1 || data[0] != 54322 {
		t.Errorf("data = %v, want just 54322", data)
	}

	// A polite close must not be counted as data, or a finished request looks alive.
	if len(closed) != 1 || closed[0] != 54326 {
		t.Errorf("closed = %v, want just 54326", closed)
	}
}

func TestReadIgnoresWhatIsNotOurs(t *testing.T) {
	cases := []struct {
		name   string
		packet []byte
	}{
		{"empty", nil},
		{"not tcp", []byte{4<<4 | 5, 0, 0, 20, 0, 0, 0, 0, 64, ip.ProtocolUDP, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"bare ack", reply(443, 54321, tcp.FlagACK, nil)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, ok := read(c.packet); ok {
				t.Error("a packet that is none of our business was reported")
			}
		})
	}
}

func TestRunSurvivesMissingCallbacks(t *testing.T) {
	f := &feed{packets: [][]byte{
		reply(443, 54321, tcp.FlagRST, nil),
		reply(443, 54322, tcp.FlagPSH, []byte{0x16}),
	}}

	if err := (Watch{}).Run(f); !errors.Is(err, errDone) {
		t.Fatalf("Run: %v", err)
	}
}
