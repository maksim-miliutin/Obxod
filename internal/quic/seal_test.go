package quic

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"testing"
)

func TestSealMatchesRFC9001Nonce(t *testing.T) {
	keys := rfc9001ClientKeys()

	nonce, err := hex.DecodeString("fa044b2f42a3fd3b46fb255e")
	if err != nil {
		t.Fatal(err)
	}

	aad := []byte{0xc3, 0x12, 0x34}
	plain := []byte("client hello frame")

	block, err := aes.NewCipher(keys.Key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	want := gcm.Seal(nil, nonce, plain, aad)

	got, err := keys.Seal(aad, plain, 2)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		t.Error("seal used a nonce other than iv xor number")
	}
}

func TestSealThenOpenRoundTrips(t *testing.T) {
	keys := rfc9001ClientKeys()

	aad := []byte{0xc3, 0x00, 0x01}
	plain := []byte("round trip me")

	ciphertext, err := keys.Seal(aad, plain, 7)
	if err != nil {
		t.Fatal(err)
	}

	got, err := keys.Open(aad, ciphertext, 7)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, plain) {
		t.Errorf("round trip opened %q, want %q", got, plain)
	}
}
