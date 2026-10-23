package bypass

import (
	"obxod/internal/divert"
	"obxod/internal/ip"
	"obxod/internal/seal"
	"obxod/internal/tcp"
)

const syndataLabel = "syndata"

// zapret's default: sixteen zero bytes, enough for a dpi to see data where a bare
// syn carries none. A recorded hello (-fake) makes a more convincing one.
var defaultSynData = make([]byte, 16)

// syndata sends the opening syn carrying a payload in place of the empty one, which
// the caller then drops. A plain server ignores data in a syn, so the handshake
// still completes; a dpi that keys on the first data packet reads this fake first.
func (e *Engine) syndata(h sender, packet []byte, addr *divert.Addr) (bool, error) {
	outer, err := ip.Parse(packet)
	if err != nil {
		return false, nil
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		return false, nil
	}

	payload := e.synDataPayload()

	made, err := seal.Remade(packet, payload, segment.Seq)
	if err != nil {
		e.once(syndataLabel, "cannot add data to the syn: %v", err)

		return false, nil
	}

	if !e.wet {
		e.once(syndataLabel, "would send a syn carrying %d bytes instead of the empty one", len(payload))

		return false, nil
	}

	e.once(syndataLabel, "syn carrying %d bytes sent instead of the empty one", len(payload))

	return true, h.Send(made, addr)
}

func (e *Engine) synDataPayload() []byte {
	if len(e.recorded) > 0 {
		return e.recorded
	}

	return defaultSynData
}

// isClientSYN reports an opening syn: syn set, ack clear. The filter has already
// narrowed to the watched ports, so the flags are all that is left to check.
func isClientSYN(packet []byte) bool {
	outer, err := ip.Parse(packet)
	if err != nil || outer.Protocol != ip.ProtocolTCP {
		return false
	}

	segment, err := tcp.Parse(outer.Payload)
	if err != nil {
		return false
	}

	return segment.Flags&tcp.FlagSYN != 0 && segment.Flags&tcp.FlagACK == 0
}
