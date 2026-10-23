package quic

import "crypto/rand"

const (
	dcidLen = 8
	pnLen   = 4
	tagLen  = 16

	// A client Initial is at least this many bytes on the wire (RFC 9000 §14.1); a
	// shorter decoy reads as a stray datagram, not the start of a handshake.
	minInitial = 1200

	// The fixed header shape: flags, version, dcid behind its length, empty scid and
	// token, a two-byte length, then the number. The length stays two bytes because a
	// 1200-byte Initial's always lands in that range.
	headerLen = 1 + 4 + 1 + dcidLen + 1 + 1 + 2 + pnLen
)

const helloRandomLen = 32

// Decoy builds a fake client Initial for name under a fresh random connection id
// and a fresh random hello, padded to the smallest legal size. It is a real,
// decryptable Initial: on the wire a dpi reads name out of it, which is the point.
func Decoy(name string) ([]byte, error) {
	seed := make([]byte, dcidLen+helloRandomLen)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}

	return decoyWith(seed[:dcidLen], seed[dcidLen:], name)
}

func decoyWith(dcid []byte, random []byte, name string) ([]byte, error) {
	plaintext := pad(cryptoFrame(clientHello(name, random)))

	keys, err := InitialKeys(dcid)
	if err != nil {
		return nil, err
	}

	packet, pnOffset := initialHeader(dcid, len(plaintext))
	packet = append(packet, plaintext...)

	return keys.Seal(packet, pnOffset, pnLen)
}

func initialHeader(dcid []byte, payloadLen int) ([]byte, int) {
	out := []byte{0xc3, 0x00, 0x00, 0x00, 0x01} // long header, Initial, 4-byte number; version 1
	out = append(out, byte(len(dcid)))
	out = append(out, dcid...)
	out = append(out, 0x00) // no source connection id
	out = append(out, 0x00) // no token

	out = append(out, varint(uint64(pnLen+payloadLen+tagLen))...)
	at := len(out)
	out = append(out, make([]byte, pnLen)...) // packet number zero

	return out, at
}

func cryptoFrame(hello []byte) []byte {
	out := []byte{0x06} // CRYPTO
	out = append(out, varint(0)...)
	out = append(out, varint(uint64(len(hello)))...)

	return append(out, hello...)
}

// pad fills the frames out so the sealed packet is exactly minInitial bytes; the
// trailing zeros read as PADDING frames.
func pad(frames []byte) []byte {
	want := minInitial - headerLen - tagLen
	if len(frames) >= want {
		return frames
	}

	return append(frames, make([]byte, want-len(frames))...)
}

func varint(n uint64) []byte {
	switch {
	case n < 1<<6:
		return []byte{byte(n)}
	case n < 1<<14:
		return []byte{byte(n>>8) | 0x40, byte(n)}
	case n < 1<<30:
		return []byte{byte(n>>24) | 0x80, byte(n >> 16), byte(n >> 8), byte(n)}
	default:
		return []byte{
			byte(n>>56) | 0xc0, byte(n >> 48), byte(n >> 40), byte(n >> 32),
			byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n),
		}
	}
}
