package clienthello

import (
	"encoding/binary"
	"errors"
)

const (
	extServerName = 0x0000
	nameTypeHost  = 0x00
)

const (
	legacyVersionLen = 2
	randomLen        = 32
)

var (
	ErrNoServerName = errors.New("clienthello: no host name in the hello")
	ErrMalformed    = errors.New("clienthello: a length runs past the captured bytes")
)

type ServerName struct {
	Host   string
	Offset int // bytes from the start of the payload, points at the name itself
}

func (h Hello) ServerName() (ServerName, error) {
	r := reader{data: h.Body, base: recordHeaderLen + handshakeHeaderLen}

	if !r.skip(legacyVersionLen + randomLen) {
		return ServerName{}, ErrMalformed
	}

	sessionLen, ok := r.u8()
	if !ok || !r.skip(sessionLen) {
		return ServerName{}, ErrMalformed
	}

	suitesLen, ok := r.u16()
	if !ok || !r.skip(suitesLen) {
		return ServerName{}, ErrMalformed
	}

	compressionLen, ok := r.u8()
	if !ok || !r.skip(compressionLen) {
		return ServerName{}, ErrMalformed
	}

	if _, ok := r.u16(); !ok {
		return ServerName{}, ErrNoServerName
	}

	for {
		kind, ok := r.u16()
		if !ok {
			return ServerName{}, ErrNoServerName
		}

		size, ok := r.u16()
		if !ok {
			return ServerName{}, ErrMalformed
		}

		if kind != extServerName {
			if !r.skip(size) {
				return ServerName{}, ErrMalformed
			}

			continue
		}

		list, ok := r.take(size)
		if !ok {
			return ServerName{}, ErrMalformed
		}

		return hostIn(reader{data: list, base: r.offset() - size})
	}
}

func hostIn(r reader) (ServerName, error) {
	if _, ok := r.u16(); !ok {
		return ServerName{}, ErrMalformed
	}

	for {
		kind, ok := r.u8()
		if !ok {
			return ServerName{}, ErrNoServerName
		}

		size, ok := r.u16()
		if !ok {
			return ServerName{}, ErrMalformed
		}

		name, ok := r.take(size)
		if !ok {
			return ServerName{}, ErrMalformed
		}

		if kind != nameTypeHost {
			continue
		}

		return ServerName{Host: string(name), Offset: r.offset() - size}, nil
	}
}

type reader struct {
	data []byte
	at   int
	base int // offset of data[0] within the payload
}

func (r *reader) offset() int {
	return r.base + r.at
}

func (r *reader) skip(n int) bool {
	if n < 0 || r.at+n > len(r.data) {
		return false
	}

	r.at += n

	return true
}

func (r *reader) take(n int) ([]byte, bool) {
	if n < 0 || r.at+n > len(r.data) {
		return nil, false
	}

	v := r.data[r.at : r.at+n]
	r.at += n

	return v, true
}

func (r *reader) u8() (int, bool) {
	if r.at+1 > len(r.data) {
		return 0, false
	}

	v := int(r.data[r.at])
	r.at++

	return v, true
}

func (r *reader) u16() (int, bool) {
	if r.at+2 > len(r.data) {
		return 0, false
	}

	v := int(binary.BigEndian.Uint16(r.data[r.at : r.at+2]))
	r.at += 2

	return v, true
}
