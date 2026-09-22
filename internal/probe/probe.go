package probe

import (
	"encoding/binary"
	"net"
	"strings"
	"time"
)

// How a host behaves when reached without any bypass. It tells apart the shapes
// of blocking we have seen on the wire; it does not name who does the blocking,
// because a server that goes quiet after a hello looks the same whether a filter
// silenced it or the server itself is down.
type Verdict int

const (
	Clear     Verdict = iota // connected, handshake finished, an answer came back
	NoConnect                // tcp never opened: blocked by address, or the host is gone
	NoAnswer                 // tcp opened, but the handshake got no reply: the shape of an SNI reset
	Reset                    // the peer sent a reset during the handshake
)

func (v Verdict) String() string {
	switch v {
	case Clear:
		return "clear"
	case NoConnect:
		return "no connection: blocked by address, or the host is down"
	case NoAnswer:
		return "no answer to the handshake: looks like a reset on the name"
	case Reset:
		return "connection reset during the handshake"
	default:
		return "unknown"
	}
}

// dialer is net.Dialer's one method we use, pulled out so a test can stand in a
// fake without touching the network.
type dialer interface {
	Dial(network, address string) (net.Conn, error)
}

// Host reaches name on 443 and reports how it answered. The handshake is written
// by hand rather than with crypto/tls, because we need to see the raw reply — a
// silence, a reset, or bytes — not tls's interpretation of it.
func Host(d dialer, name string, wait time.Duration) (Verdict, error) {
	conn, err := d.Dial("tcp", name+":443")
	if err != nil {
		return NoConnect, nil
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(wait)); err != nil {
		return Clear, err
	}

	if _, err := conn.Write(hello(name)); err != nil {
		if isReset(err) {
			return Reset, nil
		}

		return NoAnswer, nil
	}

	reply := make([]byte, 1)

	switch _, err := conn.Read(reply); {
	case err == nil:
		return Clear, nil
	case isReset(err):
		return Reset, nil
	default:
		// A deadline with no byte read is the SNI-reset shape: the peer took the
		// hello and simply stopped, so nothing ever comes back.
		return NoAnswer, nil
	}
}

// Reset shows up as an OS error whose text says so; matching the text is portable
// where matching the numeric code is not.
func isReset(err error) bool {
	if err == nil {
		return false
	}

	text := strings.ToLower(err.Error())

	return strings.Contains(text, "reset") || strings.Contains(text, "forcibly closed")
}

// A minimal TLS ClientHello carrying the name in SNI: a real record, enough for a
// filter to read the name and act, without pulling in all of crypto/tls.
func hello(name string) []byte {
	server := []byte(name)

	ext := make([]byte, 0, len(server)+9)
	ext = append(ext, 0x00, 0x00)
	ext = binary.BigEndian.AppendUint16(ext, uint16(len(server)+5))
	ext = binary.BigEndian.AppendUint16(ext, uint16(len(server)+3))
	ext = append(ext, 0x00)
	ext = binary.BigEndian.AppendUint16(ext, uint16(len(server)))
	ext = append(ext, server...)

	body := make([]byte, 0, 64+len(ext))
	body = append(body, 0x03, 0x03)
	body = append(body, make([]byte, 32)...)
	body = append(body, 0x00)
	body = append(body, 0x00, 0x02, 0x13, 0x01)
	body = append(body, 0x01, 0x00)
	body = binary.BigEndian.AppendUint16(body, uint16(len(ext)))
	body = append(body, ext...)

	hand := make([]byte, 0, 4+len(body))
	hand = append(hand, 0x01)
	hand = append(hand, byte(len(body)>>16), byte(len(body)>>8), byte(len(body)))
	hand = append(hand, body...)

	rec := make([]byte, 0, 5+len(hand))
	rec = append(rec, 0x16, 0x03, 0x01)
	rec = binary.BigEndian.AppendUint16(rec, uint16(len(hand)))
	rec = append(rec, hand...)

	return rec
}
