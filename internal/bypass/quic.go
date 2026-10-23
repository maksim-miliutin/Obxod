package bypass

import (
	"obxod/internal/divert"
	"obxod/internal/ip"
	"obxod/internal/quic"
	"obxod/internal/seal"
	"obxod/internal/udp"
)

const faking = "quic"

// One decoy is enough for an inspector, but udp drops, so a couple go out.
const quicCopies = 2

// fakeInitial throws a fake Initial for another name at the same server ahead of
// the real one. The decoy carries a wrong checksum so it dies before the server;
// the inspector reads the harmless name first and lets the real handshake by.
func (e *Engine) fakeInitial(h sender, packet []byte, addr *divert.Addr) error {
	decoy, err := quic.Decoy(e.fakeQUIC)
	if err != nil {
		e.once(faking, "cannot build a fake initial: %v", err)

		return nil
	}

	made, err := seal.SpoiledDatagram(packet, decoy)
	if err != nil {
		e.once(faking, "cannot put a fake initial ahead: %v", err)

		return nil
	}

	if !e.wet {
		e.once(faking, "would send a fake quic initial for %s first", e.fakeQUIC)

		return nil
	}

	e.once(faking, "fake quic initial for %s sent first", e.fakeQUIC)

	for range quicCopies {
		if err := h.Send(made, addr); err != nil {
			return err
		}
	}

	return nil
}

func isQUICInitial(packet []byte) bool {
	outer, err := ip.Parse(packet)
	if err != nil || outer.Protocol != ip.ProtocolUDP {
		return false
	}

	datagram, err := udp.Parse(outer.Payload)
	if err != nil || datagram.DstPort != 443 {
		return false
	}

	return quic.IsInitial(datagram.Payload)
}
