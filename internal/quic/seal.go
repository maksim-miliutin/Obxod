package quic

func (k Keys) Seal(aad, plaintext []byte, number int) ([]byte, error) {
	gcm, err := k.aead()
	if err != nil {
		return nil, err
	}

	return gcm.Seal(nil, k.nonce(number), plaintext, aad), nil
}
