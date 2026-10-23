package filter

import (
	"errors"
	"strings"
	"testing"
)

var discordVoice = []PortRange{{19294, 19344}, {50000, 50100}}

func TestOutbound(t *testing.T) {
	cases := []struct {
		name  string
		voice []PortRange
		want  string
	}{
		{
			"discord voice",
			discordVoice,
			"outbound and not loopback and not impostor and ip and " +
				"((tcp.DstPort == 443 and tcp.Payload[0] == 0x16) or " +
				"(udp.DstPort >= 19294 and udp.DstPort <= 19344) or " +
				"(udp.DstPort >= 50000 and udp.DstPort <= 50100)) and " +
				"(ip.DstAddr <= 25.255.255.255 or ip.DstAddr >= 27.0.0.0)",
		},
		{
			"no voice at all",
			nil,
			"outbound and not loopback and not impostor and ip and " +
				"((tcp.DstPort == 443 and tcp.Payload[0] == 0x16)) and " +
				"(ip.DstAddr <= 25.255.255.255 or ip.DstAddr >= 27.0.0.0)",
		},
		{
			"a single port needs no range",
			[]PortRange{{50000, 50000}},
			"outbound and not loopback and not impostor and ip and " +
				"((tcp.DstPort == 443 and tcp.Payload[0] == 0x16) or udp.DstPort == 50000) and " +
				"(ip.DstAddr <= 25.255.255.255 or ip.DstAddr >= 27.0.0.0)",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Outbound(Ports{TCP: []uint16{443}, Voice: c.voice, QUIC: false})
			if err != nil {
				t.Fatalf("Outbound: %v", err)
			}

			if got != c.want {
				t.Errorf("\n got %s\nwant %s", got, c.want)
			}
		})
	}
}

func TestOutboundErrors(t *testing.T) {
	cases := []struct {
		name  string
		voice []PortRange
		want  error
	}{
		{"ends before it starts", []PortRange{{50100, 50000}}, ErrEmptyRange},
		{"zero start", []PortRange{{0, 50000}}, ErrZeroPort},
		{"zero end", []PortRange{{50000, 0}}, ErrZeroPort},
		{"one good range then a bad one", []PortRange{{19294, 19344}, {3, 2}}, ErrEmptyRange},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Outbound(Ports{TCP: []uint16{443}, Voice: c.voice, QUIC: false})
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}

			if got != "" {
				t.Errorf("got %q, want no filter at all", got)
			}
		})
	}
}

func repliesFor(t *testing.T, ports ...uint16) string {
	t.Helper()

	got, err := Replies(ports)
	if err != nil {
		t.Fatalf("Replies: %v", err)
	}

	return got
}

func TestReplies(t *testing.T) {
	want := "inbound and ip and (tcp.SrcPort == 443) and (tcp.Rst or tcp.Fin or tcp.PayloadLength > 0)"

	if got := repliesFor(t, 443); got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestRepliesWatchesEveryPortWeCatch(t *testing.T) {
	got := repliesFor(t, 443, 2053, 8443)

	for _, port := range []string{"443", "2053", "8443"} {
		if !strings.Contains(got, "tcp.SrcPort == "+port) {
			t.Errorf("replies from %s would go unseen", port)
		}
	}
}

func TestRepliesNeedsPorts(t *testing.T) {
	if _, err := Replies(nil); !errors.Is(err, ErrNoPorts) {
		t.Errorf("err = %v, want %v", err, ErrNoPorts)
	}
}

func TestOutboundCatchesEveryTCPPort(t *testing.T) {
	got, err := Outbound(Ports{TCP: []uint16{443, 2053, 2083, 2087, 2096, 8443}})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	for _, port := range []string{"443", "2053", "2083", "2087", "2096", "8443"} {
		if !strings.Contains(got, "tcp.DstPort == "+port) {
			t.Errorf("hellos to %s would go uncaught", port)
		}
	}
}

func TestOutboundNeedsSomethingToCatch(t *testing.T) {
	if _, err := Outbound(Ports{}); !errors.Is(err, ErrNoPorts) {
		t.Errorf("err = %v, want %v", err, ErrNoPorts)
	}
}

func TestOutboundRejectsPortZero(t *testing.T) {
	if _, err := Outbound(Ports{TCP: []uint16{443, 0}}); !errors.Is(err, ErrZeroPort) {
		t.Errorf("err = %v, want %v", err, ErrZeroPort)
	}
}

func TestRepliesLeaveLoopbackAlone(t *testing.T) {
	// WinDivert never reports loopback on the inbound path, so naming it there would mislead.
	if strings.Contains(repliesFor(t, 443), "loopback") {
		t.Error("Replies names loopback, which cannot match inbound")
	}
}

func TestBracketsBalance(t *testing.T) {
	outbound, err := Outbound(Ports{TCP: []uint16{443}, Voice: discordVoice, QUIC: false})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	for _, f := range []string{outbound, repliesFor(t, 443)} {
		depth := 0

		for _, r := range f {
			if r == '(' {
				depth++
			}

			if r == ')' {
				depth--
			}

			if depth < 0 {
				t.Fatalf("%s closes a bracket that was never opened", f)
			}
		}

		if depth != 0 {
			t.Errorf("%s leaves %d brackets open", f, depth)
		}
	}
}

func TestOutboundKeepsOurOwnPacketsOut(t *testing.T) {
	f, err := Outbound(Ports{TCP: []uint16{443}, Voice: discordVoice, QUIC: false})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	for _, guard := range []string{"not loopback", "not impostor", "outbound", "ip"} {
		if !strings.Contains(f, guard) {
			t.Errorf("filter lost %q", guard)
		}
	}
}

func TestOutboundCatchesQUIC(t *testing.T) {
	with, err := Outbound(Ports{TCP: []uint16{443}, Voice: discordVoice, QUIC: true})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	if !strings.Contains(with, "udp.DstPort == 443") {
		t.Error("quic was asked for but 443 over udp is not in the filter")
	}

	without, err := Outbound(Ports{TCP: []uint16{443}, Voice: discordVoice, QUIC: false})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	if strings.Contains(without, "udp.DstPort == 443") {
		t.Error("quic was not asked for yet 443 over udp is in the filter")
	}
}

// An opening syn carries no handshake byte to match, so with syndata on it needs
// a clause of its own; without it, syns stay out of the engine entirely.
func TestOutboundCatchesSynsOnlyWhenAsked(t *testing.T) {
	with, err := Outbound(Ports{TCP: []uint16{443}, SynData: true})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	if !strings.Contains(with, "tcp.DstPort == 443 and tcp.Syn and not tcp.Ack") {
		t.Errorf("syndata was asked for but opening syns go uncaught: %s", with)
	}

	without, err := Outbound(Ports{TCP: []uint16{443}})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	if strings.Contains(without, "tcp.Syn") {
		t.Errorf("syns are caught though syndata was not asked for: %s", without)
	}
}

// Each watched port needs its own syn clause, the way each gets its own handshake one.
func TestOutboundCatchesSynsOnEveryPort(t *testing.T) {
	got, err := Outbound(Ports{TCP: []uint16{443, 8443}, SynData: true})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	for _, port := range []string{"443", "8443"} {
		if !strings.Contains(got, "tcp.DstPort == "+port+" and tcp.Syn and not tcp.Ack") {
			t.Errorf("opening syns to %s would go uncaught: %s", port, got)
		}
	}
}

// The bug this guards: the watcher learned to read fin while the filter still
// dropped it, so every finished connection looked like a killed one.
func TestRepliesLetsFinThrough(t *testing.T) {
	if !strings.Contains(repliesFor(t, 443), "tcp.Fin") {
		t.Error("a fin would never reach the watcher")
	}
}

func TestOutboundCatchesOnlyHandshakes(t *testing.T) {
	got, err := Outbound(Ports{TCP: []uint16{443}})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	if !strings.Contains(got, "tcp.Payload[0] == 0x16") {
		t.Errorf("the filter no longer narrows to the tls handshake: %s", got)
	}

	if strings.Contains(got, "PayloadLength") {
		t.Errorf("the filter still catches every data packet: %s", got)
	}
}

func TestOutboundSkipsRadminRange(t *testing.T) {
	got, err := Outbound(Ports{TCP: []uint16{443}, QUIC: true})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	if !strings.Contains(got, ") and (ip.DstAddr <= 25.255.255.255 or ip.DstAddr >= 27.0.0.0)") {
		t.Errorf("radmin range is not skipped, or not anded with the catch: %s", got)
	}
}

// Fake mode narrows udp 443 to the handshake's first packet; drop mode still takes
// every quic datagram.
func TestOutboundCatchesOnlyQUICInitials(t *testing.T) {
	narrow, err := Outbound(Ports{TCP: []uint16{443}, QUIC: true, QUICInitialsOnly: true})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	if !strings.Contains(narrow, "udp.Payload[0] >= 0xc0 and udp.Payload[0] <= 0xcf") {
		t.Errorf("fake mode does not narrow udp 443 to initials: %s", narrow)
	}

	wide, err := Outbound(Ports{TCP: []uint16{443}, QUIC: true})
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	if strings.Contains(wide, "udp.Payload") {
		t.Errorf("drop mode should catch every quic datagram, not just initials: %s", wide)
	}
}
