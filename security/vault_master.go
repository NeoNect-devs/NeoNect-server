package security

import (
	"NeoNect/internal/config"
	"crypto/sha256"
	"log"
	"os"
)

func loadAndDecryptMasterKey(keyPath string) ([]byte, error) {
	cipherData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	bootstrapKey := getSystemBootstrapKey()
	vault := NewVault(bootstrapKey)

	return vault.OpenData(cipherData)
}

func getSystemBootstrapKey() []byte {
	const envKey = "NEONECT_BOOTSTRAP_KEY"

	keyValue, exists := os.LookupEnv(envKey)
	if !exists || keyValue == "" {
		log.Fatal("Critical: NEONECT_BOOTSTRAP_KEY environment variable is not set")
	}

	return []byte(keyValue)
}

func deriveEngineKey(masterSecret []byte) []byte {
	engineKey := sha256.Sum256(masterSecret)
	return engineKey[:]
}

func CreateMasterKey(path string, keyData []byte) error {
	bootstrapKey := getSystemBootstrapKey()
	vault := NewVault(bootstrapKey)

	sealedData, err := vault.SealData(keyData)
	if err != nil {
		return err
	}

	return os.WriteFile(path, sealedData, config.FilePerm)
}
