package clienthello

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func ext(kind uint16, data []byte) []byte {
	e := binary.BigEndian.AppendUint16(nil, kind)
	e = binary.BigEndian.AppendUint16(e, uint16(len(data)))

	return append(e, data...)
}

func build(host string, before ...[]byte) []byte {
	var extensions []byte
	for _, e := range before {
		extensions = append(extensions, e...)
	}

	if host != "" {
		name := []byte(host)

		list := binary.BigEndian.AppendUint16(nil, uint16(len(name)+3))
		list = append(list, nameTypeHost)
		list = binary.BigEndian.AppendUint16(list, uint16(len(name)))
		list = append(list, name...)

		extensions = append(extensions, ext(extServerName, list)...)
	}

	block := binary.BigEndian.AppendUint16(nil, uint16(len(extensions)))
	block = append(block, extensions...)

	body := []byte{0x03, 0x03}
	body = append(body, bytes.Repeat([]byte{0xab}, 32)...)
	body = append(body, 0x00)
	body = append(body, 0x00, 0x02, 0x13, 0x01)
	body = append(body, 0x01, 0x00)
	body = append(body, block...)

	handshake := []byte{typeClientHello, 0x00, 0x00, 0x00}
	handshake[1] = byte(len(body) >> 16)
	handshake[2] = byte(len(body) >> 8)
	handshake[3] = byte(len(body))
	handshake = append(handshake, body...)

	record := []byte{recordHandshake, 0x03, 0x01, 0x00, 0x00}
	binary.BigEndian.PutUint16(record[3:5], uint16(len(handshake)))

	return append(record, handshake...)
}

func TestParseFields(t *testing.T) {
	payload := build("gateway.discord.gg")

	h, err := Parse(payload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if h.RecordLen != len(payload)-recordHeaderLen {
		t.Errorf("RecordLen = %d, want %d", h.RecordLen, len(payload)-recordHeaderLen)
	}
	if len(h.Record) != len(payload) {
		t.Errorf("Record = %d bytes, want %d", len(h.Record), len(payload))
	}
	if len(h.Body) != len(payload)-recordHeaderLen-handshakeHeaderLen {
		t.Errorf("Body = %d bytes, want %d", len(h.Body), len(payload)-9)
	}
	if !bytes.Contains(h.Body, []byte("gateway.discord.gg")) {
		t.Error("Body carries no host name")
	}
}

func TestParseAcceptsRecordVersions(t *testing.T) {
	cases := []struct {
		name  string
		major byte
		minor byte
	}{
		{"tls 1.0 record", 0x03, 0x01},
		{"tls 1.2 record", 0x03, 0x03},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			payload := build("example.com")
			payload[1] = c.major
			payload[2] = c.minor

			if _, err := Parse(payload); err != nil {
				t.Errorf("Parse: %v", err)
			}
		})
	}
}

func TestParseSplitAcrossSegments(t *testing.T) {
	payload := build("updates.discord.com")
	cut := len(payload) - 10

	h, err := Parse(payload[:cut])
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if h.RecordLen != len(payload)-recordHeaderLen {
		t.Errorf("RecordLen = %d, want the declared %d", h.RecordLen, len(payload)-recordHeaderLen)
	}
	if len(h.Record) != cut {
		t.Errorf("Record = %d bytes, want the captured %d", len(h.Record), cut)
	}
	if len(h.Body) != cut-recordHeaderLen-handshakeHeaderLen {
		t.Errorf("Body = %d bytes, want %d", len(h.Body), cut-9)
	}
}

func TestParseErrors(t *testing.T) {
	applicationData := build("example.com")
	applicationData[0] = 0x17

	serverHello := build("example.com")
	serverHello[recordHeaderLen] = 0x02

	cases := []struct {
		name    string
		payload []byte
		want    error
	}{
		{"empty", nil, ErrTooShort},
		{"one byte short of both headers", build("example.com")[:8], ErrTooShort},
		{"application data", applicationData, ErrNotHandshake},
		{"server hello", serverHello, ErrNotClientHello},
		{"record declares nothing", []byte{0x16, 0x03, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00}, ErrTooShort},
		{"record declares less than a handshake header", []byte{0x16, 0x03, 0x01, 0x00, 0x03, 0x01, 0x00, 0x00, 0x00}, ErrTooShort},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.payload)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestBodyAliasesPayload(t *testing.T) {
	payload := build("example.com")

	h, err := Parse(payload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	h.Body[0] = 0xff

	if payload[9] != 0xff {
		t.Errorf("payload[9] = %#x, want the write to go through", payload[9])
	}
}
