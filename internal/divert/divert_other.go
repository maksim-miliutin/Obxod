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

func (h *Handle) Close() error {
	return nil
}
