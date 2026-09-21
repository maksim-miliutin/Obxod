package voice

import "encoding/binary"

// What discord sends first on a voice port, and what answers a stun server. The
// ports carry games and everything else too, so the payload decides, not the port.
const (
	discoveryLen  = 74
	discoveryAsk  = 1
	discoveryBody = 70
	addressAt     = 8
	addressLen    = 64

	stunHeaderLen = 20
	stunCookie    = 0x2112a442
)

// Discovery reports the datagram discord sends to learn its own address, which
// opens a voice connection. Its address field is still all zeroes on the way out.
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

// Stun reports a stun message, which is how a call finds its way through.
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

// Ours reports a datagram worth putting a fake in front of.
func Ours(payload []byte) bool {
	return Discovery(payload) || Stun(payload)
}
