package filter

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrEmptyRange = errors.New("filter: port range ends before it starts")
	ErrZeroPort   = errors.New("filter: port 0 is not a port")
	ErrNoPorts    = errors.New("filter: nothing to catch")
)

// Ports says where to look. Discord alone speaks TLS on 443 as well as 2053,
// 2083, 2087, 2096 and 8443, so the list is not a constant.
type Ports struct {
	TCP   []uint16
	Voice []PortRange
	QUIC  bool
}

type PortRange struct {
	From uint16
	To   uint16
}

func Outbound(p Ports) (string, error) {
	var clauses []string

	for _, port := range p.TCP {
		if port == 0 {
			return "", ErrZeroPort
		}

		clauses = append(clauses, fmt.Sprintf("(tcp.DstPort == %d and tcp.PayloadLength > 0)", port))
	}

	if p.QUIC {
		clauses = append(clauses, "udp.DstPort == 443")
	}

	for _, r := range p.Voice {
		if r.From == 0 || r.To == 0 {
			return "", ErrZeroPort
		}

		if r.From > r.To {
			return "", ErrEmptyRange
		}

		if r.From == r.To {
			clauses = append(clauses, fmt.Sprintf("udp.DstPort == %d", r.From))

			continue
		}

		clauses = append(clauses, fmt.Sprintf("(udp.DstPort >= %d and udp.DstPort <= %d)", r.From, r.To))
	}

	// Windows counts any packet this machine sends to itself as loopback, whatever the
	// address, and impostor marks packets another driver injected, which would loop back.
	if len(clauses) == 0 {
		return "", ErrNoPorts
	}

	head := "outbound and not loopback and not impostor and ip and "

	return head + "(" + strings.Join(clauses, " or ") + ")", nil
}

// Meant for a handle opened in sniffing mode: replies are watched, never held up.
// A fin carries no payload, so it has to be named on its own or a connection
// closing politely would look exactly like one killed in silence.
func Replies(tcp []uint16) (string, error) {
	var ports []string

	for _, port := range tcp {
		if port == 0 {
			return "", ErrZeroPort
		}

		ports = append(ports, fmt.Sprintf("tcp.SrcPort == %d", port))
	}

	if len(ports) == 0 {
		return "", ErrNoPorts
	}

	return "inbound and ip and (" + strings.Join(ports, " or ") + ") and (tcp.Rst or tcp.Fin or tcp.PayloadLength > 0)", nil
}
