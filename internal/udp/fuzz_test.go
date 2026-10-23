package udp

import "testing"

func FuzzParse(f *testing.F) {
	f.Add([]byte{0, 80, 1, 187, 0, 8, 0, 0})
	f.Add([]byte{0, 80})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, datagram []byte) {
		h, err := Parse(datagram)
		if err != nil {
			return
		}

		if len(h.Payload) > len(datagram) {
			t.Fatalf("payload of %d is larger than the datagram of %d", len(h.Payload), len(datagram))
		}
	})
}
