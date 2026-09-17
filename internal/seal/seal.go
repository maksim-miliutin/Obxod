package seal

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
	tcpAckAt      = 8
	tcpChecksumAt = 16
)

var ErrNotTCP = errors.New("seal: only tcp packets are sealed")

// Sums writes both checksums over a packet whose bytes are otherwise final.
func Sums(packet []byte, badSum bool) error {
	outer, err := ip.Parse(packet)
	if err != nil {
		return err
	}

	if outer.Protocol != ip.ProtocolTCP {
		return ErrNotTCP
	}

	header := packet[:outer.HeaderLen]
	binary.BigEndian.PutUint16(header[ipChecksumAt:ipChecksumAt+2], checksum.IPv4(header))

	// Offload can hand us more bytes than the header claims; those trailing bytes
	// belong to no segment and must stay out of the sum.
	segment := packet[outer.HeaderLen : outer.HeaderLen+len(outer.Payload)]
	sum := checksum.TCP(outer.Src, outer.Dst, segment)

	// Flip the right sum rather than skip it: offload may leave the field
	// uncomputed, and a "wrong" value left there could accidentally be right.
	if badSum {
		sum = ^sum
	}

	binary.BigEndian.PutUint16(segment[tcpChecksumAt:tcpChecksumAt+2], sum)

	return nil
}

// Remade keeps the headers of packet, puts payload where the old one sat and
// numbers it seq, then seals what it built.
func Remade(packet []byte, payload []byte, seq uint32) ([]byte, error) {
	outer, err := ip.Parse(packet)
	if err != nil {
		return nil, err
	}

	if outer.Protocol != ip.ProtocolTCP {
		return nil, ErrNotTCP
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		return nil, err
	}

	headers := outer.HeaderLen + segment.HeaderLen

	out := make([]byte, headers+len(payload))
	copy(out, packet[:headers])
	copy(out[headers:], payload)

	binary.BigEndian.PutUint16(out[totalLenAt:totalLenAt+2], uint16(len(out)))

	// Move the number by what went before, or the stream has a hole.
	binary.BigEndian.PutUint32(out[outer.HeaderLen+tcpSeqAt:outer.HeaderLen+tcpSeqAt+4], seq)

	if err := Sums(out, false); err != nil {
		return nil, err
	}

	return out, nil
}

// Shift moves the numbers of a tcp segment by as much as asked, wrapping as tcp
// does. Sums has to follow: both numbers feed the checksum.
func Shift(packet []byte, seq int32, ack int32) error {
	outer, err := ip.Parse(packet)
	if err != nil {
		return err
	}

	if outer.Protocol != ip.ProtocolTCP {
		return ErrNotTCP
	}

	segment := packet[outer.HeaderLen:]

	if seq != 0 {
		was := binary.BigEndian.Uint32(segment[tcpSeqAt : tcpSeqAt+4])
		binary.BigEndian.PutUint32(segment[tcpSeqAt:tcpSeqAt+4], was+uint32(seq))
	}

	if ack != 0 {
		was := binary.BigEndian.Uint32(segment[tcpAckAt : tcpAckAt+4])
		binary.BigEndian.PutUint32(segment[tcpAckAt:tcpAckAt+4], was+uint32(ack))
	}

	return nil
}

const (
	optionEnd       = 0
	optionNop       = 1
	optionTimestamp = 8
	timestampLen    = 10
)

var (
	ErrNoTimestamp = errors.New("seal: the packet carries no timestamp option to age")
	ErrBadOptions  = errors.New("seal: a tcp option runs past the header")
)

// Stale moves the timestamp of a tcp segment back, so the server rejects the copy
// as an old duplicate while its sequence number keeps it inside the stream.
func Stale(packet []byte, back uint32) error {
	outer, err := ip.Parse(packet)
	if err != nil {
		return err
	}

	if outer.Protocol != ip.ProtocolTCP {
		return ErrNotTCP
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		return err
	}

	options := packet[outer.HeaderLen+20 : outer.HeaderLen+segment.HeaderLen]

	for at := 0; at < len(options); {
		switch options[at] {
		case optionEnd:
			return ErrNoTimestamp
		case optionNop:
			at++

			continue
		}

		if at+1 >= len(options) {
			return ErrBadOptions
		}

		size := int(options[at+1])
		if size < 2 || at+size > len(options) {
			return ErrBadOptions
		}

		if options[at] == optionTimestamp && size == timestampLen {
			value := binary.BigEndian.Uint32(options[at+2 : at+6])
			binary.BigEndian.PutUint32(options[at+2:at+6], value-back)

			return nil
		}

		at += size
	}

	return ErrNoTimestamp
}
