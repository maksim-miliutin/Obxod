//go:build windows

package divert

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	dll       = syscall.NewLazyDLL("WinDivert.dll")
	procOpen  = dll.NewProc("WinDivertOpen")
	procClose = dll.NewProc("WinDivertClose")
)

type Handle struct {
	raw syscall.Handle
}

func Open(filter string, mode Mode) (*Handle, error) {
	if filter == "" {
		return nil, ErrEmptyFilter
	}

	flags, err := flagsFor(mode)
	if err != nil {
		return nil, err
	}

	text, err := syscall.BytePtrFromString(filter)
	if err != nil {
		return nil, fmt.Errorf("divert: filter: %w", err)
	}

	// Call is marked uintptrescapes, so the filter stays alive for the whole call.
	raw, _, lastErr := procOpen.Call(
		uintptr(unsafe.Pointer(text)),
		uintptr(layerNetwork),
		uintptr(priority),
		uintptr(flags),
	)

	if syscall.Handle(raw) == syscall.InvalidHandle {
		return nil, openFailure(lastErr)
	}

	return &Handle{raw: syscall.Handle(raw)}, nil
}

func (h *Handle) Close() error {
	if h == nil || h.raw == 0 {
		return nil
	}

	ok, _, lastErr := procClose.Call(uintptr(h.raw))
	h.raw = 0

	if ok == 0 {
		return fmt.Errorf("divert: close: %w", lastErr)
	}

	return nil
}

func openFailure(lastErr error) error {
	errno, ok := lastErr.(syscall.Errno)
	if !ok {
		return fmt.Errorf("divert: open: %w", lastErr)
	}

	switch errno {
	case 2:
		return fmt.Errorf("divert: open: WinDivert64.sys is missing next to the program (%w)", errno)
	case 5:
		return fmt.Errorf("divert: open: run as administrator (%w)", errno)
	case 87:
		return fmt.Errorf("divert: open: the driver rejected the filter, layer, priority or flags (%w)", errno)
	case 577:
		return fmt.Errorf("divert: open: the driver file is not signed (%w)", errno)
	case 1060:
		return fmt.Errorf("divert: open: the driver is not installed (%w)", errno)
	case 1275:
		return fmt.Errorf("divert: open: security software or the virtual machine blocks the driver (%w)", errno)
	}

	return fmt.Errorf("divert: open: %w", errno)
}
