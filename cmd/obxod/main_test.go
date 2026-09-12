package main

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func hello(host string) []byte {
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
	body = append(body, 0x00)
	body = append(body, 0x00, 0x02, 0x13, 0x01)
	body = append(body, 0x01, 0x00)
	body = append(body, extensions...)

	handshake := []byte{0x01, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}
	handshake = append(handshake, body...)

	record := []byte{0x16, 0x03, 0x01, 0x00, 0x00}
	binary.BigEndian.PutUint16(record[3:5], uint16(len(handshake)))

	return append(record, handshake...)
}

func packetTo(port uint16, protocol byte, payload []byte) []byte {
	var transport []byte

	if protocol == 17 {
		transport = make([]byte, 8)
		binary.BigEndian.PutUint16(transport[2:4], port)
		binary.BigEndian.PutUint16(transport[4:6], uint16(8+len(payload)))
	}

	if protocol == 6 {
		transport = make([]byte, 20)
		binary.BigEndian.PutUint16(transport[2:4], port)
		transport[12] = 5 << 4
		transport[13] = 0x18
	}

	transport = append(transport, payload...)

	packet := make([]byte, 20)
	packet[0] = 4<<4 | 5
	packet[8] = 64
	packet[9] = protocol
	copy(packet[12:16], []byte{192, 168, 1, 2})
	copy(packet[16:20], []byte{93, 184, 216, 34})
	packet = append(packet, transport...)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))

	return packet
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		name   string
		packet []byte
		want   string
	}{
		{"hello for discord", packetTo(443, 6, hello("gateway.discord.gg")), "tcp to 443, hello for gateway.discord.gg"},
		{"hello for youtube", packetTo(443, 6, hello("www.youtube.com")), "tcp to 443, hello for www.youtube.com"},
		{"plain tcp data", packetTo(443, 6, []byte{0x17, 0x03, 0x03, 0x00, 0x10}), "tcp to 443, 5 bytes"},
		{"voice datagram", packetTo(50021, 17, bytes.Repeat([]byte{0xcd}, 200)), "udp to 50021, 200 bytes"},
		{"empty", nil, "not ipv4"},
		{"version six", []byte{6 << 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, "not ipv4"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := describe(c.packet); got != c.want {
				t.Errorf("describe = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDescribeSurvivesTruncation(t *testing.T) {
	packet := packetTo(443, 6, hello("gateway.discord.gg"))

	for cut := 0; cut <= len(packet); cut++ {
		got := describe(packet[:cut])

		if strings.Contains(got, "gateway.discord.gg") && cut < len(packet) {
			continue
		}

		if got == "" {
			t.Fatalf("cut %d: describe said nothing", cut)
		}
	}
}
