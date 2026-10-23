package clienthello

import "testing"

func FuzzParse(f *testing.F) {
	f.Add([]byte{recordHandshake, 0x03, 0x01, 0, 4, typeClientHello, 0, 0, 0})
	f.Add([]byte{recordHandshake})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, payload []byte) {
		h, err := Parse(payload)
		if err != nil {
			return
		}

		if len(h.Record) > len(payload) {
			t.Fatalf("record of %d runs past a payload of %d", len(h.Record), len(payload))
		}

		if len(h.Body) > len(payload) {
			t.Fatalf("body of %d runs past a payload of %d", len(h.Body), len(payload))
		}

		if _, err := h.ServerName(); err != nil {
			return
		}
	})
}
