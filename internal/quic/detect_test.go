package quic

import "testing"

func TestIsInitialSpotsADecoy(t *testing.T) {
	decoy, err := Decoy("www.google.com")
	if err != nil {
		t.Fatal(err)
	}

	if !IsInitial(decoy) {
		t.Error("a decoy we built was not taken for an Initial")
	}
}

func TestIsInitialRejectsOthers(t *testing.T) {
	for name, payload := range map[string][]byte{
		"empty":         nil,
		"too short":     {0xc0, 0x00, 0x00},
		"short header":  {0x40, 0x00, 0x00, 0x00, 0x01},
		"handshake":     {0xe0, 0x00, 0x00, 0x00, 0x01},
		"wrong version": {0xc0, 0x00, 0x00, 0x00, 0x02},
	} {
		if IsInitial(payload) {
			t.Errorf("%s was taken for an Initial", name)
		}
	}
}
