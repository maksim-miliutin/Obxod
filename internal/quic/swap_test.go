package quic

import (
	"bytes"
	"errors"
	"testing"
)

func buildInitial(t *testing.T, dcid []byte, host string, number int) []byte {
	t.Helper()

	plain := cryptoPayload(host)
	keys := ClientInitialKeys(dcid)

	header := []byte{0xc0}
	header = append(header, 0x00, 0x00, 0x00, 0x01)
	header = append(header, byte(len(dcid)))
	header = append(header, dcid...)
	header = append(header, 0x00, 0x00)
	header = appendVarint(header, 1+len(plain)+16)

	pnOffset := len(header)
	header = append(header, byte(number))

	aad := append([]byte(nil), header...)

	sealed, err := keys.Seal(aad, plain, number)
	if err != nil {
		t.Fatal(err)
	}

	datagram := append(header, sealed...)

	if err := Protect(datagram, pnOffset, keys.HP); err != nil {
		t.Fatal(err)
	}

	return datagram
}

func TestSwapNameMatchesADatagramBuiltWithTheDecoy(t *testing.T) {
	dcid := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	original := buildInitial(t, dcid, "cdn.discordapp.com", 0)
	decoy := buildInitial(t, dcid, "xyz.discordapp.com", 0)

	got, err := SwapName(original, "xyz.discordapp.com")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, decoy) {
		t.Error("swapped datagram differs from one built with the decoy name")
	}
}

func TestSwapNameLeavesTheInputAlone(t *testing.T) {
	dcid := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	original := buildInitial(t, dcid, "cdn.discordapp.com", 0)
	keep := append([]byte(nil), original...)

	if _, err := SwapName(original, "xyz.discordapp.com"); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(original, keep) {
		t.Error("swap mutated the datagram it was given")
	}
}

func TestSwapNameRejectsADifferentLength(t *testing.T) {
	dcid := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	original := buildInitial(t, dcid, "cdn.discordapp.com", 0)

	if _, err := SwapName(original, "short.com"); !errors.Is(err, ErrNameLength) {
		t.Errorf("err = %v, want ErrNameLength", err)
	}
}
