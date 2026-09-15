package forge

import (
	"encoding/binary"
	"errors"

	"obxod/internal/ip"
	"obxod/internal/seal"
	"obxod/internal/tcp"
)

const (
	ttlAt    = 8
	tcpSeqAt = 4
)

var (
	ErrNotTCP    = errors.New("forge: only tcp packets are copied")
	ErrNoPayload = errors.New("forge: the packet carries nothing to copy")
	ErrNameSpace = errors.New("forge: the decoy name does not fit where the real one sits")

	ErrNoRecording = errors.New("forge: a recorded hello is needed, give one with -fake")
)

type Recipe struct {
	TTL      uint8  // hops the copy may live; zero keeps whatever the original had
	SeqDelta uint32 // added to the sequence number so the server drops the copy; zero leaves it
	BadSum   bool   // leave a wrong TCP checksum so the copy is dropped past the inspector

	// Must match the real name in length: a hello counts the name in three places.
	Name   string
	NameAt int // where the real name starts inside the TCP payload
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

	// Before sealing: the number feeds the checksum.
	if r.SeqDelta != 0 {
		segment := copied[outer.HeaderLen:]
		seq := binary.BigEndian.Uint32(segment[tcpSeqAt : tcpSeqAt+4])
		binary.BigEndian.PutUint32(segment[tcpSeqAt:tcpSeqAt+4], seq+r.SeqDelta)
	}

	if r.Name != "" {
		payloadAt := outer.HeaderLen + segment.HeaderLen

		if r.NameAt < 0 || r.NameAt+len(r.Name) > len(segment.Payload) {
			return nil, ErrNameSpace
		}

		copy(copied[payloadAt+r.NameAt:], r.Name)
	}

	if err := seal.Sums(copied, r.BadSum); err != nil {
		return nil, err
	}

	return copied, nil
}

// Instead builds a fake out of recorded bytes: the headers of the real packet,
// somebody else's hello where the real one sat.
func Instead(packet []byte, recorded []byte, r Recipe) ([]byte, error) {
	if len(recorded) == 0 {
		return nil, ErrNoRecording
	}

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

	made, err := seal.Remade(packet, recorded, segment.Seq+r.SeqDelta)
	if err != nil {
		return nil, err
	}

	if r.TTL == 0 && !r.BadSum {
		return made, nil
	}

	if r.TTL != 0 {
		made[ttlAt] = r.TTL
	}

	if err := seal.Sums(made, r.BadSum); err != nil {
		return nil, err
	}

	return made, nil
}
