package hello

import (
	"obxod/internal/clienthello"
	"obxod/internal/ip"
	"obxod/internal/tcp"
)

type Outgoing struct {
	Host string
}

// Found reports a TLS client hello leaving for port 443 and the site it names.
// A packet that is not such a hello yields ok false, not an error: most packets
// on the wire are not client hellos, and that is the ordinary case, not a fault.
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

	if segment.DstPort != 443 {
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

	return Outgoing{Host: name.Host}, true
}
