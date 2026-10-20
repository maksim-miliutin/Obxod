package clienthello

import "testing"

func TestFromHandshakeFindsServerName(t *testing.T) {
	for _, host := range []string{"cdn.discordapp.com", "a.io"} {
		handshake := build(host)[recordHeaderLen:]

		h, err := FromHandshake(handshake)
		if err != nil {
			t.Fatalf("%s: FromHandshake: %v", host, err)
		}

		name, err := h.ServerName()
		if err != nil {
			t.Fatalf("%s: ServerName: %v", host, err)
		}

		if name.Host != host {
			t.Errorf("host = %q, want %q", name.Host, host)
		}

		if got := handshake[name.Offset : name.Offset+len(name.Host)]; string(got) != host {
			t.Errorf("offset %d points at %q, want %q", name.Offset, got, host)
		}
	}
}
