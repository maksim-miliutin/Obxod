package forge

import (
	"errors"

	"obxod/internal/clienthello"
	"obxod/internal/ip"
	"obxod/internal/seal"
	"obxod/internal/tcp"
)

const ttlAt = 8

var (
	ErrNotTCP    = errors.New("forge: only tcp packets are copied")
	ErrNoPayload = errors.New("forge: the packet carries nothing to copy")

	ErrNoRecording = errors.New("forge: a recorded hello is needed, give one with -fake")
)

type Recipe struct {
	TTL      uint8  // hops the copy may live; zero keeps whatever the original had
	SeqDelta int32  // added to the sequence number so the server drops the copy; zero leaves it
	AckDelta int32  // added to the acknowledgement number, usually backwards; zero leaves it
	Stale    uint32 // taken off the timestamp so the server calls the copy old; zero leaves it
	BadSum   bool   // leave a wrong TCP checksum so the copy is dropped past the inspector

	// Must match the real name in length: a hello counts the name in three places.
	Name string
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

	copied, err := dressed(packet, segment, r.Name)
	if err != nil {
		return nil, err
	}

	if r.TTL != 0 {
		copied[ttlAt] = r.TTL
	}

	if err := seal.Shift(copied, r.SeqDelta, r.AckDelta); err != nil {
		return nil, err
	}

	if r.Stale != 0 {
		if err := seal.Stale(copied, r.Stale); err != nil {
			return nil, err
		}
	}

	if err := seal.Sums(copied, r.BadSum); err != nil {
		return nil, err
	}

	return copied, nil
}

// A name of another length moves six declared lengths inside the hello, so the
// packet is rebuilt rather than painted over.
func dressed(packet []byte, segment tcp.Header, name string) ([]byte, error) {
	if name == "" {
		out := make([]byte, len(packet))
		copy(out, packet)

		return out, nil
	}

	parsed, err := clienthello.Parse(segment.Payload)
	if err != nil {
		return nil, err
	}

	renamed, err := parsed.Renamed(name)
	if err != nil {
		return nil, err
	}

	return seal.Remade(packet, renamed, segment.Seq)
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

	made, err := seal.Remade(packet, recorded, segment.Seq)
	if err != nil {
		return nil, err
	}

	if r.TTL != 0 {
		made[ttlAt] = r.TTL
	}

	if err := seal.Shift(made, r.SeqDelta, r.AckDelta); err != nil {
		return nil, err
	}

	if r.Stale != 0 {
		if err := seal.Stale(made, r.Stale); err != nil {
			return nil, err
		}
	}

	if err := seal.Sums(made, r.BadSum); err != nil {
		return nil, err
	}

	return made, nil
}
