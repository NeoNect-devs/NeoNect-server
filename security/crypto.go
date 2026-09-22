package security

import (
	"crypto/sha256"
	"encoding/hex"
	"golang.org/x/crypto/bcrypt"
)

var BcryptCost = bcrypt.DefaultCost

func ComputeHash(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

func HashPassword(plainTextPassword string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(plainTextPassword), BcryptCost)
	return string(hashedBytes), err
}

func CheckPasswordHash(plainTextPassword, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainTextPassword))
	return err == nil
}
