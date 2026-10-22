package quic

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
)

// RFC 9001 §5.2: a version 1 Initial is keyed from its own connection id and this
// one published salt, so anyone on the path derives the same keys. A fake Initial
// therefore has to be genuinely encrypted, not stuffed, to read as one.
var initialSalt = []byte{
	0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17,
	0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a,
}

const (
	keyLen = 16
	ivLen  = 12
)

type Keys struct {
	aead cipher.AEAD
	hp   cipher.Block
	iv   [ivLen]byte
}

func InitialKeys(dcid []byte) (Keys, error) {
	key, iv, hp := clientInitial(dcid)

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return Keys{}, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Keys{}, err
	}

	mask, err := aes.NewCipher(hp[:])
	if err != nil {
		return Keys{}, err
	}

	return Keys{aead: aead, hp: mask, iv: iv}, nil
}

func clientInitial(dcid []byte) (key [keyLen]byte, iv [ivLen]byte, hp [keyLen]byte) {
	secret := expandLabel(hkdfExtract(initialSalt, dcid), "client in", sha256.Size)
	copy(key[:], expandLabel(secret, "quic key", keyLen))
	copy(iv[:], expandLabel(secret, "quic iv", ivLen))
	copy(hp[:], expandLabel(secret, "quic hp", keyLen))

	return
}

func hkdfExtract(salt, keying []byte) []byte {
	mac := hmac.New(sha256.New, salt)
	mac.Write(keying)

	return mac.Sum(nil)
}

// expandLabel is HKDF-Expand-Label (RFC 8446 §7.1) with the "tls13 " prefix and an
// empty context. One hmac block covers it because every QUIC label asks for no more
// than a hash's worth of bytes.
func expandLabel(secret []byte, label string, length int) []byte {
	full := "tls13 " + label

	info := make([]byte, 0, 4+len(full))
	info = append(info, byte(length>>8), byte(length), byte(len(full)))
	info = append(info, full...)
	info = append(info, 0)

	mac := hmac.New(sha256.New, secret)
	mac.Write(info)
	mac.Write([]byte{1})

	return mac.Sum(nil)[:length]
}
