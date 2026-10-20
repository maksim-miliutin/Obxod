package quic

import (
	"crypto/aes"
	"fmt"
)

func Protect(datagram []byte, pnOffset int, hp []byte) error {
	sampleAt := pnOffset + 4
	if sampleAt+16 > len(datagram) {
		return ErrShort
	}

	block, err := aes.NewCipher(hp)
	if err != nil {
		return fmt.Errorf("quic: header protection key: %w", err)
	}

	var mask [16]byte
	block.Encrypt(mask[:], datagram[sampleAt:sampleAt+16])

	// The first byte is still clear here, so its low two bits give the number length
	// before the mask hides them again (RFC 9001 5.4.1).
	pnLength := int(datagram[0]&0x03) + 1

	for i := 0; i < pnLength; i++ {
		datagram[pnOffset+i] ^= mask[1+i]
	}

	datagram[0] ^= mask[0] & 0x0f

	return nil
}
