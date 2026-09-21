package bypass

import (
	"bytes"
	"encoding/binary"
	"obxod/internal/sweep"
	"strings"
	"testing"
	"time"

	"obxod/internal/checksum"
	"obxod/internal/divert"
	"obxod/internal/ip"
)

func voicePacket(payload []byte) []byte {
	out := make([]byte, 20)
	out[0] = 4<<4 | 5
	out[8] = 64
	out[9] = ip.ProtocolUDP
	copy(out[12:16], []byte{192, 168, 1, 2})
	copy(out[16:20], []byte{162, 159, 130, 234})

	body := make([]byte, 8)
	binary.BigEndian.PutUint16(body[0:2], 51000)
	binary.BigEndian.PutUint16(body[2:4], 50001)
	binary.BigEndian.PutUint16(body[4:6], uint16(8+len(payload)))
	body = append(body, payload...)

	out = append(out, body...)
	binary.BigEndian.PutUint16(out[2:4], uint16(len(out)))
	binary.BigEndian.PutUint16(out[10:12], checksum.IPv4(out[:20]))

	return out
}

func discovery() []byte {
	out := make([]byte, 74)
	binary.BigEndian.PutUint16(out[0:2], 1)
	binary.BigEndian.PutUint16(out[2:4], 70)
	binary.BigEndian.PutUint32(out[4:8], 0x1a2b3c4d)

	return out
}

func voiceEngine(t *testing.T, rule string, recorded []byte) (*Engine, *lines) {
	t.Helper()

	out := &lines{}

	return New(Settings{
		Rules:  setOf(t, rule),
		Wet:    true,
		Voiced: recorded,
		Report: out.say,
	}), out
}

func TestRecordedDatagramsGoFirst(t *testing.T) {
	recorded := bytes.Repeat([]byte{0xab}, 1200)

	e, _ := voiceEngine(t, "discord.media=fakeudp:5", recorded)
	r := &recorder{}

	if sent, err := e.voice(r, voicePacket(discovery()), &divert.Addr{}); err != nil || sent {
		t.Fatalf("voice returned %v, %v; the real datagram still has to go through", sent, err)
	}

	if len(r.sent) != 5 {
		t.Fatalf("sent %d datagrams, want the five asked for", len(r.sent))
	}

	for i, one := range r.sent {
		if !bytes.Equal(one[28:], recorded) {
			t.Errorf("datagram %d does not carry what was recorded", i)
		}
	}
}

// These ports carry games and voice audio too, and a fake in front of those
// breaks what was working.
func TestNothingGoesAheadOfOtherTraffic(t *testing.T) {
	for name, payload := range map[string][]byte{
		"a game":      bytes.Repeat([]byte{0x7f}, 200),
		"voice audio": bytes.Repeat([]byte{0x80, 0x78}, 60),
		"an answer":   append(append([]byte{}, discovery()[:8]...), bytes.Repeat([]byte{9}, 66)...),
	} {
		t.Run(name, func(t *testing.T) {
			e, _ := voiceEngine(t, "discord.media=fakeudp:5", bytes.Repeat([]byte{0xab}, 100))
			r := &recorder{}

			if _, err := e.voice(r, voicePacket(payload), &divert.Addr{}); err != nil {
				t.Fatalf("voice: %v", err)
			}

			if len(r.sent) != 0 {
				t.Errorf("sent %d datagrams ahead of somebody else's traffic", len(r.sent))
			}
		})
	}
}

func TestNothingHappensWithoutTheWay(t *testing.T) {
	e, _ := voiceEngine(t, "discord.media=hostfake:mail.ru", bytes.Repeat([]byte{0xab}, 100))
	r := &recorder{}

	if _, err := e.voice(r, voicePacket(discovery()), &divert.Addr{}); err != nil {
		t.Fatalf("voice: %v", err)
	}

	if len(r.sent) != 0 {
		t.Errorf("sent %d datagrams for a rule that never asked", len(r.sent))
	}
}

// A way that silently does nothing is the worst kind: the file is the whole point.
func TestTheWayWithoutARecordingSaysSo(t *testing.T) {
	e, out := voiceEngine(t, "discord.media=fakeudp:5", nil)
	r := &recorder{}

	if _, err := e.voice(r, voicePacket(discovery()), &divert.Addr{}); err != nil {
		t.Fatalf("voice: %v", err)
	}

	if len(r.sent) != 0 {
		t.Errorf("sent %d datagrams with nothing recorded", len(r.sent))
	}

	var told bool

	for _, said := range out.all() {
		if strings.Contains(said, "none was recorded") {
			told = true
		}
	}

	if !told {
		t.Errorf("said nothing about the missing recording:\n%s", strings.Join(out.all(), "\n"))
	}
}

func TestNothingIsSentWhileDry(t *testing.T) {
	out := &lines{}

	e := New(Settings{
		Rules:  setOf(t, "discord.media=fakeudp:5"),
		Voiced: bytes.Repeat([]byte{0xab}, 100),
		Report: out.say,
	})

	r := &recorder{}

	if _, err := e.voice(r, voicePacket(discovery()), &divert.Addr{}); err != nil {
		t.Fatalf("voice: %v", err)
	}

	if len(r.sent) != 0 {
		t.Errorf("sent %d datagrams while dry", len(r.sent))
	}
}

// The bug this guards: a sweep swaps its candidate into the set, none of which
// carry fakeudp, and voice used to read the set — so tuning one host by sweep
// silently dropped voice for the whole run. It has to keep working from the base.
func TestVoiceKeepsWorkingDuringASweep(t *testing.T) {
	recorded := bytes.Repeat([]byte{0xab}, 1200)

	// The sweep is tuning the very host the fakeudp rule is on, so withCandidate
	// drops that rule from the set — and voice must still find it in the base.
	e := New(Settings{
		Rules:  setOf(t, "discord.media=fakeudp:5"),
		Hunt:   sweep.New("discord.media", sweep.Candidates("discord.media"), time.Second, time.Now()),
		Wet:    true,
		Voiced: recorded,
	})

	r := &recorder{}

	if _, err := e.voice(r, voicePacket(discovery()), &divert.Addr{}); err != nil {
		t.Fatalf("voice: %v", err)
	}

	if len(r.sent) != 5 {
		t.Fatalf("sent %d datagrams during a sweep, want the five fakeudp asks for", len(r.sent))
	}
}
