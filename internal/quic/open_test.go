package quic

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"testing"
)

func rfc9001ClientKeys() Keys {
	return ClientInitialKeys([]byte{0x83, 0x94, 0xc8, 0xf0, 0x3e, 0x51, 0x57, 0x08})
}

func TestOpenReconstructsRFC9001Nonce(t *testing.T) {
	keys := rfc9001ClientKeys()

	// The A.1 client iv xored with packet number 2 is this nonce; encrypting with
	// it by hand, Open must derive the same nonce from iv and number to verify.
	nonce, err := hex.DecodeString("fa044b2f42a3fd3b46fb255e")
	if err != nil {
		t.Fatal(err)
	}

	aad := []byte{0xc3, 0xaa, 0xbb}
	plain := []byte("a crypto frame")

	block, err := aes.NewCipher(keys.Key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := gcm.Seal(nil, nonce, plain, aad)

	got, err := keys.Open(aad, ciphertext, 2)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, plain) {
		t.Errorf("opened %q, want %q", got, plain)
	}
}

func TestOpenRejectsWrongAAD(t *testing.T) {
	keys := rfc9001ClientKeys()

	block, _ := aes.NewCipher(keys.Key)
	gcm, _ := cipher.NewGCM(block)
	ciphertext := gcm.Seal(nil, keys.nonce(1), []byte("x"), []byte{0x01, 0x02})

	if _, err := keys.Open([]byte{0x01, 0x03}, ciphertext, 1); err == nil {
		t.Fatal("open accepted a header it was not sealed with")
	}
}
