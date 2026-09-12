package ip

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func build(ihl int, totalLen int, ttl byte, protocol byte, payload []byte) []byte {
	headerLen := ihl * 4
	p := make([]byte, headerLen+len(payload))

	p[0] = byte(4<<4 | ihl)
	binary.BigEndian.PutUint16(p[2:4], uint16(totalLen))
	p[8] = ttl
	p[9] = protocol
	copy(p[12:16], []byte{192, 168, 1, 2})
	copy(p[16:20], []byte{93, 184, 216, 34})
	copy(p[headerLen:], payload)

	return p
}

func TestParseFields(t *testing.T) {
	payload := []byte("hello")
	h, err := Parse(build(5, 25, 64, ProtocolTCP, payload))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if h.HeaderLen != 20 {
		t.Errorf("HeaderLen = %d, want 20", h.HeaderLen)
	}
	if h.TotalLen != 25 {
		t.Errorf("TotalLen = %d, want 25", h.TotalLen)
	}
	if h.TTL != 64 {
		t.Errorf("TTL = %d, want 64", h.TTL)
	}
	if h.Protocol != ProtocolTCP {
		t.Errorf("Protocol = %d, want %d", h.Protocol, ProtocolTCP)
	}
	if h.Src != [4]byte{192, 168, 1, 2} {
		t.Errorf("Src = %v, want 192.168.1.2", h.Src)
	}
	if h.Dst != [4]byte{93, 184, 216, 34} {
		t.Errorf("Dst = %v, want 93.184.216.34", h.Dst)
	}
	if !bytes.Equal(h.Payload, payload) {
		t.Errorf("Payload = %q, want %q", h.Payload, payload)
	}
}

func TestParseSkipsOptions(t *testing.T) {
	payload := []byte("hello")
	packet := build(6, 29, 64, ProtocolTCP, payload)

	h, err := Parse(packet)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if h.HeaderLen != 24 {
		t.Fatalf("HeaderLen = %d, want 24", h.HeaderLen)
	}
	if !bytes.Equal(h.Payload, payload) {
		t.Errorf("Payload = %q, want %q", h.Payload, payload)
	}
}

func TestParseUDP(t *testing.T) {
	h, err := Parse(build(5, 28, 128, ProtocolUDP, []byte("12345678")))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if h.Protocol != ProtocolUDP {
		t.Errorf("Protocol = %d, want %d", h.Protocol, ProtocolUDP)
	}
}

func TestParseErrors(t *testing.T) {
	short := build(5, 20, 64, ProtocolTCP, nil)[:19]

	truncated := build(6, 24, 64, ProtocolTCP, nil)[:20]
	truncated[0] = byte(4<<4 | 6)

	sixth := build(5, 20, 64, ProtocolTCP, nil)
	sixth[0] = byte(6<<4 | 5)

	stunted := build(5, 20, 64, ProtocolTCP, nil)
	stunted[0] = byte(4<<4 | 4)

	cases := []struct {
		name   string
		packet []byte
		want   error
	}{
		{"empty", nil, ErrTooShort},
		{"one byte short of a header", short, ErrTooShort},
		{"options promised but not captured", truncated, ErrTooShort},
		{"version 6", sixth, ErrNotIPv4},
		{"header length below 20", stunted, ErrBadHeaderLen},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.packet)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestPayloadFallsBackToCapturedLength(t *testing.T) {
	payload := []byte("hello")

	cases := []struct {
		name     string
		totalLen int
	}{
		{"offload left it larger", 1500},
		{"offload left it zero", 0},
		{"smaller than the header", 12},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, err := Parse(build(5, c.totalLen, 64, ProtocolTCP, payload))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if !bytes.Equal(h.Payload, payload) {
				t.Errorf("Payload = %q, want %q", h.Payload, payload)
			}
		})
	}
}

func TestPayloadAliasesPacket(t *testing.T) {
	packet := build(5, 25, 64, ProtocolTCP, []byte("hello"))

	h, err := Parse(packet)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	h.Payload[0] = 'j'

	if packet[20] != 'j' {
		t.Errorf("packet[20] = %q, want the write to go through", packet[20])
	}
}
