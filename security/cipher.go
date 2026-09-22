package security

type SecureVault struct {
	masterKey []byte
}

func NewVault(key []byte) *SecureVault {
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)

	return &SecureVault{
		masterKey: keyCopy,
	}
}

func (v *SecureVault) SealData(rawData []byte) ([]byte, error) {
	aead, err := v.initializeGCM()
	if err != nil {
		return nil, err
	}

	nonce, err := v.generateNonce(aead.NonceSize())
	if err != nil {
		return nil, err
	}
	defer v.scrubBuffer(nonce)

	return aead.Seal(nonce, nonce, rawData, nil), nil
}

func (v *SecureVault) OpenData(sealedData []byte) ([]byte, error) {
	aead, err := v.initializeGCM()
	if err != nil {
		return nil, err
	}

	nonceSize := aead.NonceSize()
	if len(sealedData) < nonceSize {
		return nil, ErrTruncatedPayload
	}

	nonce := sealedData[:nonceSize]
	ciphertext := sealedData[nonceSize:]

	return aead.Open(nil, nonce, ciphertext, nil)
}

func (v *SecureVault) scrubBuffer(data []byte) {
	if data == nil {
		return
	}
	for i := range data {
		data[i] = 0
	}
}
