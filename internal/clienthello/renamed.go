package clienthello

import (
	"encoding/binary"
	"errors"
)

var (
	ErrTruncated = errors.New("clienthello: the record is cut short, renaming needs it whole")
	ErrNameSize  = errors.New("clienthello: the new name makes a length overflow its field")
)

// Renamed returns the payload with another host name in place of the real one.
// Unlike a same-length swap, six declared lengths count the name in and move with it.
func (h Hello) Renamed(name string) ([]byte, error) {
	if len(h.Record) != recordHeaderLen+h.RecordLen {
		return nil, ErrTruncated
	}

	found, err := h.ServerName()
	if err != nil {
		return nil, err
	}

	delta := len(name) - len(found.Host)

	out := make([]byte, 0, len(h.payload)+delta)
	out = append(out, h.payload[:found.Offset]...)
	out = append(out, name...)
	out = append(out, h.payload[found.Offset+len(found.Host):]...)

	for _, at := range []int{recordLenAt, found.extensionsLenAt, found.extLenAt, found.listLenAt, found.nameLenAt} {
		if err := bump16(out, at, delta); err != nil {
			return nil, err
		}
	}

	if err := bump24(out, handshakeLenAt, delta); err != nil {
		return nil, err
	}

	return out, nil
}

const (
	recordLenAt    = 3
	handshakeLenAt = 6
)

func bump16(out []byte, at int, delta int) error {
	if at < 0 || at+2 > len(out) {
		return ErrMalformed
	}

	was := int(binary.BigEndian.Uint16(out[at : at+2]))

	now := was + delta
	if now < 0 || now > 0xffff {
		return ErrNameSize
	}

	binary.BigEndian.PutUint16(out[at:at+2], uint16(now))

	return nil
}

func bump24(out []byte, at int, delta int) error {
	if at < 0 || at+3 > len(out) {
		return ErrMalformed
	}

	was := int(out[at])<<16 | int(out[at+1])<<8 | int(out[at+2])

	now := was + delta
	if now < 0 || now > 0xffffff {
		return ErrNameSize
	}

	out[at] = byte(now >> 16)
	out[at+1] = byte(now >> 8)
	out[at+2] = byte(now)

	return nil
}
