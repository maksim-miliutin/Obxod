package bypass

import (
	"obxod/internal/divert"
	"obxod/internal/ip"
	"obxod/internal/rules"
	"obxod/internal/seal"
	"obxod/internal/udp"
	"obxod/internal/voice"
)

// voice puts recorded datagrams in front of the one that opens a call. Unlike a
// hello there is no name in here to match, so the rule is looked up by nothing
// but the protocol: whoever asked for fakeudp gets it on every voice datagram.
func (e *Engine) voice(h sender, packet []byte, addr *divert.Addr) (bool, error) {
	outer, err := ip.Parse(packet)
	if err != nil || outer.Protocol != ip.ProtocolUDP {
		return false, nil
	}

	datagram, err := udp.Parse(outer.Payload)
	if err != nil || !voice.Ours(datagram.Payload) {
		return false, nil
	}

	r, ok := e.voiceRule()
	if !ok {
		return false, nil
	}

	if len(e.voiced) == 0 {
		e.once(opening, "cannot put a datagram ahead: none was recorded, give one with -fakeudp")

		return false, nil
	}

	made, err := seal.Datagram(packet, e.voiced)
	if err != nil {
		e.once(opening, "cannot put a datagram ahead: %v", err)

		return false, nil
	}

	if !e.wet {
		e.once(opening, "would send %d recorded datagrams first", r.FakeUDP)

		return false, nil
	}

	e.once(opening, "%d recorded datagrams sent first", r.FakeUDP)

	for range r.FakeUDP {
		if err := h.Send(made, addr); err != nil {
			return false, err
		}
	}

	return false, nil
}

// A voice datagram carries no name, so the way is taken from whichever rule asked
// for it. Two rules asking for different counts is a question nobody has posed.
func (e *Engine) voiceRule() (rules.Rule, bool) {
	for _, r := range e.set {
		if r.FakeUDP != 0 {
			return r, true
		}
	}

	return rules.Rule{}, false
}

// Voice datagrams have no host to name the line after.
const opening = "voice"
