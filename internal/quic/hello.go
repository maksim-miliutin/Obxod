package quic

import (
	"errors"

	"obxod/internal/clienthello"
)

var (
	ErrNoCrypto        = errors.New("quic: no crypto frame at offset zero")
	ErrUnexpectedFrame = errors.New("quic: unexpected frame in an initial payload")
)

type Name struct {
	Host   string
	Offset int
}

func ReadHello(payload []byte) (Name, error) {
	data, start, err := cryptoAtZero(payload)
	if err != nil {
		return Name{}, err
	}

	hello, err := clienthello.FromHandshake(data)
	if err != nil {
		return Name{}, err
	}

	found, err := hello.ServerName()
	if err != nil {
		return Name{}, err
	}

	return Name{Host: found.Host, Offset: start + found.Offset}, nil
}

func cryptoAtZero(payload []byte) ([]byte, int, error) {
	at := 0

	for at < len(payload) {
		switch payload[at] {
		case 0x00, 0x01: // padding and ping carry nothing
			at++
		case 0x06: // crypto
			at++

			offset, next, ok := varint(payload, at)
			if !ok {
				return nil, 0, ErrShort
			}
			at = next

			length, next, ok := varint(payload, at)
			if !ok {
				return nil, 0, ErrShort
			}
			at = next

			if length > len(payload)-at {
				return nil, 0, ErrShort
			}

			if offset == 0 {
				return payload[at : at+length], at, nil
			}

			at += length
		default:
			return nil, 0, ErrUnexpectedFrame
		}
	}

	return nil, 0, ErrNoCrypto
}
