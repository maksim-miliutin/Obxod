package main

import (
	"bytes"
	"encoding/binary"
	"testing"

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

	extensions := binary.BigEndian.AppendUint16(nil, uint16(len(sni)))
	extensions = append(extensions, sni...)

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
