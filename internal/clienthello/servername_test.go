package clienthello

import (
	"bytes"
	"errors"
	"testing"
)

func serverNameOf(t *testing.T, payload []byte) ServerName {
	t.Helper()

	h, err := Parse(payload)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	name, err := h.ServerName()
	if err != nil {
		t.Fatalf("ServerName: %v", err)
	}

	return name
}

func TestServerNameHost(t *testing.T) {
	hosts := []string{
		"gateway.discord.gg",
		"updates.discord.com",
		"a.io",
		"very-long-name-that-stretches-past-a-hundred-and-twenty-eight-bytes-just-to-be-sure.example.com",
	}

	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			if got := serverNameOf(t, build(host)).Host; got != host {
				t.Errorf("Host = %q, want %q", got, host)
			}
		})
	}
}

func TestServerNameOffsetPointsAtTheName(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
	}{
		{"sni alone", build("gateway.discord.gg")},
		{"one extension ahead", build("gateway.discord.gg", ext(0x002b, []byte{0x02, 0x03, 0x04}))},
		{"three extensions ahead", build("gateway.discord.gg",
			ext(0x0017, nil),
			ext(0x002b, bytes.Repeat([]byte{0xcd}, 40)),
			ext(0x000d, bytes.Repeat([]byte{0xef}, 300)),
		)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			name := serverNameOf(t, c.payload)

			got := c.payload[name.Offset : name.Offset+len(name.Host)]
			if string(got) != name.Host {
				t.Errorf("payload at offset %d = %q, want %q", name.Offset, got, name.Host)
			}

			if want := bytes.Index(c.payload, []byte(name.Host)); name.Offset != want {
				t.Errorf("Offset = %d, want %d", name.Offset, want)
			}
		})
	}
}

func TestServerNameSkipsUnknownNameType(t *testing.T) {
	list := []byte{0x00, 0x0c, 0x07, 0x00, 0x02, 0xaa, 0xbb}
	list = append(list, nameTypeHost, 0x00, 0x04)
	list = append(list, []byte("a.io")...)

	payload := build("", ext(extServerName, list))

	name := serverNameOf(t, payload)
	if name.Host != "a.io" {
		t.Errorf("Host = %q, want the host_name entry", name.Host)
	}

	if got := payload[name.Offset : name.Offset+len(name.Host)]; string(got) != "a.io" {
		t.Errorf("payload at offset %d = %q, want a.io", name.Offset, got)
	}
}

func TestServerNameErrors(t *testing.T) {
	full := build("gateway.discord.gg")

	lyingSession := build("gateway.discord.gg")
	lyingSession[offSessionLen] = 0xff

	cases := []struct {
		name    string
		payload []byte
		want    error
	}{
		{"no extensions at all", build(""), ErrNoServerName},
		{"only an unknown extension", build("", ext(0x002b, []byte{0x01, 0x02})), ErrNoServerName},
		{"session id longer than the hello", lyingSession, ErrMalformed},
		{"cut in the middle of the name", full[:len(full)-4], ErrMalformed},
		{"cut before the extensions", full[:12], ErrMalformed},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, err := Parse(c.payload)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if _, err := h.ServerName(); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

const (
	offSessionLen = recordHeaderLen + handshakeHeaderLen + legacyVersionLen + randomLen
	offSuitesLen  = offSessionLen + 1
	offCompLen    = offSuitesLen + 2 + 2
	offExtBlock   = offCompLen + 1 + 1
	offFirstExt   = offExtBlock + 2
)

func TestServerNameLyingLengths(t *testing.T) {
	suites := build("gateway.discord.gg")
	suites[offSuitesLen] = 0xff

	compression := build("gateway.discord.gg")
	compression[offCompLen] = 0xff

	unknownAhead := build("gateway.discord.gg", ext(0x002b, []byte{0x01, 0x02, 0x03}))

	cases := []struct {
		name    string
		payload []byte
		want    error
	}{
		{"cipher suites outlast the hello", suites, ErrMalformed},
		{"compression list outlasts the hello", compression, ErrMalformed},
		{"no extensions block", build("gateway.discord.gg")[:offExtBlock], ErrNoServerName},
		{"extension header cut in half", build("gateway.discord.gg")[:offFirstExt+3], ErrMalformed},
		{"unknown extension outlasts the hello", unknownAhead[:offFirstExt+4], ErrMalformed},
		{"server name list has no length", build("", ext(extServerName, nil)), ErrMalformed},
		{"list length but no entry", build("", ext(extServerName, []byte{0x00, 0x03})), ErrNoServerName},
		{"entry type but no length", build("", ext(extServerName, []byte{0x00, 0x03, nameTypeHost})), ErrMalformed},
		{"name longer than the entry", build("", ext(extServerName, []byte{0x00, 0x03, nameTypeHost, 0x00, 0x09})), ErrMalformed},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, err := Parse(c.payload)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if _, err := h.ServerName(); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestServerNameSurvivesEveryTruncation(t *testing.T) {
	payload := build("gateway.discord.gg",
		ext(0x0017, nil),
		ext(0x002b, bytes.Repeat([]byte{0xcd}, 40)),
	)

	for cut := 0; cut <= len(payload); cut++ {
		h, err := Parse(payload[:cut])
		if err != nil {
			continue
		}

		name, err := h.ServerName()
		if err != nil {
			continue
		}

		if name.Offset+len(name.Host) > cut {
			t.Fatalf("cut %d: name runs to %d, past what was captured", cut, name.Offset+len(name.Host))
		}

		if got := string(payload[name.Offset : name.Offset+len(name.Host)]); got != name.Host {
			t.Fatalf("cut %d: payload holds %q, name says %q", cut, got, name.Host)
		}
	}
}

func FuzzServerName(f *testing.F) {
	f.Add(build("gateway.discord.gg"))
	f.Add(build("updates.discord.com", ext(0x002b, []byte{0x01})))
	f.Add(build(""))
	f.Add([]byte{0x16, 0x03, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, payload []byte) {
		h, err := Parse(payload)
		if err != nil {
			return
		}

		name, err := h.ServerName()
		if err != nil {
			return
		}

		if name.Offset < 0 || name.Offset+len(name.Host) > len(payload) {
			t.Fatalf("name at %d+%d lies outside a payload of %d", name.Offset, len(name.Host), len(payload))
		}

		if got := string(payload[name.Offset : name.Offset+len(name.Host)]); got != name.Host {
			t.Fatalf("payload holds %q, name says %q", got, name.Host)
		}
	})
}
