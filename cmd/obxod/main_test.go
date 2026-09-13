package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"obxod/internal/cut"
	"obxod/internal/forge"
	"obxod/internal/hello"
	"obxod/internal/ip"
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
