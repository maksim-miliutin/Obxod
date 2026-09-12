package ip

import (
	"encoding/binary"
	"errors"
)

const (
	ProtocolTCP = 6
	ProtocolUDP = 17
)

const minHeaderLen = 20

var (
	ErrTooShort     = errors.New("ip: packet shorter than its header")
	ErrNotIPv4      = errors.New("ip: not version 4")
	ErrBadHeaderLen = errors.New("ip: header length below 20 bytes")
)

type Header struct {
	HeaderLen int // bytes
	TotalLen  int // bytes, as the field says, not as captured
	TTL       uint8
	Protocol  uint8
	Src       [4]byte
	Dst       [4]byte
	Payload   []byte // aliases packet: writes through it change the packet
}

func Parse(packet []byte) (Header, error) {
	if len(packet) < minHeaderLen {
		return Header{}, ErrTooShort
	}

	if packet[0]>>4 != 4 {
		return Header{}, ErrNotIPv4
	}

	headerLen := int(packet[0]&0x0f) * 4
	if headerLen < minHeaderLen {
		return Header{}, ErrBadHeaderLen
	}

	if headerLen > len(packet) {
		return Header{}, ErrTooShort
	}

	h := Header{
		HeaderLen: headerLen,
		TotalLen:  int(binary.BigEndian.Uint16(packet[2:4])),
		TTL:       packet[8],
		Protocol:  packet[9],
	}
	copy(h.Src[:], packet[12:16])
	copy(h.Dst[:], packet[16:20])

	// Windows offload leaves the length field inconsistent with what was captured.
	end := h.TotalLen
	if end < headerLen || end > len(packet) {
		end = len(packet)
	}

	h.Payload = packet[headerLen:end]

	return h, nil
}
