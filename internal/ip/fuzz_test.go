package ip

import "testing"

func FuzzParse(f *testing.F) {
	f.Add([]byte{0x45, 0, 0, 20, 0, 0, 0, 0, 64, 6, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8})
	f.Add([]byte{0x45})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, packet []byte) {
		h, err := Parse(packet)
		if err != nil {
			return
		}

		if h.HeaderLen < minHeaderLen || h.HeaderLen > len(packet) {
			t.Fatalf("header len %d outside a packet of %d", h.HeaderLen, len(packet))
		}

		if len(h.Payload) > len(packet)-h.HeaderLen {
			t.Fatalf("payload of %d runs past the packet", len(h.Payload))
		}
	})
}
