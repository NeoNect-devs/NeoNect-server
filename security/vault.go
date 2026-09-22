package security

import (
	"crypto/sha256"
	"encoding/hex"
)

type SystemVault struct {
	vaultEngine *SecureVault
}

func NewSystemVault(masterSecretPath string) (*SystemVault, error) {
	masterKey, err := loadAndDecryptMasterKey(masterSecretPath)
	if err != nil {
		return nil, err
	}
	return &SystemVault{
		vaultEngine: NewVault(deriveEngineKey(masterKey)),
	}, nil
}

func (v *SystemVault) Protect(data string) ([]byte, error) {
	return v.vaultEngine.SealData([]byte(data))
}

func (v *SystemVault) Reveal(encryptedData []byte) (string, error) {
	decryptedBytes, err := v.vaultEngine.OpenData(encryptedData)
	if err != nil {
		return "", err
	}
	return string(decryptedBytes), nil
}

func (v *SystemVault) Hash(data string) string {
	hasher := sha256.New()
	hasher.Write([]byte(data))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (v *SystemVault) VerifyIntegrity(seed string) bool {
	protectedBlob, err := v.Protect(seed)
	if err != nil {
		return false
	}
	revealedBlob, err := v.Reveal(protectedBlob)
	if err != nil {
		return false
	}
	return revealedBlob == seed
}
