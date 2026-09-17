package bypass

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"obxod/internal/cut"
	"obxod/internal/divert"
	"obxod/internal/forge"
	"obxod/internal/hello"
	"obxod/internal/ip"
	"obxod/internal/rules"
)

func clientHello(host string) []byte {
	name := []byte(host)

	list := binary.BigEndian.AppendUint16(nil, uint16(len(name)+3))
	list = append(list, 0x00)
	list = binary.BigEndian.AppendUint16(list, uint16(len(name)))
	list = append(list, name...)

	sni := binary.BigEndian.AppendUint16(nil, 0x0000)
	sni = binary.BigEndian.AppendUint16(sni, uint16(len(list)))
	sni = append(sni, list...)

	// Real hellos carry more extensions after the name; without one, "after" has
	// nothing left to cut.
	trailing := binary.BigEndian.AppendUint16(nil, 0x0015)
	trailing = binary.BigEndian.AppendUint16(trailing, 8)
	trailing = append(trailing, bytes.Repeat([]byte{0x00}, 8)...)

	extensions := binary.BigEndian.AppendUint16(nil, uint16(len(sni)+len(trailing)))
	extensions = append(extensions, sni...)
	extensions = append(extensions, trailing...)

	body := []byte{0x03, 0x03}
	body = append(body, bytes.Repeat([]byte{0xab}, 32)...)
	body = append(body, 0x00, 0x00, 0x02, 0x13, 0x01, 0x01, 0x00)
	body = append(body, extensions...)

	handshake := []byte{0x01, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}
	handshake = append(handshake, body...)

	record := []byte{0x16, 0x03, 0x01, 0x00, 0x00}
	binary.BigEndian.PutUint16(record[3:5], uint16(len(handshake)))

	return append(record, handshake...)
}

func packet443(payload []byte) []byte {
	transport := make([]byte, 20)
	binary.BigEndian.PutUint16(transport[2:4], 443)
	transport[12] = 5 << 4
	transport[13] = 0x18
	transport = append(transport, payload...)

	p := make([]byte, 20)
	p[0] = 4<<4 | 5
	p[8] = 64
	p[9] = ip.ProtocolTCP
	copy(p[12:16], []byte{192, 168, 1, 2})
	copy(p[16:20], []byte{93, 184, 216, 34})
	p = append(p, transport...)
	binary.BigEndian.PutUint16(p[2:4], uint16(len(p)))

	return p
}

func datagramTo(port uint16) []byte {
	datagram := make([]byte, 8)
	binary.BigEndian.PutUint16(datagram[0:2], 51234)
	binary.BigEndian.PutUint16(datagram[2:4], port)
	binary.BigEndian.PutUint16(datagram[4:6], 8+4)
	datagram = append(datagram, 0xc0, 0x00, 0x00, 0x01)

	packet := make([]byte, 20)
	packet[0] = 4<<4 | 5
	packet[8] = 64
	packet[9] = ip.ProtocolUDP
	packet = append(packet, datagram...)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))

	return packet
}

type recorder struct {
	sent [][]byte
}

func (r *recorder) Send(packet []byte, addr *divert.Addr) error {
	r.sent = append(r.sent, append([]byte(nil), packet...))

	return nil
}

func engineFor(t *testing.T, wet bool, pattern []byte, texts ...string) *Engine {
	t.Helper()

	set, err := rules.ParseAll(texts)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}

	return New(Settings{Rules: set, Pattern: pattern, Wet: wet})
}

// The whole point of commit 12: a hello for the chosen host yields a valid copy
// that sits ahead of the original, and everything else is left untouched.
func TestChosenHostIsCopied(t *testing.T) {
	packet := packet443(clientHello("gateway.discord.gg"))

	found, ok := hello.Found(packet)
	if !ok || found.Host != "gateway.discord.gg" {
		t.Fatalf("host not recognised: %v %q", ok, found.Host)
	}

	copied, err := forge.Copy(packet, forge.Recipe{TTL: 4})
	if err != nil {
		t.Fatalf("forge.Copy: %v", err)
	}

	if copied[8] != 4 {
		t.Errorf("copy ttl = %d, want 4", copied[8])
	}

	if !bytes.Equal(packet, packet443(clientHello("gateway.discord.gg"))) {
		t.Error("the original packet was modified")
	}
}

func TestOtherTrafficIsNotAHello(t *testing.T) {
	cases := map[string][]byte{
		"plain data": packet443([]byte{0x17, 0x03, 0x03, 0x00, 0x05}),
		"bare ack":   packet443(nil),
	}

	for name, packet := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := hello.Found(packet); ok {
				t.Error("plain traffic taken for a hello")
			}
		})
	}
}

// The bug this guards: splitting just past the name leaves the whole name in the
// first packet, so an inspector reading packets one by one still sees it.
func TestCutPointBreaksTheName(t *testing.T) {
	const host = "updates.discord.com"

	packet := packet443(clientHello(host))

	found, ok := hello.Found(packet)
	if !ok {
		t.Fatal("hello went unrecognised")
	}

	// Only a cut through the name leaves neither packet holding it whole. The other
	// two aim at inspectors that judge a stream by its first packet.
	cases := map[string]bool{
		"name":  true,
		"after": false,
		"start": false,
	}

	for where, wantBroken := range cases {
		t.Run(where, func(t *testing.T) {
			point, err := pointFor(found, where)
			if err != nil {
				t.Fatalf("pointFor: %v", err)
			}

			first, second, err := cut.At(packet, point)
			if err != nil {
				t.Fatalf("cut.At: %v", err)
			}

			broken := !bytes.Contains(first, []byte(host)) && !bytes.Contains(second, []byte(host))

			if broken != wantBroken {
				t.Errorf("name broken across packets = %v, want %v", broken, wantBroken)
			}

			joined := append(append([]byte(nil), first[40:]...), second[40:]...)

			if !bytes.Contains(joined, []byte(host)) {
				t.Error("the name did not survive being put back together")
			}
		})
	}
}

func TestCutPointRejectsNonsense(t *testing.T) {
	if _, err := pointFor(hello.Outgoing{Host: "x"}, "sideways"); err == nil {
		t.Error("an unknown cut point was accepted")
	}
}

func TestDecoyForKeepsTheLength(t *testing.T) {
	hosts := []string{
		"updates.discord.com",
		"gateway.discord.gg",
		"ya.ru",
		"a.io",
		"www.youtube.com",
		"very-long-subdomain.example.co.uk",
	}

	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			decoy := decoyFor(host)

			if len(decoy) != len(host) {
				t.Errorf("decoy %q is %d bytes, host %q is %d", decoy, len(decoy), host, len(host))
			}

			if decoy == host {
				t.Error("the decoy is the very name we are hiding")
			}
		})
	}
}

// The bug this guards: a cut used to return before the decoy was ever sent, so
// asking for both quietly gave only the cut.
func TestDecoyAndCutBothGoOut(t *testing.T) {
	const host = "updates.discord.com"

	packet := packet443(clientHello(host))
	addr := &divert.Addr{}

	cases := []struct {
		name    string
		rule    string
		decoy   string
		want    int
		ownSend bool
	}{
		{"decoy alone", "updates.discord.com=ttl:4,decoy", "auto", 1, false},
		{"cut alone", "updates.discord.com=cut:name", "", 2, true},
		{"decoy and cut", "updates.discord.com=ttl:4,decoy,cut:name", "auto", 3, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &recorder{}

			sent, err := engineFor(t, true, nil, c.rule).forward(r, packet, addr)
			if err != nil {
				t.Fatalf("forward: %v", err)
			}

			if len(r.sent) != c.want {
				t.Fatalf("sent %d packets, want %d", len(r.sent), c.want)
			}

			if sent != c.ownSend {
				t.Errorf("took over sending = %v, want %v", sent, c.ownSend)
			}

			if c.decoy != "" {
				decoy := decoyFor(host)

				if !bytes.Contains(r.sent[0], []byte(decoy)) {
					t.Error("the first packet out is not the decoy")
				}

				if bytes.Contains(r.sent[0], []byte(host)) {
					t.Error("the decoy still carries the blocked name")
				}
			}

			if strings.Contains(c.rule, "cut") {
				halves := r.sent[len(r.sent)-2:]

				for _, half := range halves {
					if bytes.Contains(half, []byte(host)) {
						t.Error("a half carries the whole name")
					}
				}
			}
		})
	}
}

// The trap this guards: asking for no repeats must still send exactly one copy,
// not none, so a zero Repeats is never taken for a count.
func TestCopiesGoOutAsManyTimesAsAsked(t *testing.T) {
	const host = "gateway.discord.gg"

	cases := map[string]struct {
		rule string
		want int
	}{
		"nothing asked":  {host + "=badseq:100000,decoy", 1},
		"asked for one":  {host + "=badseq:100000,decoy,repeats:1", 1},
		"asked for five": {host + "=badseq:100000,decoy,repeats:5", 5},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := &recorder{}

			if _, err := engineFor(t, true, nil, c.rule).forward(r, packet443(clientHello(host)), &divert.Addr{}); err != nil {
				t.Fatalf("forward: %v", err)
			}

			if len(r.sent) != c.want {
				t.Fatalf("sent %d copies, want %d", len(r.sent), c.want)
			}

			for i, out := range r.sent {
				if !bytes.Equal(out, r.sent[0]) {
					t.Errorf("copy %d differs from the first; repeats send the same packet again", i)
				}
			}
		})
	}
}

// Repeats multiply the copy, never the real hello or its halves.
func TestRepeatsDoNotMultiplyTheCut(t *testing.T) {
	const host = "gateway.discord.gg"

	r := &recorder{}

	sent, err := engineFor(t, true, nil, host+"=decoy,repeats:4,cut:name").forward(r, packet443(clientHello(host)), &divert.Addr{})
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	if !sent {
		t.Error("the cut did not take over sending")
	}

	if len(r.sent) != 6 {
		t.Fatalf("sent %d packets, want 4 copies and 2 halves", len(r.sent))
	}
}

func TestDryRunPutsNothingOnTheWire(t *testing.T) {
	const host = "updates.discord.com"

	r := &recorder{}

	sent, err := engineFor(t, false, nil, host+"=ttl:4,decoy,cut:name").forward(r, packet443(clientHello(host)), &divert.Addr{})
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	if sent || len(r.sent) != 0 {
		t.Errorf("a dry run sent %d packets and took over sending = %v", len(r.sent), sent)
	}
}

func TestOverlapGoesOutAsTwoPackets(t *testing.T) {
	const host = "updates.discord.com"

	r := &recorder{}
	pattern := clientHello("www.4pda.to")

	sent, err := engineFor(t, true, pattern, host+"=overlap:1").forward(r, packet443(clientHello(host)), &divert.Addr{})
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	if !sent {
		t.Error("overlap did not take over sending, so the original would follow its own halves")
	}

	if len(r.sent) != 2 {
		t.Fatalf("sent %d packets, want 2", len(r.sent))
	}

	if !bytes.Contains(r.sent[0], []byte("www.4pda.to")) {
		t.Error("the first packet does not carry the recorded hello")
	}
}

func TestOverlapWithoutAPatternIsRefused(t *testing.T) {
	const host = "updates.discord.com"

	r := &recorder{}

	sent, err := engineFor(t, true, nil, host+"=overlap:1").forward(r, packet443(clientHello(host)), &divert.Addr{})
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	if sent || len(r.sent) != 0 {
		t.Error("an overlap with no pattern put something on the wire")
	}
}

// The whole point of the recorded fake: what goes ahead is somebody else's
// hello, not the real one with its name painted over.
func TestRecordedHelloGoesAheadInsteadOfACopy(t *testing.T) {
	const host = "updates.discord.com"

	recorded := clientHello("www.google.com")
	r := &recorder{}

	set, err := rules.ParseAll([]string{host + "=fake,badsum,repeats:3,cut:start"})
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}

	e := New(Settings{Rules: set, Recorded: recorded, Wet: true})

	sent, err := e.forward(r, packet443(clientHello(host)), &divert.Addr{})
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	if !sent {
		t.Error("the cut did not take over sending")
	}

	if len(r.sent) != 5 {
		t.Fatalf("sent %d packets, want 3 fakes and 2 halves", len(r.sent))
	}

	for i, fake := range r.sent[:3] {
		if !bytes.Contains(fake, []byte("www.google.com")) {
			t.Errorf("fake %d does not carry the recorded name", i)
		}

		if bytes.Contains(fake, []byte(host)) {
			t.Errorf("fake %d still carries the real name", i)
		}
	}
}

func TestRecordedFakeWithoutAFileIsRefused(t *testing.T) {
	const host = "updates.discord.com"

	r := &recorder{}

	if _, err := engineFor(t, true, nil, host+"=fake").forward(r, packet443(clientHello(host)), &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	if len(r.sent) != 0 {
		t.Errorf("sent %d packets with no recording loaded", len(r.sent))
	}
}

// The point of disorder: the halves reach the wire back to front, so an inspector
// reading them in arrival order never sees the hello start where it should.
func TestDisorderSendsTheHalvesBackToFront(t *testing.T) {
	const host = "updates.discord.com"

	packet := packet443(clientHello(host))

	inOrder := &recorder{}
	if _, err := engineFor(t, true, nil, host+"=cut:name").forward(inOrder, packet, &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	backwards := &recorder{}
	if _, err := engineFor(t, true, nil, host+"=cut:name,disorder").forward(backwards, packet, &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	if len(inOrder.sent) != 2 || len(backwards.sent) != 2 {
		t.Fatalf("sent %d and %d packets, want two halves each", len(inOrder.sent), len(backwards.sent))
	}

	if !bytes.Equal(backwards.sent[0], inOrder.sent[1]) || !bytes.Equal(backwards.sent[1], inOrder.sent[0]) {
		t.Error("the halves went out in the same order as without disorder")
	}
}

// Disorder without a cut has nothing to turn around, and must not quietly pass
// for a way to bypass anything on its own.
func TestDisorderAloneIsNotAWay(t *testing.T) {
	if _, err := rules.Parse("discord.com=disorder"); err == nil {
		t.Error("a rule that only turns nothing around was accepted")
	}
}

func TestOverlapGoesOutBackToFrontToo(t *testing.T) {
	const host = "updates.discord.com"

	packet := packet443(clientHello(host))
	pattern := clientHello("www.4pda.to")

	inOrder := &recorder{}
	if _, err := engineFor(t, true, pattern, host+"=overlap:1").forward(inOrder, packet, &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	backwards := &recorder{}
	if _, err := engineFor(t, true, pattern, host+"=overlap:1,disorder").forward(backwards, packet, &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	if !bytes.Equal(backwards.sent[0], inOrder.sent[1]) {
		t.Error("the recorded hello still went first")
	}
}

// A hello too big for one packet declares a record longer than it carries.
func splitHello(host string) []byte {
	payload := clientHello(host)
	binary.BigEndian.PutUint16(payload[3:5], binary.BigEndian.Uint16(payload[3:5])+20)

	return packet443(payload)
}

// The check this replaces lived here and turned a decoy of another size into a
// fatal error, so the whole point of rebuilding the hello never ran once.
func TestDecoyMayBeShorterThanTheRealName(t *testing.T) {
	const host = "updates.discord.com"

	for _, name := range []string{"mail.ru", "ya.ru", "a.io", "xxxxxxxx.google.com"} {
		t.Run(name, func(t *testing.T) {
			r := &recorder{}

			if _, err := engineFor(t, true, nil, host+"=decoy:"+name).forward(r, packet443(clientHello(host)), &divert.Addr{}); err != nil {
				t.Fatalf("forward: %v", err)
			}

			if len(r.sent) != 1 {
				t.Fatalf("sent %d packets, want the one copy", len(r.sent))
			}

			if !bytes.Contains(r.sent[0], []byte(name)) {
				t.Error("the copy does not wear the decoy")
			}
		})
	}
}

// A rule the engine cannot apply is reported and skipped: one bad site must not
// take the whole run down with it.
func TestARuleThatCannotBeAppliedDoesNotStopTheRun(t *testing.T) {
	const host = "updates.discord.com"

	r := &recorder{}

	sent, err := engineFor(t, true, nil, host+"=decoy:mail.ru").forward(r, splitHello(host), &divert.Addr{})
	if err != nil {
		t.Fatalf("forward returned %v, want the run to carry on", err)
	}

	if sent || len(r.sent) != 0 {
		t.Errorf("sent %d packets for a hello it cannot rebuild", len(r.sent))
	}
}

// A same size decoy needs no rebuilding, so it works even on a hello we see only
// part of. That is the case discord.com actually presents.
func TestSameSizeDecoyWorksOnASplitHello(t *testing.T) {
	const host = "updates.discord.com"

	r := &recorder{}

	if _, err := engineFor(t, true, nil, host+"=decoy").forward(r, splitHello(host), &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	if len(r.sent) != 1 {
		t.Fatalf("sent %d packets, want the one copy", len(r.sent))
	}

	if bytes.Contains(r.sent[0], []byte(host)) {
		t.Error("the copy still carries the real name")
	}
}

// The order is the whole trick: the wrong name arrives where the real one sits,
// and the real one is written over it afterwards.
func TestHostFakeSendsFourPartsInOrder(t *testing.T) {
	const host = "updates.discord.com"

	r := &recorder{}

	sent, err := engineFor(t, true, nil, host+"=hostfake:mail.ru").forward(r, packet443(clientHello(host)), &divert.Addr{})
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	if !sent {
		t.Fatal("hostfake did not take over sending")
	}

	if len(r.sent) != 4 {
		t.Fatalf("sent %d packets, want before, wrong name, real name, after", len(r.sent))
	}

	wrong := worn(host, "mail.ru")

	if !bytes.Contains(r.sent[1], []byte(wrong)) {
		t.Errorf("packet 2 does not carry the made up name %q", wrong)
	}

	if bytes.Contains(r.sent[1], []byte(host)) {
		t.Error("packet 2 still carries the real name")
	}

	if !bytes.Contains(r.sent[2], []byte(host)) {
		t.Error("packet 3 does not write the real name back")
	}
}

// The name has to keep its size: the lengths inside a hello count it, and nothing
// here rewrites them.
func TestTheSwappedNameKeepsTheSize(t *testing.T) {
	cases := map[string]string{
		"mail.ru":           "updates.discord.com",
		"a.io":              "updates.discord.com",
		"www.google.com":    "ya.ru",
		"auto":              "updates.discord.com",
		"a-very-long.co.uk": "a.io",
	}

	for want, real := range cases {
		t.Run(want, func(t *testing.T) {
			if got := worn(real, want); len(got) != len(real) {
				t.Errorf("worn(%q, %q) = %q, %d bytes, want %d", real, want, got, len(got), len(real))
			}
		})
	}
}

// A longer name keeps its tail, the way the reference does it.
func TestALongerNameIsCutFromTheFront(t *testing.T) {
	if got := worn("gle.com", "google.com"); got != "gle.com" {
		t.Errorf("worn = %q, want the tail %q", got, "gle.com")
	}
}

func TestHostFakePutsTheRestBeforeTheRealNameWhenAsked(t *testing.T) {
	const host = "updates.discord.com"

	packet := packet443(clientHello(host))

	inOrder := &recorder{}
	if _, err := engineFor(t, true, nil, host+"=hostfake:mail.ru").forward(inOrder, packet, &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	swapped := &recorder{}
	if _, err := engineFor(t, true, nil, host+"=hostfake:mail.ru,disorder").forward(swapped, packet, &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	if !bytes.Equal(swapped.sent[2], inOrder.sent[3]) || !bytes.Equal(swapped.sent[3], inOrder.sent[2]) {
		t.Error("disorder did not move what follows the name ahead of the real name")
	}
}

func TestHostFakeRepeatsOnlyTheWrongName(t *testing.T) {
	const host = "updates.discord.com"

	r := &recorder{}

	if _, err := engineFor(t, true, nil, host+"=hostfake,repeats:3").forward(r, packet443(clientHello(host)), &divert.Addr{}); err != nil {
		t.Fatalf("forward: %v", err)
	}

	if len(r.sent) != 6 {
		t.Fatalf("sent %d packets, want before, three wrong names, real name, after", len(r.sent))
	}

	for i := 1; i <= 3; i++ {
		if !bytes.Equal(r.sent[i], r.sent[1]) {
			t.Errorf("repeat %d differs from the first", i)
		}
	}
}

// The bug this guards: spoiling a rule counts as wanting a forged copy, so a
// swapped name used to go out behind a whole fake hello nobody asked for.
func TestHostFakeSendsNoCopyOnTopOfItsParts(t *testing.T) {
	const host = "updates.discord.com"

	packet := packet443(clientHello(host))

	for _, rule := range []string{
		host + "=hostfake:mail.ru,badack:-66000",
		host + "=hostfake,badseq:100000",
		host + "=hostfake:mail.ru,badsum,ttl:4",
	} {
		t.Run(rule, func(t *testing.T) {
			r := &recorder{}

			if _, err := engineFor(t, true, nil, rule).forward(r, packet, &divert.Addr{}); err != nil {
				t.Fatalf("forward: %v", err)
			}

			if len(r.sent) != 4 {
				t.Fatalf("sent %d packets, want the four parts alone", len(r.sent))
			}
		})
	}
}

// Windows sends no timestamps unless told to. A rule asking to age one on a packet
// that carries none has to say so and leave the hello alone, not send it half done.
func TestHostFakeSaysWhenThereIsNoTimestampToAge(t *testing.T) {
	const host = "updates.discord.com"

	out := &lines{}

	set, err := rules.ParseAll([]string{host + "=hostfake:mail.ru,ts"})
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}

	r := &recorder{}

	sent, err := New(Settings{Rules: set, Wet: true, Report: out.say}).forward(r, packet443(clientHello(host)), &divert.Addr{})
	if err != nil {
		t.Fatalf("forward returned %v, want the run to carry on", err)
	}

	if sent || len(r.sent) != 0 {
		t.Errorf("sent %d packets for a hello it cannot age", len(r.sent))
	}

	var told bool

	for _, said := range out.all() {
		if strings.Contains(said, "cannot swap the name") {
			told = true
		}
	}

	if !told {
		t.Error("nothing was said about why the name was not swapped")
	}
}
