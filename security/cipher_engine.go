package security

import (
	"NeoNect/internal/config"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

var (
	ErrInvalidKeySize   = errors.New("secure vault: invalid key size for AES-256")
	ErrTruncatedPayload = errors.New("secure vault: truncated payload")
)

func (v *SecureVault) checkKeyLength() error {
	if len(v.masterKey) != config.TokenEntropy {
		return ErrInvalidKeySize
	}
	return nil
}

func (v *SecureVault) initializeGCM() (cipher.AEAD, error) {
	if err := v.checkKeyLength(); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

func (v *SecureVault) generateNonce(size int) ([]byte, error) {
	nonce := make([]byte, size)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}
