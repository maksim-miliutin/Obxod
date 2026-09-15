package cut

import (
	"errors"

	"obxod/internal/ip"
	"obxod/internal/seal"
	"obxod/internal/tcp"
)

var (
	ErrNotTCP     = errors.New("cut: only tcp packets are split")
	ErrNoPayload  = errors.New("cut: the packet carries nothing to split")
	ErrBadPoint   = errors.New("cut: the split point lies outside the payload")
	ErrNoRoomLeft = errors.New("cut: a split needs at least one byte on each side")
	ErrNoPattern  = errors.New("cut: an overlap needs a recorded hello to lay over")
)

func Filler(pattern []byte, size int) []byte {
	if size <= 0 {
		return nil
	}

	out := make([]byte, size)
	copy(out, pattern)

	return out
}

// A server drops what lands behind its receive window, so the filler rides there
// and only the real bytes are taken.
func Overlap(packet []byte, pattern []byte, point int) ([]byte, []byte, error) {
	if len(pattern) == 0 {
		return nil, nil, ErrNoPattern
	}

	_, segment, err := layers(packet)
	if err != nil {
		return nil, nil, err
	}

	if point < 0 || point > len(segment.Payload) {
		return nil, nil, ErrBadPoint
	}

	if point == 0 || point == len(segment.Payload) {
		return nil, nil, ErrNoRoomLeft
	}

	ahead := make([]byte, 0, len(pattern)+point)
	ahead = append(ahead, pattern...)
	ahead = append(ahead, segment.Payload[:point]...)

	return both(packet, ahead, segment.Seq-uint32(len(pattern)), segment.Payload[point:], segment.Seq+uint32(point))
}

func layers(packet []byte) (ip.Header, tcp.Header, error) {
	outer, err := ip.Parse(packet)
	if err != nil {
		return ip.Header{}, tcp.Header{}, err
	}

	if outer.Protocol != ip.ProtocolTCP {
		return ip.Header{}, tcp.Header{}, ErrNotTCP
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		return ip.Header{}, tcp.Header{}, err
	}

	if len(segment.Payload) == 0 {
		return ip.Header{}, tcp.Header{}, ErrNoPayload
	}

	return outer, segment, nil
}

func At(packet []byte, point int) ([]byte, []byte, error) {
	_, segment, err := layers(packet)
	if err != nil {
		return nil, nil, err
	}

	if point < 0 || point > len(segment.Payload) {
		return nil, nil, ErrBadPoint
	}

	if point == 0 || point == len(segment.Payload) {
		return nil, nil, ErrNoRoomLeft
	}

	return both(packet, segment.Payload[:point], segment.Seq, segment.Payload[point:], segment.Seq+uint32(point))
}

func both(packet []byte, ahead []byte, aheadSeq uint32, rest []byte, restSeq uint32) ([]byte, []byte, error) {
	first, err := seal.Remade(packet, ahead, aheadSeq)
	if err != nil {
		return nil, nil, err
	}

	second, err := seal.Remade(packet, rest, restSeq)
	if err != nil {
		return nil, nil, err
	}

	return first, second, nil
}
