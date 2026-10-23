package cut

import (
	"errors"
	"sort"

	"obxod/internal/ip"
	"obxod/internal/seal"
	"obxod/internal/tcp"
)

var (
	ErrNotTCP       = errors.New("cut: only tcp packets are split")
	ErrNoPayload    = errors.New("cut: the packet carries nothing to split")
	ErrBadPoint     = errors.New("cut: the split point lies outside the payload")
	ErrNoRoomLeft   = errors.New("cut: a split needs at least one byte on each side")
	ErrNoPattern    = errors.New("cut: an overlap needs a recorded hello to lay over")
	ErrTooFewPieces = errors.New("cut: a scatter needs at least two pieces")
)

// Pieces breaks the payload into n parts, each sealed at its own sequence number,
// so sending them in any order still reassembles whole on the server. The parts
// come back in stream order; the caller is what sends them scrambled.
func Pieces(packet []byte, n int) ([][]byte, error) {
	if n < 2 {
		return nil, ErrTooFewPieces
	}

	_, segment, err := layers(packet)
	if err != nil {
		return nil, err
	}

	total := len(segment.Payload)
	if n > total {
		n = total
	}

	out := make([][]byte, 0, n)

	for i := 0; i < n; i++ {
		from := i * total / n
		to := (i + 1) * total / n

		piece, err := seal.Remade(packet, segment.Payload[from:to], segment.Seq+uint32(from))
		if err != nil {
			return nil, err
		}

		out = append(out, piece)
	}

	return out, nil
}

// AtPositions breaks the payload at the given offsets into one more piece than
// there are offsets, each sealed at its own sequence number so the server puts them
// back whole whatever order they arrive in. The offsets are sorted and deduplicated,
// and any at or past the edges are refused, since every piece must carry a byte.
func AtPositions(packet []byte, points []int) ([][]byte, error) {
	_, segment, err := layers(packet)
	if err != nil {
		return nil, err
	}

	bounds, err := interior(points, len(segment.Payload))
	if err != nil {
		return nil, err
	}

	out := make([][]byte, 0, len(bounds)+1)
	from := 0

	for _, to := range bounds {
		piece, err := seal.Remade(packet, segment.Payload[from:to], segment.Seq+uint32(from))
		if err != nil {
			return nil, err
		}

		out = append(out, piece)
		from = to
	}

	last, err := seal.Remade(packet, segment.Payload[from:], segment.Seq+uint32(from))
	if err != nil {
		return nil, err
	}

	return append(out, last), nil
}

func interior(points []int, total int) ([]int, error) {
	seen := make(map[int]bool, len(points))

	var kept []int

	for _, p := range points {
		if p <= 0 || p >= total {
			return nil, ErrBadPoint
		}

		if seen[p] {
			continue
		}

		seen[p] = true
		kept = append(kept, p)
	}

	if len(kept) == 0 {
		return nil, ErrNoRoomLeft
	}

	sort.Ints(kept)

	return kept, nil
}

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
