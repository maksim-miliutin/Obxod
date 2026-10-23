package quic

import "encoding/binary"

// IsInitial reports a client's opening QUIC packet: a version 1 long-header
// Initial. Only the handshake's start is worth a decoy; the rest of a connection
// just passes. On the wire the first byte is 0xc0..0xcf because header protection
// scrambles its low nibble but leaves the long-header high one.
func IsInitial(payload []byte) bool {
	if len(payload) < 5 {
		return false
	}

	if payload[0]&0xf0 != 0xc0 {
		return false
	}

	return binary.BigEndian.Uint32(payload[1:5]) == 1
}
