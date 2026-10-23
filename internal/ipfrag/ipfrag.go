package ipfrag

import (
	"encoding/binary"
	"errors"

	"obxod/internal/checksum"
	"obxod/internal/ip"
)

var (
	ErrNoPayload  = errors.New("ipfrag: the packet carries nothing to fragment")
	ErrBadPoint   = errors.New("ipfrag: the split point lies outside the payload")
	ErrNotAligned = errors.New("ipfrag: a fragment offset counts eight-byte units, so the point must be a multiple of 8")
)

const (
	totalLenAt = 2
	flagsAt    = 6
	checksumAt = 10
	flagMore   = 0x2000 // more fragments follow
	offsetMask = 0x1fff // the low 13 bits hold the offset, in 8-byte units
)

// At splits one ipv4 packet into two fragments at point bytes into the ip payload.
// The receiver joins them by offset, so the tcp segment inside is whole only once
// reassembled: an inspector reading each packet alone finds no tcp header in the
// second and no full hello in either. point counts from the start of the ip payload
// and must be a multiple of 8, the unit an ip fragment offset is measured in.
func At(packet []byte, point int) ([]byte, []byte, error) {
	outer, err := ip.Parse(packet)
	if err != nil {
		return nil, nil, err
	}

	if len(outer.Payload) == 0 {
		return nil, nil, ErrNoPayload
	}

	if point <= 0 || point >= len(outer.Payload) {
		return nil, nil, ErrBadPoint
	}

	if point%8 != 0 {
		return nil, nil, ErrNotAligned
	}

	head := outer.HeaderLen
	field := binary.BigEndian.Uint16(packet[flagsAt : flagsAt+2])
	base := field & offsetMask // the offset this packet already sits at, if it is itself a fragment
	tail := field & flagMore   // whether fragments follow beyond the piece we are splitting

	first := fragment(packet, head, outer.Payload[:point], base, flagMore)
	second := fragment(packet, head, outer.Payload[point:], base+uint16(point/8), tail)

	return first, second, nil
}

// fragment builds one fragment: the original header with its total length, offset and
// more-fragments bit rewritten for this piece, and a fresh header checksum. The
// don't-fragment bit is cleared, since fragmenting is the whole point.
func fragment(packet []byte, head int, piece []byte, offsetUnits uint16, more uint16) []byte {
	out := make([]byte, head+len(piece))
	copy(out, packet[:head])
	copy(out[head:], piece)

	binary.BigEndian.PutUint16(out[totalLenAt:totalLenAt+2], uint16(head+len(piece)))
	binary.BigEndian.PutUint16(out[flagsAt:flagsAt+2], more|(offsetUnits&offsetMask))

	binary.BigEndian.PutUint16(out[checksumAt:checksumAt+2], 0)
	binary.BigEndian.PutUint16(out[checksumAt:checksumAt+2], checksum.IPv4(out[:head]))

	return out
}
