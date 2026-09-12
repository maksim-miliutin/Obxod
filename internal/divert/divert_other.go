//go:build !windows

package divert

type Handle struct{}

func Open(filter string, mode Mode) (*Handle, error) {
	if filter == "" {
		return nil, ErrEmptyFilter
	}

	if _, err := flagsFor(mode); err != nil {
		return nil, err
	}

	return nil, ErrNotWindows
}

func (h *Handle) Recv(buf []byte) (int, Addr, error) {
	if len(buf) == 0 {
		return 0, Addr{}, ErrEmptyBuffer
	}

	return 0, Addr{}, ErrNotWindows
}

func (h *Handle) Send(packet []byte, addr *Addr) error {
	if len(packet) == 0 {
		return ErrNoPacket
	}

	return ErrNotWindows
}

func (h *Handle) Close() error {
	return nil
}
