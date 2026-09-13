package replies

import (
	"obxod/internal/divert"
	"obxod/internal/ip"
	"obxod/internal/tcp"
)

const maxPacket = 0xffff + 40

type Receiver interface {
	Recv(buf []byte) (int, divert.Addr, error)
}

// Watch reports what comes back on port 443 for a connection we touched. A reset
// says the way through failed after the handshake started, which no amount of
// watching our own outgoing packets would ever show.
type Watch struct {
	Reset func(port uint16)
	Data  func(port uint16)
}

func (w Watch) Run(h Receiver) error {
	buf := make([]byte, maxPacket)

	for {
		n, _, err := h.Recv(buf)
		if err != nil {
			return err
		}

		port, reset, ok := read(buf[:n])
		if !ok {
			continue
		}

		if reset && w.Reset != nil {
			w.Reset(port)

			continue
		}

		if !reset && w.Data != nil {
			w.Data(port)
		}
	}
}

// read returns the port on our side, so the caller can tell which of its own
// connections the reply belongs to.
func read(packet []byte) (uint16, bool, bool) {
	outer, err := ip.Parse(packet)
	if err != nil || outer.Protocol != ip.ProtocolTCP {
		return 0, false, false
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil || segment.SrcPort != 443 {
		return 0, false, false
	}

	if segment.Flags&tcp.FlagRST != 0 {
		return segment.DstPort, true, true
	}

	if len(segment.Payload) == 0 {
		return 0, false, false
	}

	return segment.DstPort, false, true
}
