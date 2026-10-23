package quic

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"testing"

	"obxod/internal/clienthello"
)

func TestDecoyLooksLikeAClientInitial(t *testing.T) {
	decoy, err := Decoy("www.youtube.com")
	if err != nil {
		t.Fatal(err)
	}

	if len(decoy) != minInitial {
		t.Errorf("decoy is %d bytes, want %d", len(decoy), minInitial)
	}

	// Header protection touches the low four bits of the first byte, never the high
	// four, so a long-header Initial still reads 0xc0 up top.
	if decoy[0]&0xf0 != 0xc0 {
		t.Errorf("first byte %#x is not a long-header Initial", decoy[0])
	}

	if decoy[5] != dcidLen {
		t.Errorf("connection id length byte is %d, want %d", decoy[5], dcidLen)
	}
}

// Opening the decoy and reading the name back is the whole claim: a dpi on the path
// derives the same keys from the connection id and sees exactly this.
func TestDecoyCarriesTheName(t *testing.T) {
	const name = "www.google.com"

	dcid := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	decoy, err := decoyWith(dcid, make([]byte, helloRandomLen), name)
	if err != nil {
		t.Fatal(err)
	}

	plain := openInitial(t, dcid, decoy, headerLen-pnLen)

	if plain[0] != 0x06 {
		t.Fatalf("first frame %#x, want a CRYPTO frame", plain[0])
	}

	at := 1
	_, n := readVarint(plain[at:])
	at += n
	length, n := readVarint(plain[at:])
	at += n
	hello := plain[at : at+int(length)]

	record := append([]byte{0x16, 0x03, 0x01, byte(len(hello) >> 8), byte(len(hello))}, hello...)

	parsed, err := clienthello.Parse(record)
	if err != nil {
		t.Fatalf("decoy hello did not parse: %v", err)
	}

	host, err := parsed.ServerName()
	if err != nil {
		t.Fatalf("decoy hello had no server name: %v", err)
	}

	if host.Host != name {
		t.Errorf("decoy names %q, want %q", host.Host, name)
	}
}

// openInitial reverses Seal with nothing but the standard library, so a shared bug
// cannot hide a round trip; the keys it derives are pinned by the A.1 vector test.
func openInitial(t *testing.T, dcid, packet []byte, pnOffset int) []byte {
	t.Helper()

	key, iv, hp := clientInitial(dcid)

	block, err := aes.NewCipher(hp[:])
	if err != nil {
		t.Fatal(err)
	}

	mask := make([]byte, aes.BlockSize)
	block.Encrypt(mask, packet[pnOffset+4:pnOffset+4+sampleLen])

	header := append([]byte{}, packet[:pnOffset+pnLen]...)
	header[0] = packet[0] ^ (mask[0] & 0x0f)

	var number uint64
	for i := 0; i < pnLen; i++ {
		header[pnOffset+i] = packet[pnOffset+i] ^ mask[1+i]
		number = number<<8 | uint64(header[pnOffset+i])
	}

	nonce := make([]byte, ivLen)
	copy(nonce, iv[:])
	binary.BigEndian.PutUint64(nonce[ivLen-8:], binary.BigEndian.Uint64(nonce[ivLen-8:])^number)

	data, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatal(err)
	}

	aead, err := cipher.NewGCM(data)
	if err != nil {
		t.Fatal(err)
	}

	plain, err := aead.Open(nil, nonce, packet[pnOffset+pnLen:], header)
	if err != nil {
		t.Fatalf("decoy did not open: %v", err)
	}

	return plain
}

func readVarint(b []byte) (uint64, int) {
	size := 1 << (b[0] >> 6)

	v := uint64(b[0] & 0x3f)
	for i := 1; i < size; i++ {
		v = v<<8 | uint64(b[i])
	}

	return v, size
}

// Two decoys for the same name must not share a client hello random, or a dpi could
// flag the fixed value and drop every fake.
func TestDecoyRandomizesTheHello(t *testing.T) {
	first, err := Decoy("www.google.com")
	if err != nil {
		t.Fatal(err)
	}

	second, err := Decoy("www.google.com")
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(helloRandom(t, first), helloRandom(t, second)) {
		t.Error("two decoys share a client hello random")
	}
}

func helloRandom(t *testing.T, decoy []byte) []byte {
	t.Helper()

	plain := openInitial(t, decoy[6:6+dcidLen], decoy, headerLen-pnLen)

	at := 1
	_, n := readVarint(plain[at:])
	at += n
	_, n = readVarint(plain[at:])
	at += n
	hello := plain[at:]

	// hello is 0x01, a three-byte length, the two-byte legacy version, then the random.
	return hello[6 : 6+helloRandomLen]
}
