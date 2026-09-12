package divert

import "errors"

type Mode int

const (
	Modify Mode = iota // packets are held until they are sent back
	Sniff              // copies arrive, nothing is held up, nothing can be sent
)

const (
	flagSniff    = 0x0001
	flagRecvOnly = 0x0004
)

const (
	layerNetwork = 0
	priority     = 0
)

var (
	ErrUnknownMode = errors.New("divert: unknown mode")
	ErrEmptyFilter = errors.New("divert: empty filter")
	ErrNotWindows  = errors.New("divert: WinDivert runs on Windows only")
)

func flagsFor(m Mode) (uint64, error) {
	switch m {
	case Modify:
		return 0, nil
	case Sniff:
		return flagSniff | flagRecvOnly, nil
	}

	return 0, ErrUnknownMode
}
