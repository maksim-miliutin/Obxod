package voice

import "encoding/binary"

// These ports carry games and audio too, so the payload decides, not the port.
const (
	discoveryLen  = 74
	discoveryAsk  = 1
	discoveryBody = 70
	addressAt     = 8
	addressLen    = 64

	stunHeaderLen = 20
	stunCookie    = 0x2112a442
)

func Discovery(payload []byte) bool {
	if len(payload) != discoveryLen {
		return false
	}

	if binary.BigEndian.Uint16(payload[0:2]) != discoveryAsk {
		return false
	}

	if binary.BigEndian.Uint16(payload[2:4]) != discoveryBody {
		return false
	}

	for _, b := range payload[addressAt : addressAt+addressLen] {
		if b != 0 {
			return false
		}
	}

	return true
}

func Stun(payload []byte) bool {
	if len(payload) < stunHeaderLen {
		return false
	}

	// The two top bits of a stun message are zero, and its length counts whole
	// words, which is what tells it apart from anything else on these ports.
	if payload[0]&0xc0 != 0 || payload[3]&3 != 0 {
		return false
	}

	if binary.BigEndian.Uint32(payload[4:8]) != stunCookie {
		return false
	}

	return int(binary.BigEndian.Uint16(payload[2:4])) <= len(payload)-stunHeaderLen
}

func Ours(payload []byte) bool {
	return Discovery(payload) || Stun(payload)
}
