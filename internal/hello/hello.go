package hello

import (
	"obxod/internal/clienthello"
	"obxod/internal/ip"
	"obxod/internal/tcp"
)

type Outgoing struct {
	Host string

	NameStart int
	NameEnd   int

	SrcPort uint16
	Seq     uint32
}

// Not a hello is the ordinary case, not a fault, hence ok rather than an error.
func Found(packet []byte) (Outgoing, bool) {
	outer, err := ip.Parse(packet)
	if err != nil {
		return Outgoing{}, false
	}

	if outer.Protocol != ip.ProtocolTCP {
		return Outgoing{}, false
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		return Outgoing{}, false
	}

	parsed, err := clienthello.Parse(segment.Payload)
	if err != nil {
		return Outgoing{}, false
	}

	name, err := parsed.ServerName()
	if err != nil {
		return Outgoing{}, false
	}

	// A hello can declare a name of no bytes at all, and every way below assumes
	// there is something to cut at, swap out or write a rule for.
	if name.Host == "" {
		return Outgoing{}, false
	}

	return Outgoing{
		Host:      name.Host,
		NameStart: name.Offset,
		NameEnd:   name.Offset + len(name.Host),
		SrcPort:   segment.SrcPort,
		Seq:       segment.Seq,
	}, true
}
