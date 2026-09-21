package voice

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func discovery() []byte {
	out := make([]byte, discoveryLen)
	binary.BigEndian.PutUint16(out[0:2], discoveryAsk)
	binary.BigEndian.PutUint16(out[2:4], discoveryBody)
	binary.BigEndian.PutUint32(out[4:8], 0x1a2b3c4d)

	return out
}

func stun(body int) []byte {
	out := make([]byte, stunHeaderLen+body)
	out[0] = 0x00
	out[1] = 0x01
	binary.BigEndian.PutUint16(out[2:4], uint16(body))
	binary.BigEndian.PutUint32(out[4:8], stunCookie)

	return out
}

func TestDiscoveryIsRecognised(t *testing.T) {
	if !Discovery(discovery()) {
		t.Error("the datagram discord opens a call with was not recognised")
	}
}

// The trap: these ports carry games and anything else, and a fake put in front of
// someone else's traffic breaks what was working.
func TestDiscoveryRefusesWhatIsNotIt(t *testing.T) {
	short := discovery()[:73]

	wrongAsk := discovery()
	binary.BigEndian.PutUint16(wrongAsk[0:2], 2)

	wrongBody := discovery()
	binary.BigEndian.PutUint16(wrongBody[2:4], 60)

	// A reply carries the address discord asked for; only the question is ours.
	answered := discovery()
	answered[addressAt] = 0x5b

	// The size is exact, not a floor: something longer that begins the same way is
	// somebody else's traffic.
	longer := append(discovery(), bytes.Repeat([]byte{0}, 26)...)

	for name, payload := range map[string][]byte{
		"too short":         short,
		"not a question":    wrongAsk,
		"wrong body size":   wrongBody,
		"address is filled": answered,
		"nothing at all":    nil,
		"a stun message":    stun(8),
		"longer than it is": longer,
	} {
		t.Run(name, func(t *testing.T) {
			if Discovery(payload) {
				t.Error("taken for a discovery request")
			}
		})
	}
}

func TestStunIsRecognised(t *testing.T) {
	for _, body := range []int{0, 8, 100} {
		if !Stun(stun(body)) {
			t.Errorf("a stun message with a %d byte body was not recognised", body)
		}
	}
}

func TestStunRefusesWhatIsNotIt(t *testing.T) {
	noCookie := stun(8)
	binary.BigEndian.PutUint32(noCookie[4:8], 0xdeadbeef)

	topBits := stun(8)
	topBits[0] = 0x80

	oddLength := stun(8)
	oddLength[3] = 5

	// A length that claims more than the datagram holds is the usual lie.
	lying := stun(8)
	binary.BigEndian.PutUint16(lying[2:4], 400)

	for name, payload := range map[string][]byte{
		"no magic cookie":   noCookie,
		"top bits set":      topBits,
		"length not a word": oddLength,
		"length lies":       lying,
		"too short":         stun(0)[:19],
		"nothing at all":    nil,
	} {
		t.Run(name, func(t *testing.T) {
			if Stun(payload) {
				t.Error("taken for a stun message")
			}
		})
	}
}

// Whatever else lives on these ports has to pass through untouched.
func TestOtherTrafficIsLeftAlone(t *testing.T) {
	for name, payload := range map[string][]byte{
		"a game":      bytes.Repeat([]byte{0x7f}, 200),
		"voice audio": bytes.Repeat([]byte{0x80, 0x78}, 60),
		"one byte":    {0x00},
		"empty":       {},
	} {
		t.Run(name, func(t *testing.T) {
			if Ours(payload) {
				t.Error("a fake would have gone in front of this")
			}
		})
	}
}

func TestOursTakesBoth(t *testing.T) {
	if !Ours(discovery()) || !Ours(stun(8)) {
		t.Error("Ours misses one of the two it is made of")
	}
}
