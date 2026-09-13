package cut

import (
	"encoding/binary"
	"errors"

	"obxod/internal/checksum"
	"obxod/internal/ip"
	"obxod/internal/tcp"
)

const (
	totalLenAt    = 2
	ipChecksumAt  = 10
	tcpSeqAt      = 4
	tcpChecksumAt = 16
)

var (
	ErrNotTCP     = errors.New("cut: only tcp packets are split")
	ErrNoPayload  = errors.New("cut: the packet carries nothing to split")
	ErrBadPoint   = errors.New("cut: the split point lies outside the payload")
	ErrNoRoomLeft = errors.New("cut: a split needs at least one byte on each side")
)

// At splits the packet in two at the given offset into the TCP payload. Both
// halves are real data, not copies: together they carry exactly what the
// original carried, so the server rebuilds the same stream.
func At(packet []byte, point int) ([]byte, []byte, error) {
	outer, err := ip.Parse(packet)
	if err != nil {
		return nil, nil, err
	}

	if outer.Protocol != ip.ProtocolTCP {
		return nil, nil, ErrNotTCP
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		return nil, nil, err
	}

	if len(segment.Payload) == 0 {
		return nil, nil, ErrNoPayload
	}

	if point < 0 || point > len(segment.Payload) {
		return nil, nil, ErrBadPoint
	}

	if point == 0 || point == len(segment.Payload) {
		return nil, nil, ErrNoRoomLeft
	}

	headers := outer.HeaderLen + segment.HeaderLen

	first := build(packet, headers, segment.Payload[:point], segment.Seq, outer)
	second := build(packet, headers, segment.Payload[point:], segment.Seq+uint32(point), outer)

	return first, second, nil
}

func build(packet []byte, headers int, payload []byte, seq uint32, outer ip.Header) []byte {
	out := make([]byte, headers+len(payload))
	copy(out, packet[:headers])
	copy(out[headers:], payload)

	binary.BigEndian.PutUint16(out[totalLenAt:totalLenAt+2], uint16(len(out)))

	segment := out[outer.HeaderLen:]

	// The second half starts further along the stream, so its sequence number moves
	// by as many bytes as the first half carried, or the server cannot rebuild the hello.
	binary.BigEndian.PutUint32(segment[tcpSeqAt:tcpSeqAt+4], seq)

	header := out[:outer.HeaderLen]
	binary.BigEndian.PutUint16(header[ipChecksumAt:ipChecksumAt+2], checksum.IPv4(header))
	binary.BigEndian.PutUint16(segment[tcpChecksumAt:tcpChecksumAt+2], checksum.TCP(outer.Src, outer.Dst, segment))

	return out
}
