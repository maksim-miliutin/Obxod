package udp

import (
	"encoding/binary"
	"errors"
)

const headerLen = 8

var ErrTooShort = errors.New("udp: datagram shorter than its header")

type Header struct {
	SrcPort uint16
	DstPort uint16
	Length  int    // bytes, header included, as the field says
	Payload []byte // aliases datagram: writes through it change the datagram
}

func Parse(datagram []byte) (Header, error) {
	if len(datagram) < headerLen {
		return Header{}, ErrTooShort
	}

	h := Header{
		SrcPort: binary.BigEndian.Uint16(datagram[0:2]),
		DstPort: binary.BigEndian.Uint16(datagram[2:4]),
		Length:  int(binary.BigEndian.Uint16(datagram[4:6])),
	}

	// Windows offload leaves the length field inconsistent with what was captured.
	end := h.Length
	if end < headerLen || end > len(datagram) {
		end = len(datagram)
	}

	h.Payload = datagram[headerLen:end]

	return h, nil
}
