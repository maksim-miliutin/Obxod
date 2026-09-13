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

type Watch struct {
	Reset func(port uint16)
	Data  func(port uint16)

	Closed func(port uint16)

	// Counts every reply, matched or not: two different silences to tell apart.
	Seen func(total int)
}

func (w Watch) Run(h Receiver) error {
	buf := make([]byte, maxPacket)

	var total int

	for {
		n, _, err := h.Recv(buf)
		if err != nil {
			return err
		}

		total++

		if w.Seen != nil {
			w.Seen(total)
		}

		port, kind, ok := read(buf[:n])
		if !ok {
			continue
		}

		switch kind {
		case wasReset:
			if w.Reset != nil {
				w.Reset(port)
			}
		case wasClosed:
			if w.Closed != nil {
				w.Closed(port)
			}
		case wasData:
			if w.Data != nil {
				w.Data(port)
			}
		}
	}
}

type kind int

const (
	wasData kind = iota
	wasReset
	wasClosed
)

func read(packet []byte) (uint16, kind, bool) {
	outer, err := ip.Parse(packet)
	if err != nil || outer.Protocol != ip.ProtocolTCP {
		return 0, wasData, false
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		return 0, wasData, false
	}

	if segment.Flags&tcp.FlagRST != 0 {
		return segment.DstPort, wasReset, true
	}

	if segment.Flags&tcp.FlagFIN != 0 {
		return segment.DstPort, wasClosed, true
	}

	if len(segment.Payload) == 0 {
		return 0, wasData, false
	}

	return segment.DstPort, wasData, true
}
