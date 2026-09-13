package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"obxod/internal/cut"
	"obxod/internal/divert"
	"obxod/internal/forge"
	"obxod/internal/hello"
	"obxod/internal/ip"
	"obxod/internal/rules"
	"strings"
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

type recorder struct {
	sent [][]byte
}

func (r *recorder) Send(packet []byte, addr *divert.Addr) error {
	r.sent = append(r.sent, append([]byte(nil), packet...))

	return nil
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

			set, err := rules.ParseAll([]string{c.rule})
			if err != nil {
				t.Fatalf("ParseAll: %v", err)
			}

			sent, err := forward(r, packet, addr, set, true)
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

func TestPlanSpreadsFlagsOverHosts(t *testing.T) {
	set, err := plan(nil, "discord.com, discord.gg", 0, 100000, false, "auto", "name")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if len(set) != 2 {
		t.Fatalf("plan made %d rules, want 2", len(set))
	}

	for _, r := range set {
		if r.BadSeq != 100000 || r.Decoy != "auto" || r.Cut != "name" {
			t.Errorf("rule %+v lost part of the flags", r)
		}
	}
}

func TestPlanPrefersExplicitRules(t *testing.T) {
	set, err := plan([]string{"discord.gg=ttl:2,cut:start"}, "ignored.example", 4, 0, false, "", "")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if len(set) != 1 || set[0].Host != "discord.gg" || set[0].TTL != 2 {
		t.Errorf("plan = %+v, want the rule as written", set)
	}
}

func TestPlanNeedsSomething(t *testing.T) {
	if _, err := plan(nil, "discord.com", 0, 0, false, "", ""); err == nil {
		t.Error("a plan with no way to bypass was accepted")
	}
}

func TestWatchesCoversSubdomains(t *testing.T) {
	watched := parseHosts("discord.com, discord.gg ,discordapp.com")

	hit := []string{
		"discord.com",
		"updates.discord.com",
		"cdn.discord.com",
		"gateway.discord.gg",
		"DISCORD.COM",
		"media.discordapp.com",
	}

	for _, host := range hit {
		if !watches(watched, host) {
			t.Errorf("%q went unwatched", host)
		}
	}

	miss := []string{
		"ya.ru",
		"notdiscord.com",
		"discord.com.evil.net",
		"google.com",
	}

	for _, host := range miss {
		if watches(watched, host) {
			t.Errorf("%q was watched but should not be", host)
		}
	}
}

func TestWatchesAll(t *testing.T) {
	watched := parseHosts("all")

	for _, host := range []string{"ya.ru", "discord.com", "anything.example"} {
		if !watches(watched, host) {
			t.Errorf("%q went unwatched under all", host)
		}
	}
}

func TestParseHostsDropsBlanks(t *testing.T) {
	if got := parseHosts(" , ,"); len(got) != 0 {
		t.Errorf("parseHosts = %v, want nothing", got)
	}
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

func TestIsQUIC(t *testing.T) {
	if !isQUIC(datagramTo(443)) {
		t.Error("a datagram to 443 was not taken for quic")
	}

	if isQUIC(datagramTo(50021)) {
		t.Error("a voice datagram was taken for quic")
	}

	if isQUIC(packet443(clientHello("discord.com"))) {
		t.Error("a tcp hello was taken for quic")
	}

	if isQUIC(nil) {
		t.Error("an empty packet was taken for quic")
	}
}
