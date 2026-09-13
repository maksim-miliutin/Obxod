package hello

import (
	"bytes"
	"encoding/binary"
	"testing"

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

func packetTo(port uint16, protocol byte, payload []byte) []byte {
	var transport []byte

	if protocol == ip.ProtocolUDP {
		transport = make([]byte, 8)
		binary.BigEndian.PutUint16(transport[2:4], port)
		binary.BigEndian.PutUint16(transport[4:6], uint16(8+len(payload)))
	}

	if protocol == ip.ProtocolTCP {
		transport = make([]byte, 20)
		binary.BigEndian.PutUint16(transport[0:2], 54321)
		binary.BigEndian.PutUint16(transport[2:4], port)
		binary.BigEndian.PutUint32(transport[4:8], 900100)
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

func TestFoundHost(t *testing.T) {
	hosts := []string{"gateway.discord.gg", "updates.discord.com", "www.youtube.com"}

	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			out, ok := Found(packetTo(443, ip.ProtocolTCP, clientHello(host)))
			if !ok {
				t.Fatal("hello went unrecognised")
			}

			if out.Host != host {
				t.Errorf("Host = %q, want %q", out.Host, host)
			}
		})
	}
}

func TestFoundRejects(t *testing.T) {
	cases := []struct {
		name   string
		packet []byte
	}{
		{"empty", nil},
		{"udp voice", packetTo(50021, ip.ProtocolUDP, bytes.Repeat([]byte{0xcd}, 200))},
		{"plain tcp data", packetTo(443, ip.ProtocolTCP, []byte{0x17, 0x03, 0x03, 0x00, 0x05})},
		{"bare ack", packetTo(443, ip.ProtocolTCP, nil)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, ok := Found(c.packet); ok {
				t.Error("recognised a hello where there is none")
			}
		})
	}
}

func TestFoundSurvivesTruncation(t *testing.T) {
	packet := packetTo(443, ip.ProtocolTCP, clientHello("gateway.discord.gg"))

	for cut := 0; cut <= len(packet); cut++ {
		out, ok := Found(packet[:cut])

		if ok && out.Host != "gateway.discord.gg" && cut == len(packet) {
			t.Fatalf("cut %d: host came out %q", cut, out.Host)
		}
	}
}

func TestFoundNameEnd(t *testing.T) {
	const host = "updates.discord.com"

	packet := packetTo(443, ip.ProtocolTCP, clientHello(host))

	out, ok := Found(packet)
	if !ok {
		t.Fatal("hello went unrecognised")
	}

	payload := packet[40:]

	if out.NameEnd <= 0 || out.NameEnd > len(payload) {
		t.Fatalf("NameEnd = %d, outside a payload of %d", out.NameEnd, len(payload))
	}

	// Everything before the split point ends with the name, nothing of it spills past.
	if !bytes.HasSuffix(payload[:out.NameEnd], []byte(host)) {
		t.Errorf("payload up to %d does not end with the host name", out.NameEnd)
	}
}

func TestFoundNamesTheAttempt(t *testing.T) {
	packet := packetTo(443, ip.ProtocolTCP, clientHello("gateway.discord.gg"))

	out, ok := Found(packet)
	if !ok {
		t.Fatal("hello went unrecognised")
	}

	if out.SrcPort != 54321 {
		t.Errorf("SrcPort = %d, want 54321", out.SrcPort)
	}

	if out.Seq != 900100 {
		t.Errorf("Seq = %d, want 900100", out.Seq)
	}
}

// The port belongs to the filter, not here: a hello on 8443 is still a hello,
// and Discord speaks TLS on 2053, 2083, 2087, 2096 and 8443 as well as 443.
func TestFoundDoesNotCareAboutThePort(t *testing.T) {
	for _, port := range []uint16{443, 2053, 8443} {
		out, ok := Found(packetTo(port, ip.ProtocolTCP, clientHello("discord.media")))
		if !ok {
			t.Errorf("a hello to port %d went unrecognised", port)
		}

		if out.Host != "discord.media" {
			t.Errorf("Host = %q, want discord.media", out.Host)
		}
	}
}
