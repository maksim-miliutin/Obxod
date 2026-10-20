package quic

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

func TestParseInitialReadsRFC9001Header(t *testing.T) {
	head, err := hex.DecodeString("c300000001088394c8f03e5157080000449e00000002")
	if err != nil {
		t.Fatal(err)
	}

	packet := make([]byte, 1200)
	copy(packet, head)

	got, err := ParseInitial(packet)
	if err != nil {
		t.Fatal(err)
	}

	if got.Version != 1 {
		t.Errorf("version = %d, want 1", got.Version)
	}
	if h := hex.EncodeToString(got.DCID); h != "8394c8f03e515708" {
		t.Errorf("dcid = %s, want 8394c8f03e515708", h)
	}
	if len(got.SCID) != 0 {
		t.Errorf("scid = %x, want empty", got.SCID)
	}
	if len(got.Token) != 0 {
		t.Errorf("token = %x, want empty", got.Token)
	}
	if got.Length != 1182 {
		t.Errorf("length = %d, want 1182", got.Length)
	}
	if got.PNOffset != 18 {
		t.Errorf("pn offset = %d, want 18", got.PNOffset)
	}
}

func TestParseInitialKeepsScidAndToken(t *testing.T) {
	packet, err := hex.DecodeString("c00000000102aabb0311223302dead050000000000")
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseInitial(packet)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got.DCID, []byte{0xaa, 0xbb}) {
		t.Errorf("dcid = %x, want aabb", got.DCID)
	}
	if !bytes.Equal(got.SCID, []byte{0x11, 0x22, 0x33}) {
		t.Errorf("scid = %x, want 112233", got.SCID)
	}
	if !bytes.Equal(got.Token, []byte{0xde, 0xad}) {
		t.Errorf("token = %x, want dead", got.Token)
	}
	if got.Length != 5 {
		t.Errorf("length = %d, want 5", got.Length)
	}
	if got.PNOffset != 16 {
		t.Errorf("pn offset = %d, want 16", got.PNOffset)
	}
}

func TestParseInitialRejects(t *testing.T) {
	for _, c := range []struct {
		name string
		hex  string
		want error
	}{
		{"short header", "4000000000", ErrNotLongHeader},
		{"not initial type", "e000000001088394c8f03e5157080000449e", ErrNotInitial},
		{"wrong version", "c000000002088394c8f03e5157080000449e", ErrNotInitial},
		{"truncated dcid", "c00000000108839400", ErrShort},
	} {
		p, err := hex.DecodeString(c.hex)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := ParseInitial(p); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}
