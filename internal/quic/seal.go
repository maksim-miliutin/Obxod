package quic

import (
	"crypto/aes"
	"encoding/binary"
	"errors"
)

const sampleLen = 16

var ErrShortPacket = errors.New("quic: packet ends before its number or sample")

// Seal encrypts the payload of an unprotected Initial, then masks the first byte
// and the packet number, and returns the protected packet. pnOffset is where the
// number starts and pnLen how wide it is. The input packet is not changed.
func (k Keys) Seal(packet []byte, pnOffset, pnLen int) ([]byte, error) {
	if pnLen <= 0 || pnOffset+pnLen > len(packet) {
		return nil, ErrShortPacket
	}

	header := packet[:pnOffset+pnLen]
	payload := packet[pnOffset+pnLen:]

	var number uint64
	for _, b := range packet[pnOffset : pnOffset+pnLen] {
		number = number<<8 | uint64(b)
	}

	nonce := make([]byte, ivLen)
	copy(nonce, k.iv[:])
	binary.BigEndian.PutUint64(nonce[ivLen-8:], binary.BigEndian.Uint64(nonce[ivLen-8:])^number)

	out := make([]byte, len(header), len(header)+len(payload)+k.aead.Overhead())
	copy(out, header)
	out = k.aead.Seal(out, nonce, payload, header)

	// RFC 9001 §5.4.2: the sample starts four bytes past the number field whatever
	// its width, so it lands at a fixed place in the ciphertext.
	from := pnOffset + 4
	if from+sampleLen > len(out) {
		return nil, ErrShortPacket
	}

	mask := make([]byte, aes.BlockSize)
	k.hp.Encrypt(mask, out[from:from+sampleLen])

	out[0] ^= mask[0] & 0x0f // a long header protects only the low four bits
	for i := 0; i < pnLen; i++ {
		out[pnOffset+i] ^= mask[1+i]
	}

	return out, nil
}
