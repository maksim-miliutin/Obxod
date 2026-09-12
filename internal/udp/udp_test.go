package udp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"obxod/internal/ip"
)

func build(srcPort, dstPort uint16, length int, payload []byte) []byte {
	d := make([]byte, headerLen+len(payload))

	binary.BigEndian.PutUint16(d[0:2], srcPort)
	binary.BigEndian.PutUint16(d[2:4], dstPort)
	binary.BigEndian.PutUint16(d[4:6], uint16(length))
	copy(d[headerLen:], payload)

	return d
}

func TestParseFields(t *testing.T) {
	payload := bytes.Repeat([]byte{0xaa}, 120)

	h, err := Parse(build(51234, 50021, headerLen+len(payload), payload))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if h.SrcPort != 51234 {
		t.Errorf("SrcPort = %d, want 51234", h.SrcPort)
	}
	if h.DstPort != 50021 {
		t.Errorf("DstPort = %d, want 50021", h.DstPort)
	}
	if h.Length != headerLen+len(payload) {
		t.Errorf("Length = %d, want %d", h.Length, headerLen+len(payload))
	}
	if !bytes.Equal(h.Payload, payload) {
		t.Errorf("Payload = %d bytes, want %d", len(h.Payload), len(payload))
	}
}

func TestParseEmptyDatagram(t *testing.T) {
	h, err := Parse(build(1, 2, headerLen, nil))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(h.Payload) != 0 {
		t.Errorf("Payload = %d bytes, want none", len(h.Payload))
	}
}

func TestParseFallsBackToCapturedLength(t *testing.T) {
	payload := []byte("voice")

	cases := []struct {
		name   string
		length int
	}{
		{"offload left it larger", 1200},
		{"offload left it zero", 0},
		{"shorter than the header", 4},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, err := Parse(build(1, 2, c.length, payload))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if !bytes.Equal(h.Payload, payload) {
				t.Errorf("Payload = %q, want %q", h.Payload, payload)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name     string
		datagram []byte
		want     error
	}{
		{"empty", nil, ErrTooShort},
		{"one byte short of a header", build(1, 2, headerLen, nil)[:7], ErrTooShort},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.datagram)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestPayloadAliasesDatagram(t *testing.T) {
	datagram := build(1, 2, headerLen+5, []byte("hello"))

	h, err := Parse(datagram)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	h.Payload[0] = 'j'

	if datagram[headerLen] != 'j' {
		t.Errorf("datagram[%d] = %q, want the write to go through", headerLen, datagram[headerLen])
	}
}

func TestParseOverIPPayload(t *testing.T) {
	payload := bytes.Repeat([]byte{0xcd}, 200)
	datagram := build(51234, 19301, headerLen+len(payload), payload)

	packet := make([]byte, 20+len(datagram))
	packet[0] = 4<<4 | 5
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	packet[8] = 64
	packet[9] = ip.ProtocolUDP
	copy(packet[20:], datagram)

	outer, err := ip.Parse(packet)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	if outer.Protocol != ip.ProtocolUDP {
		t.Fatalf("Protocol = %d, want %d", outer.Protocol, ip.ProtocolUDP)
	}

	h, err := Parse(outer.Payload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if h.DstPort != 19301 {
		t.Errorf("DstPort = %d, want 19301", h.DstPort)
	}
	if !bytes.Equal(h.Payload, payload) {
		t.Errorf("Payload = %d bytes, want %d", len(h.Payload), len(payload))
	}

	h.Payload[0] = 0xff

	if packet[20+headerLen] != 0xff {
		t.Errorf("packet[%d] = %#x, want the write to reach the original packet", 20+headerLen, packet[20+headerLen])
	}
}
