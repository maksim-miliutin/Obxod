//go:build !windows

package divert

import (
	"errors"
	"testing"
)

// Guarded by the build tag: on Windows these calls would reach the real driver.

func TestOpenSaysWhereItRuns(t *testing.T) {
	if _, err := Open("outbound and tcp.DstPort == 443", Modify); !errors.Is(err, ErrNotWindows) {
		t.Errorf("err = %v, want %v", err, ErrNotWindows)
	}
}

func TestRecvSaysWhereItRuns(t *testing.T) {
	var h Handle

	if _, _, err := h.Recv(make([]byte, 1500)); !errors.Is(err, ErrNotWindows) {
		t.Errorf("err = %v, want %v", err, ErrNotWindows)
	}
}

func TestSendSaysWhereItRuns(t *testing.T) {
	var h Handle
	var a Addr

	if err := h.Send([]byte{0x45}, &a); !errors.Is(err, ErrNotWindows) {
		t.Errorf("err = %v, want %v", err, ErrNotWindows)
	}
}
