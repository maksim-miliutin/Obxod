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
				"((tcp.DstPort == 443 and tcp.PayloadLength > 0) or " +
				"(udp.DstPort >= 19294 and udp.DstPort <= 19344) or " +
				"(udp.DstPort >= 50000 and udp.DstPort <= 50100))",
		},
		{
			"no voice at all",
			nil,
			"outbound and not loopback and not impostor and ip and " +
				"((tcp.DstPort == 443 and tcp.PayloadLength > 0))",
		},
		{
			"a single port needs no range",
			[]PortRange{{50000, 50000}},
			"outbound and not loopback and not impostor and ip and " +
				"((tcp.DstPort == 443 and tcp.PayloadLength > 0) or udp.DstPort == 50000)",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Outbound(c.voice, false)
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
			got, err := Outbound(c.voice, false)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}

			if got != "" {
				t.Errorf("got %q, want no filter at all", got)
			}
		})
	}
}

func TestReplies(t *testing.T) {
	want := "inbound and ip and tcp.SrcPort == 443 and (tcp.Rst or tcp.PayloadLength > 0)"

	if got := Replies(); got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestRepliesLeaveLoopbackAlone(t *testing.T) {
	// WinDivert never reports loopback on the inbound path, so naming it there would mislead.
	if strings.Contains(Replies(), "loopback") {
		t.Error("Replies names loopback, which cannot match inbound")
	}
}

func TestBracketsBalance(t *testing.T) {
	outbound, err := Outbound(discordVoice, false)
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	for _, f := range []string{outbound, Replies()} {
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
	f, err := Outbound(discordVoice, false)
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
	with, err := Outbound(discordVoice, true)
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	if !strings.Contains(with, "udp.DstPort == 443") {
		t.Error("quic was asked for but 443 over udp is not in the filter")
	}

	without, err := Outbound(discordVoice, false)
	if err != nil {
		t.Fatalf("Outbound: %v", err)
	}

	if strings.Contains(without, "udp.DstPort == 443") {
		t.Error("quic was not asked for yet 443 over udp is in the filter")
	}
}
