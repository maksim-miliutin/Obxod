package tcp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"obxod/internal/ip"
)

func build(dataOffset int, seq uint32, flags byte, payload []byte) []byte {
	headerLen := dataOffset * 4
	s := make([]byte, headerLen+len(payload))

	binary.BigEndian.PutUint16(s[0:2], 54321)
	binary.BigEndian.PutUint16(s[2:4], 443)
	binary.BigEndian.PutUint32(s[4:8], seq)
	s[12] = byte(dataOffset << 4)
	s[13] = flags
	copy(s[headerLen:], payload)

	return s
}

func TestParseFields(t *testing.T) {
	payload := []byte{0x16, 0x03, 0x01}

	h, err := Parse(build(5, 1000, FlagPSH|FlagACK, payload))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if h.SrcPort != 54321 {
		t.Errorf("SrcPort = %d, want 54321", h.SrcPort)
	}
	if h.DstPort != 443 {
		t.Errorf("DstPort = %d, want 443", h.DstPort)
	}
	if h.Seq != 1000 {
		t.Errorf("Seq = %d, want 1000", h.Seq)
	}
	if h.HeaderLen != 20 {
		t.Errorf("HeaderLen = %d, want 20", h.HeaderLen)
	}
	if !bytes.Equal(h.Payload, payload) {
		t.Errorf("Payload = %x, want %x", h.Payload, payload)
	}
}

func TestParseSkipsOptions(t *testing.T) {
	payload := []byte("hello")

	h, err := Parse(build(8, 1, FlagACK, payload))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if h.HeaderLen != 32 {
		t.Fatalf("HeaderLen = %d, want 32", h.HeaderLen)
	}
	if !bytes.Equal(h.Payload, payload) {
		t.Errorf("Payload = %q, want %q", h.Payload, payload)
	}
}

func TestParseFlags(t *testing.T) {
	cases := []struct {
		name  string
		raw   byte
		set   uint8
		unset uint8
	}{
		{"syn", FlagSYN, FlagSYN, FlagACK},
		{"syn ack", FlagSYN | FlagACK, FlagACK, FlagRST},
		{"push ack", FlagPSH | FlagACK, FlagPSH, FlagSYN},
		{"fin ack", FlagFIN | FlagACK, FlagFIN, FlagSYN},
		{"reset", FlagRST, FlagRST, FlagACK},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, err := Parse(build(5, 1, c.raw, nil))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if h.Flags&c.set == 0 {
				t.Errorf("flags %08b missing %08b", h.Flags, c.set)
			}
			if h.Flags&c.unset != 0 {
				t.Errorf("flags %08b carry %08b", h.Flags, c.unset)
			}
		})
	}
}

func TestParseAckCarriesNoPayload(t *testing.T) {
	h, err := Parse(build(5, 7, FlagACK, nil))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(h.Payload) != 0 {
		t.Errorf("Payload = %x, want empty", h.Payload)
	}
}

func TestParseErrors(t *testing.T) {
	short := build(5, 1, FlagACK, nil)[:19]

	truncated := build(8, 1, FlagACK, nil)[:20]
	truncated[12] = byte(8 << 4)

	stunted := build(5, 1, FlagACK, nil)
	stunted[12] = byte(4 << 4)

	cases := []struct {
		name    string
		segment []byte
		want    error
	}{
		{"empty", nil, ErrTooShort},
		{"one byte short of a header", short, ErrTooShort},
		{"options promised but not captured", truncated, ErrTooShort},
		{"data offset below 20", stunted, ErrBadDataOffset},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.segment)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestPayloadAliasesSegment(t *testing.T) {
	segment := build(5, 1, FlagPSH|FlagACK, []byte("hello"))

	h, err := Parse(segment)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	h.Payload[0] = 'j'

	if segment[20] != 'j' {
		t.Errorf("segment[20] = %q, want the write to go through", segment[20])
	}
}

func TestParseOverIPPayload(t *testing.T) {
	payload := []byte{0x16, 0x03, 0x01, 0x00, 0x05}
	segment := build(5, 1, FlagPSH|FlagACK, payload)

	packet := make([]byte, 20+len(segment))
	packet[0] = 4<<4 | 5
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	packet[8] = 64
	packet[9] = ip.ProtocolTCP
	copy(packet[20:], segment)

	outer, err := ip.Parse(packet)
	if err != nil {
		t.Fatalf("ip.Parse: %v", err)
	}

	h, err := Parse(outer.Payload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if h.DstPort != 443 {
		t.Errorf("DstPort = %d, want 443", h.DstPort)
	}
	if !bytes.Equal(h.Payload, payload) {
		t.Errorf("Payload = %x, want %x", h.Payload, payload)
	}

	h.Payload[0] = 0xff

	if packet[40] != 0xff {
		t.Errorf("packet[40] = %#x, want the write to reach the original packet", packet[40])
	}
}
