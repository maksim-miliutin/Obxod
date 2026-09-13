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
	ErrNoPattern  = errors.New("cut: an overlap needs a recorded hello to lay over")
)

// Filler makes the block that rides ahead of the stream: the recorded hello as
// far as it goes, then zeroes. The size is asked for separately because it has
// no reason to match the file, and a longer overlap reaches further back.
func Filler(pattern []byte, size int) []byte {
	if size <= 0 {
		return nil
	}

	out := make([]byte, size)
	copy(out, pattern)

	return out
}

// Overlap splits the packet like At, but sends the first half from a sequence
// number that many bytes earlier, filled with a hello recorded from another site.
//
// The server counts from where it left off, finds those bytes behind its window
// and drops them, keeping only the real ones. An inspector that just stacks
// payloads in the order they arrive reads the recorded hello instead, and stops
// caring about the connection.
//
// Both halves together still carry the whole original payload, so nothing the
// server rebuilds changes.
func Overlap(packet []byte, pattern []byte, point int) ([]byte, []byte, error) {
	if len(pattern) == 0 {
		return nil, nil, ErrNoPattern
	}

	outer, segment, err := layers(packet)
	if err != nil {
		return nil, nil, err
	}

	if point < 0 || point > len(segment.Payload) {
		return nil, nil, ErrBadPoint
	}

	if point == 0 || point == len(segment.Payload) {
		return nil, nil, ErrNoRoomLeft
	}

	headers := outer.HeaderLen + segment.HeaderLen

	ahead := make([]byte, 0, len(pattern)+point)
	ahead = append(ahead, pattern...)
	ahead = append(ahead, segment.Payload[:point]...)

	first := build(packet, headers, ahead, segment.Seq-uint32(len(pattern)), outer)
	second := build(packet, headers, segment.Payload[point:], segment.Seq+uint32(point), outer)

	return first, second, nil
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

// At splits the packet in two at the given offset into the TCP payload. Both
// halves are real data, not copies: together they carry exactly what the
// original carried, so the server rebuilds the same stream.
func At(packet []byte, point int) ([]byte, []byte, error) {
	outer, segment, err := layers(packet)
	if err != nil {
		return nil, nil, err
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
