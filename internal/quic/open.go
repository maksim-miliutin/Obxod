package quic

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

func (k Keys) Open(aad, ciphertext []byte, number int) ([]byte, error) {
	gcm, err := k.aead()
	if err != nil {
		return nil, err
	}

	plain, err := gcm.Open(nil, k.nonce(number), ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("quic: open initial payload: %w", err)
	}

	return plain, nil
}

func (k Keys) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(k.Key)
	if err != nil {
		return nil, fmt.Errorf("quic: payload key: %w", err)
	}

	return cipher.NewGCM(block)
}

// RFC 9001 5.3: the nonce is the iv with the packet number, big-endian and left
// padded, xored into its tail.
func (k Keys) nonce(number int) []byte {
	n := make([]byte, len(k.IV))
	copy(n, k.IV)

	for i := 0; i < 8; i++ {
		n[len(n)-1-i] ^= byte(number >> (8 * i))
	}

	return n
}
