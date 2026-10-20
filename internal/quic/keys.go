package quic

import (
	"crypto/hmac"
	"crypto/sha256"
)

// RFC 9001 5.2: the version-1 Initial salt, mixed with the client's chosen
// destination connection id to seed every Initial key.
var initialSalt = []byte{
	0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17,
	0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a,
}

type Keys struct {
	Key []byte
	IV  []byte
	HP  []byte
}

func ClientInitialKeys(dcid []byte) Keys {
	secret := expandLabel(extract(initialSalt, dcid), "client in", 32)

	return Keys{
		Key: expandLabel(secret, "quic key", 16),
		IV:  expandLabel(secret, "quic iv", 12),
		HP:  expandLabel(secret, "quic hp", 16),
	}
}

func extract(salt, ikm []byte) []byte {
	mac := hmac.New(sha256.New, salt)
	mac.Write(ikm)

	return mac.Sum(nil)
}

// HKDF-Expand with TLS 1.3 label framing (RFC 8446 7.1); one round fills it,
// since no Initial secret is longer than a single hash.
func expandLabel(secret []byte, label string, length int) []byte {
	full := "tls13 " + label

	info := []byte{byte(length >> 8), byte(length), byte(len(full))}
	info = append(info, full...)
	info = append(info, 0)

	mac := hmac.New(sha256.New, secret)
	mac.Write(info)
	mac.Write([]byte{1})

	return mac.Sum(nil)[:length]
}
