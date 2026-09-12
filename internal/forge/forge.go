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
	tcpChecksumAt = 16
)

var (
	ErrNotTCP    = errors.New("forge: only tcp packets are copied")
	ErrNoPayload = errors.New("forge: the packet carries nothing to copy")
)

type Recipe struct {
	TTL uint8 // hops the copy may live; zero keeps whatever the original had
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
