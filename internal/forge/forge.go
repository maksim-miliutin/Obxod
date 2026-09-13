package forge

import (
	"encoding/binary"
	"errors"

	"obxod/internal/checksum"
	"obxod/internal/ip"
	"obxod/internal/tcp"
)

const (
	ttlAt         = 8
	ipChecksumAt  = 10
	tcpSeqAt      = 4
	tcpChecksumAt = 16
)

var (
	ErrNotTCP    = errors.New("forge: only tcp packets are copied")
	ErrNoPayload = errors.New("forge: the packet carries nothing to copy")
)

type Recipe struct {
	TTL      uint8  // hops the copy may live; zero keeps whatever the original had
	SeqDelta uint32 // added to the sequence number so the server drops the copy; zero leaves it
}

func Copy(packet []byte, r Recipe) ([]byte, error) {
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

	if len(segment.Payload) == 0 {
		return nil, ErrNoPayload
	}

	copied := make([]byte, len(packet))
	copy(copied, packet)

	if r.TTL != 0 {
		copied[ttlAt] = r.TTL
	}

	// The sequence number feeds the TCP checksum, so damage it before sealing,
	// or the sum would cover the old number and the copy would die anywhere, not
	// only at the server that rejects the wrong sequence.
	if r.SeqDelta != 0 {
		segment := copied[outer.HeaderLen:]
		seq := binary.BigEndian.Uint32(segment[tcpSeqAt : tcpSeqAt+4])
		binary.BigEndian.PutUint32(segment[tcpSeqAt:tcpSeqAt+4], seq+r.SeqDelta)
	}

	seal(copied, outer)

	return copied, nil
}

func seal(packet []byte, outer ip.Header) {
	header := packet[:outer.HeaderLen]
	binary.BigEndian.PutUint16(header[ipChecksumAt:ipChecksumAt+2], checksum.IPv4(header))

	// Offload can hand us more bytes than the header claims; those trailing bytes
	// belong to no segment and must stay out of the sum.
	segment := packet[outer.HeaderLen : outer.HeaderLen+len(outer.Payload)]
	binary.BigEndian.PutUint16(segment[tcpChecksumAt:tcpChecksumAt+2], checksum.TCP(outer.Src, outer.Dst, segment))
}
