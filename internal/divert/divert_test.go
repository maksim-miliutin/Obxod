package divert

import (
	"errors"
	"testing"

	"obxod/internal/filter"
)

func TestFlagsFor(t *testing.T) {
	cases := []struct {
		name string
		mode Mode
		want uint64
	}{
		{"modify holds packets", Modify, 0},
		{"sniff only copies", Sniff, flagSniff | flagRecvOnly},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := flagsFor(c.mode)
			if err != nil {
				t.Fatalf("flagsFor: %v", err)
			}

			if got != c.want {
				t.Errorf("flags = %#x, want %#x", got, c.want)
			}
		})
	}
}

func TestFlagsForUnknownMode(t *testing.T) {
	if _, err := flagsFor(Mode(7)); !errors.Is(err, ErrUnknownMode) {
		t.Errorf("err = %v, want %v", err, ErrUnknownMode)
	}
}

func TestSniffNeverDrops(t *testing.T) {
	const flagDrop = 0x0002

	flags, err := flagsFor(Sniff)
	if err != nil {
		t.Fatalf("flagsFor: %v", err)
	}

	// The driver refuses a handle that both sniffs and drops.
	if flags&flagDrop != 0 {
		t.Error("sniffing mode asks the driver to drop packets as well")
	}
}

func TestOpenRefusesEmptyFilter(t *testing.T) {
	if _, err := Open("", Modify); !errors.Is(err, ErrEmptyFilter) {
		t.Errorf("err = %v, want %v", err, ErrEmptyFilter)
	}
}

func TestOpenRefusesUnknownMode(t *testing.T) {
	if _, err := Open("outbound and tcp.DstPort == 443", Mode(7)); !errors.Is(err, ErrUnknownMode) {
		t.Errorf("err = %v, want %v", err, ErrUnknownMode)
	}
}

func TestFiltersWeBuildPassTheGuard(t *testing.T) {
	outbound, err := filter.Outbound(filter.Ports{TCP: []uint16{443}, Voice: []filter.PortRange{{From: 19294, To: 19344}}})
	if err != nil {
		t.Fatalf("filter.Outbound: %v", err)
	}

	// Open is not called here on purpose: on Windows it would install the driver.
	watching, err := filter.Replies([]uint16{443})
	if err != nil {
		t.Fatalf("filter.Replies: %v", err)
	}

	for _, f := range []string{outbound, watching} {
		if f == "" {
			t.Error("built an empty filter, which Open refuses")
		}
	}
}

func TestCloseOnNothingIsQuiet(t *testing.T) {
	var h *Handle

	if err := h.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
