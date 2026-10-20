package quic

import "testing"

func appendVarint(b []byte, v int) []byte {
	if v < 64 {
		return append(b, byte(v))
	}

	return append(b, byte(0x40|v>>8), byte(v))
}

func clientHello(host string) []byte {
	list := []byte{0x00, byte(len(host) >> 8), byte(len(host))}
	list = append(list, host...)

	ext := []byte{0x00, 0x00}
	ext = append(ext, byte((2+len(list))>>8), byte(2+len(list)))
	ext = append(ext, byte(len(list)>>8), byte(len(list)))
	ext = append(ext, list...)

	body := []byte{0x03, 0x03}
	body = append(body, make([]byte, 32)...)
	body = append(body, 0x00)
	body = append(body, 0x00, 0x02, 0x13, 0x01)
	body = append(body, 0x01, 0x00)
	body = append(body, byte(len(ext)>>8), byte(len(ext)))
	body = append(body, ext...)

	hs := []byte{0x01, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}

	return append(hs, body...)
}

func cryptoPayload(host string) []byte {
	hs := clientHello(host)

	p := []byte{0x06}
	p = appendVarint(p, 0)
	p = appendVarint(p, len(hs))
	p = append(p, hs...)

	return append(p, 0, 0, 0, 0)
}

func TestReadHelloFindsTheHost(t *testing.T) {
	host := "cdn.discordapp.com"
	payload := cryptoPayload(host)

	name, err := ReadHello(payload)
	if err != nil {
		t.Fatal(err)
	}

	if name.Host != host {
		t.Errorf("host = %q, want %q", name.Host, host)
	}

	if got := payload[name.Offset : name.Offset+len(host)]; string(got) != host {
		t.Errorf("offset %d points at %q, want %q", name.Offset, got, host)
	}
}

func TestReadHelloRejectsUnknownFrame(t *testing.T) {
	if _, err := ReadHello([]byte{0x1a, 0x00}); err != ErrUnexpectedFrame {
		t.Errorf("err = %v, want ErrUnexpectedFrame", err)
	}
}
