package divert

import (
	"encoding/binary"
	"errors"
)

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
	ErrEmptyBuffer = errors.New("divert: nowhere to put the packet")
	ErrNoPacket    = errors.New("divert: nothing to send")
	ErrNotWindows  = errors.New("divert: WinDivert runs on Windows only")
)

const AddrLen = 80

// Handed back to the driver untouched: it reads Outbound, Impostor, the checksum
// flags and the interface out of it when injecting.
type Addr [AddrLen]byte

const (
	bitSniffed = iota + 16
	bitOutbound
	bitLoopback
	bitImpostor
	bitIPv6
)

func (a *Addr) Sniffed() bool  { return a.bit(bitSniffed) }
func (a *Addr) Outbound() bool { return a.bit(bitOutbound) }
func (a *Addr) Loopback() bool { return a.bit(bitLoopback) }
func (a *Addr) Impostor() bool { return a.bit(bitImpostor) }
func (a *Addr) IPv6() bool     { return a.bit(bitIPv6) }

func (a *Addr) bit(n int) bool {
	return binary.LittleEndian.Uint32(a[8:12])&(1<<n) != 0
}

func flagsFor(m Mode) (uint64, error) {
	switch m {
	case Modify:
		return 0, nil
	case Sniff:
		return flagSniff | flagRecvOnly, nil
	}

	return 0, ErrUnknownMode
}
