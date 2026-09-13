package filter

import (
	"errors"
	"fmt"
	"strings"
)

const httpsPort = 443

var (
	ErrEmptyRange = errors.New("filter: port range ends before it starts")
	ErrZeroPort   = errors.New("filter: port 0 is not a port")
)

type PortRange struct {
	From uint16
	To   uint16
}

func Outbound(voice []PortRange, quic bool) (string, error) {
	clauses := []string{fmt.Sprintf("(tcp.DstPort == %d and tcp.PayloadLength > 0)", httpsPort)}

	if quic {
		clauses = append(clauses, fmt.Sprintf("udp.DstPort == %d", httpsPort))
	}

	for _, r := range voice {
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
	head := "outbound and not loopback and not impostor and ip and "

	return head + "(" + strings.Join(clauses, " or ") + ")", nil
}

// Meant for a handle opened in sniffing mode: replies are watched, never held up.
// A fin carries no payload, so it has to be named on its own or a connection
// closing politely would look exactly like one killed in silence.
func Replies() string {
	return fmt.Sprintf("inbound and ip and tcp.SrcPort == %d and (tcp.Rst or tcp.Fin or tcp.PayloadLength > 0)", httpsPort)
}
