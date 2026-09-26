package runner

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"obxod/internal/bypass"
	"obxod/internal/divert"
	"obxod/internal/filter"
)

// held stands in for a driver handle whose read blocks until the handle is
// closed, so a test can drive a run to its stop without a real WinDivert.
type held struct {
	closed chan struct{}
	closes atomic.Int32
	once   sync.Once
}

func newHeld() *held {
	return &held{closed: make(chan struct{})}
}

func (h *held) Recv(buf []byte) (int, divert.Addr, error) {
	<-h.closed

	return 0, divert.Addr{}, errors.New("handle closed")
}

func (h *held) Send(packet []byte, addr *divert.Addr) error {
	return nil
}

func (h *held) Close() error {
	h.closes.Add(1)
	h.once.Do(func() { close(h.closed) })

	return nil
}

// broken stands in for a driver that fails on its own, with no stop asked.
type broken struct{}

func (broken) Recv(buf []byte) (int, divert.Addr, error) {
	return 0, divert.Addr{}, errors.New("driver gone")
}

func (broken) Send(packet []byte, addr *divert.Addr) error {
	return nil
}

func session(wire bypass.Wire, eyes bypass.Eyes, handles ...io.Closer) *Session {
	return &Session{
		engine:  bypass.New(bypass.Settings{Report: func(string) {}}),
		wire:    wire,
		eyes:    eyes,
		handles: handles,
	}
}

func TestRunStopsCleanly(t *testing.T) {
	wire, eyes := newHeld(), newHeld()
	s := session(wire, eyes, wire, eyes)

	done := make(chan error, 1)
	go func() { done <- s.Run() }()

	s.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run after Stop = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Stop")
	}
}

func TestRunSurfacesADriverError(t *testing.T) {
	eyes := newHeld()
	defer eyes.Close()

	if err := session(broken{}, eyes).Run(); err == nil {
		t.Fatal("Run with a failing driver returned nil, want the error")
	}
}

func TestStopClosesEachHandleOnce(t *testing.T) {
	wire, eyes := newHeld(), newHeld()
	s := session(wire, eyes, wire, eyes)

	s.Stop()
	s.Stop()

	if wire.closes.Load() != 1 || eyes.closes.Load() != 1 {
		t.Errorf("closes: wire %d, eyes %d, want 1 each", wire.closes.Load(), eyes.closes.Load())
	}
}

func TestDefaultPortsGiveTheFilterSomethingToCatch(t *testing.T) {
	if _, err := filter.Replies(DefaultPorts()); err != nil {
		t.Fatalf("replies filter on default ports: %v", err)
	}

	if _, err := filter.Outbound(filter.Ports{TCP: DefaultPorts()}); err != nil {
		t.Fatalf("outbound filter on default ports: %v", err)
	}
}

func TestDefaultVoiceHasTheDiscordRanges(t *testing.T) {
	got := DefaultVoice()
	if len(got) != 2 || got[0].From != 19294 || got[1].To != 50100 {
		t.Fatalf("DefaultVoice = %v, want the two Discord ranges", got)
	}
}
