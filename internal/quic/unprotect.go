package quic

import (
	"crypto/aes"
	"fmt"
)

type Revealed struct {
	PNLength  int
	Number    int
	HeaderLen int
}

func Unprotect(datagram []byte, pnOffset int, hp []byte) (Revealed, error) {
	sampleAt := pnOffset + 4
	if sampleAt+16 > len(datagram) {
		return Revealed{}, ErrShort
	}

	block, err := aes.NewCipher(hp)
	if err != nil {
		return Revealed{}, fmt.Errorf("quic: header protection key: %w", err)
	}

	var mask [16]byte
	block.Encrypt(mask[:], datagram[sampleAt:sampleAt+16])

	// RFC 9001 5.4.1: a long header hides only the low four bits of the first byte
	// (reserved and packet-number-length), so the mask touches just those.
	datagram[0] ^= mask[0] & 0x0f
	pnLength := int(datagram[0]&0x03) + 1

	number := 0
	for i := 0; i < pnLength; i++ {
		datagram[pnOffset+i] ^= mask[1+i]
		number = number<<8 | int(datagram[pnOffset+i])
	}

	return Revealed{
		PNLength:  pnLength,
		Number:    number,
		HeaderLen: pnOffset + pnLength,
	}, nil
}
