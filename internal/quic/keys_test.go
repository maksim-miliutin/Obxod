package quic

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// RFC 9001 Appendix A.1: the client Initial key, iv and header-protection key
// derived from DCID 8394c8f03e515708. Getting these wrong is getting the salt, the
// hkdf or a label wrong, and every one shows up here.
func TestClientInitialKeysMatchRFC9001(t *testing.T) {
	key, iv, hp := clientInitial(unhex(t, "8394c8f03e515708"))

	for _, c := range []struct {
		name string
		got  []byte
		want string
	}{
		{"key", key[:], "1f369613dd76d5467730efcbe3b1a22d"},
		{"iv", iv[:], "fa044b2f42a3fd3b46fb255c"},
		{"hp", hp[:], "9f50449e04a0e810283a1e9933adedd2"},
	} {
		if !bytes.Equal(c.got, unhex(t, c.want)) {
			t.Errorf("%s = %x, want %s", c.name, c.got, c.want)
		}
	}
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}

	return b
}
