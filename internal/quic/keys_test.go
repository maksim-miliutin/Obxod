package quic

import (
	"encoding/hex"
	"testing"
)

func TestClientInitialKeysMatchRFC9001(t *testing.T) {
	dcid, err := hex.DecodeString("8394c8f03e515708")
	if err != nil {
		t.Fatal(err)
	}

	keys := ClientInitialKeys(dcid)

	for _, want := range []struct {
		name string
		got  []byte
		hex  string
	}{
		{"key", keys.Key, "1f369613dd76d5467730efcbe3b1a22d"},
		{"iv", keys.IV, "fa044b2f42a3fd3b46fb255c"},
		{"hp", keys.HP, "9f50449e04a0e810283a1e9933adedd2"},
	} {
		if got := hex.EncodeToString(want.got); got != want.hex {
			t.Errorf("%s = %s, want %s", want.name, got, want.hex)
		}
	}
}

func TestClientInitialKeysDependOnConnectionID(t *testing.T) {
	one := ClientInitialKeys([]byte{1, 2, 3, 4})
	two := ClientInitialKeys([]byte{4, 3, 2, 1})

	if hex.EncodeToString(one.Key) == hex.EncodeToString(two.Key) {
		t.Fatal("different connection ids gave the same key")
	}
}
