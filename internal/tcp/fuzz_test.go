package tcp

import "testing"

func FuzzParse(f *testing.F) {
	f.Add([]byte{0, 80, 0, 80, 0, 0, 0, 0, 0, 0, 0, 0, 0x50, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{0, 80})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, segment []byte) {
		h, err := Parse(segment)
		if err != nil {
			return
		}

		if len(h.Payload) > len(segment) {
			t.Fatalf("payload of %d is larger than the segment of %d", len(h.Payload), len(segment))
		}
	})
}
