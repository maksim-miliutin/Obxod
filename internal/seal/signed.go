package seal

import (
	"encoding/binary"
	"errors"
	"math/rand/v2"

	"obxod/internal/ip"
	"obxod/internal/tcp"
)

const (
	tcpOffsetAt  = 12
	maxTCPHeader = 60 // the data offset counts words and has four bits

	signatureKind = 19
	signatureLen  = 18
	grownBy       = 20 // the option plus two nops, because the header counts words
)

var ErrNoRoomToSign = errors.New("seal: the tcp header has no room left for a signature")

// A server that never agreed to md5 signatures must drop a segment that carries
// one, by rfc 2385, while an inspector reading the payload has no reason to care.
func Signed(packet []byte) ([]byte, error) {
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

	grown := segment.HeaderLen + grownBy
	if grown > maxTCPHeader {
		return nil, ErrNoRoomToSign
	}

	out := make([]byte, 0, len(packet)+grownBy)
	out = append(out, packet[:outer.HeaderLen+segment.HeaderLen]...)
	out = append(out, signature()...)
	out = append(out, segment.Payload...)

	at := outer.HeaderLen + tcpOffsetAt
	out[at] = byte(grown/4)<<4 | out[at]&0x0f

	binary.BigEndian.PutUint16(out[ipLengthAt:ipLengthAt+2], uint16(len(out)))

	if err := Sums(out, false); err != nil {
		return nil, err
	}

	return out, nil
}

// Sixteen bytes nobody can check, padded to a whole word. A signature that never
// changes is one an inspector can learn.
func signature() []byte {
	out := make([]byte, grownBy)
	out[0] = signatureKind
	out[1] = signatureLen

	for i := 2; i < signatureLen; i++ {
		out[i] = byte(rand.IntN(256))
	}

	out[signatureLen] = optionNop
	out[signatureLen+1] = optionNop

	return out
}
