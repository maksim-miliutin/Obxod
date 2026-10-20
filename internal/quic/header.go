package quic

import "errors"

var (
	ErrNotLongHeader = errors.New("quic: not a long header packet")
	ErrNotInitial    = errors.New("quic: long header is not a version 1 initial")
	ErrShort         = errors.New("quic: packet ends inside its header")
	ErrBadVarint     = errors.New("quic: truncated variable-length integer")
)

type Initial struct {
	Version  uint32
	DCID     []byte
	SCID     []byte
	Token    []byte
	Length   int
	PNOffset int
}

func ParseInitial(p []byte) (Initial, error) {
	if len(p) < 5 {
		return Initial{}, ErrShort
	}

	if p[0]&0x80 == 0 {
		return Initial{}, ErrNotLongHeader
	}

	// Form bit and long type (0xb0) sit outside header protection, so an Initial
	// (long form, type 00) is told from the other long packets before unmasking.
	if p[0]&0xb0 != 0x80 {
		return Initial{}, ErrNotInitial
	}

	version := uint32(p[1])<<24 | uint32(p[2])<<16 | uint32(p[3])<<8 | uint32(p[4])
	if version != 1 {
		return Initial{}, ErrNotInitial
	}

	at := 5

	dcid, at, ok := cid(p, at)
	if !ok {
		return Initial{}, ErrShort
	}

	scid, at, ok := cid(p, at)
	if !ok {
		return Initial{}, ErrShort
	}

	tokenLen, at, ok := varint(p, at)
	if !ok {
		return Initial{}, ErrBadVarint
	}

	if tokenLen > len(p)-at {
		return Initial{}, ErrShort
	}

	token := p[at : at+tokenLen]
	at += tokenLen

	length, at, ok := varint(p, at)
	if !ok {
		return Initial{}, ErrBadVarint
	}

	if length > len(p)-at {
		return Initial{}, ErrShort
	}

	return Initial{
		Version:  version,
		DCID:     dcid,
		SCID:     scid,
		Token:    token,
		Length:   length,
		PNOffset: at,
	}, nil
}

func cid(p []byte, at int) ([]byte, int, bool) {
	if at >= len(p) {
		return nil, at, false
	}

	n := int(p[at])
	at++

	if n > 20 || n > len(p)-at {
		return nil, at, false
	}

	return p[at : at+n], at + n, true
}

func varint(p []byte, at int) (int, int, bool) {
	if at >= len(p) {
		return 0, at, false
	}

	size := 1 << (p[at] >> 6)
	if size > len(p)-at {
		return 0, at, false
	}

	value := int(p[at] & 0x3f)
	for i := 1; i < size; i++ {
		value = value<<8 | int(p[at+i])
	}

	return value, at + size, true
}
