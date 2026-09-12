package tcp

import (
	"encoding/binary"
	"errors"
)

const (
	FlagFIN = 1 << 0
	FlagSYN = 1 << 1
	FlagRST = 1 << 2
	FlagPSH = 1 << 3
	FlagACK = 1 << 4
	FlagURG = 1 << 5
)

const minHeaderLen = 20

var (
	ErrTooShort      = errors.New("tcp: segment shorter than its header")
	ErrBadDataOffset = errors.New("tcp: data offset below 20 bytes")
)

type Header struct {
	SrcPort   uint16
	DstPort   uint16
	Seq       uint32
	HeaderLen int    // bytes, data offset * 4
	Flags     uint8  // FlagFIN and friends
	Payload   []byte // aliases segment: writes through it change the segment
}

func Parse(segment []byte) (Header, error) {
	if len(segment) < minHeaderLen {
		return Header{}, ErrTooShort
	}

	headerLen := int(segment[12]>>4) * 4
	if headerLen < minHeaderLen {
		return Header{}, ErrBadDataOffset
	}

	if headerLen > len(segment) {
		return Header{}, ErrTooShort
	}

	h := Header{
		SrcPort:   binary.BigEndian.Uint16(segment[0:2]),
		DstPort:   binary.BigEndian.Uint16(segment[2:4]),
		Seq:       binary.BigEndian.Uint32(segment[4:8]),
		HeaderLen: headerLen,
		Flags:     segment[13],
	}
	h.Payload = segment[headerLen:]

	return h, nil
}
