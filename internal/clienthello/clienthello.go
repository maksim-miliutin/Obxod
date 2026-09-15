package clienthello

import (
	"encoding/binary"
	"errors"
)

const (
	recordHandshake = 0x16
	typeClientHello = 0x01
)

const (
	recordHeaderLen    = 5
	handshakeHeaderLen = 4
)

var (
	ErrTooShort       = errors.New("clienthello: payload shorter than a record header")
	ErrNotHandshake   = errors.New("clienthello: not a handshake record")
	ErrNotClientHello = errors.New("clienthello: handshake is not a client hello")
)

type Hello struct {
	RecordLen int    // bytes, as the record header declares, not as captured
	Record    []byte // record header and body, aliases payload
	Body      []byte // hello itself, past the handshake header, aliases payload

	payload []byte
}

func Parse(payload []byte) (Hello, error) {
	if len(payload) < recordHeaderLen+handshakeHeaderLen {
		return Hello{}, ErrTooShort
	}

	// A TLS 1.3 record still carries version 0x0301, so the field is worth no check.
	if payload[0] != recordHandshake {
		return Hello{}, ErrNotHandshake
	}

	if payload[recordHeaderLen] != typeClientHello {
		return Hello{}, ErrNotClientHello
	}

	recordLen := int(binary.BigEndian.Uint16(payload[3:5]))

	// A record shorter than the handshake header cannot hold a hello, and clamping
	// its end below the body start would slice backwards.
	if recordLen < handshakeHeaderLen {
		return Hello{}, ErrTooShort
	}

	// A hello split across segments declares more than this segment carries.
	recordEnd := recordHeaderLen + recordLen
	if recordEnd > len(payload) {
		recordEnd = len(payload)
	}

	bodyLen := int(payload[6])<<16 | int(payload[7])<<8 | int(payload[8])
	bodyEnd := recordHeaderLen + handshakeHeaderLen + bodyLen
	if bodyEnd > recordEnd {
		bodyEnd = recordEnd
	}

	h := Hello{
		payload:   payload,
		RecordLen: recordLen,
		Record:    payload[:recordEnd],
		Body:      payload[recordHeaderLen+handshakeHeaderLen : bodyEnd],
	}

	return h, nil
}
