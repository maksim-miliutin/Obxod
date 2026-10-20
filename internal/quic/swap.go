package quic

import "errors"

var ErrNameLength = errors.New("quic: replacement name is a different length")

func SwapName(datagram []byte, name string) ([]byte, error) {
	head, err := ParseInitial(datagram)
	if err != nil {
		return nil, err
	}

	end := head.PNOffset + head.Length
	if end > len(datagram) {
		return nil, ErrShort
	}

	out := append([]byte(nil), datagram...)
	keys := ClientInitialKeys(head.DCID)

	revealed, err := Unprotect(out, head.PNOffset, keys.HP)
	if err != nil {
		return nil, err
	}

	aad := out[:revealed.HeaderLen]
	ciphertext := out[revealed.HeaderLen:end]

	plain, err := keys.Open(aad, ciphertext, revealed.Number)
	if err != nil {
		return nil, err
	}

	found, err := ReadHello(plain)
	if err != nil {
		return nil, err
	}

	if len(name) != len(found.Host) {
		return nil, ErrNameLength
	}

	copy(plain[found.Offset:found.Offset+len(name)], name)

	sealed, err := keys.Seal(aad, plain, revealed.Number)
	if err != nil {
		return nil, err
	}

	copy(out[revealed.HeaderLen:end], sealed)

	if err := Protect(out, head.PNOffset, keys.HP); err != nil {
		return nil, err
	}

	return out, nil
}
