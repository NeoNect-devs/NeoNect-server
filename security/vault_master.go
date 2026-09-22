package security

import (
	"NeoNect/internal/config"
	"crypto/sha256"
	"log"
	"os"
	"strings"
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
	const fileEnvKey = "NEONECT_BOOTSTRAP_KEY_FILE"

	if filePath, exists := os.LookupEnv(fileEnvKey); exists && filePath != "" {
		keyData, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("Critical: Failed to read %s: %v", fileEnvKey, err)
		}
		keyStr := strings.TrimSpace(string(keyData))
		if keyStr == "" {
			log.Fatalf("Critical: %s is empty", fileEnvKey)
		}
		return []byte(keyStr)
	}

	keyValue, exists := os.LookupEnv(envKey)
	if !exists || keyValue == "" {
		log.Fatalf("Critical: Neither %s nor %s is set", envKey, fileEnvKey)
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
