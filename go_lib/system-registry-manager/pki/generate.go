/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pki

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

const (
	secretCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	userPasswordLength = 16
	randomSecretLength = 25
)

func GenerateRandomSecret() (string, error) {
	return generateSecret(randomSecretLength)
}

func GenerateUserPassword() (string, error) {
	return generateSecret(userPasswordLength)
}

func GeneratePasswordHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func generateSecret(size int) (string, error) {
	secret := make([]byte, size)
	charsetLength := big.NewInt(int64(len(secretCharset)))

	for i := range secret {
		index, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			return "", fmt.Errorf("random error: %w", err)
		}
		secret[i] = secretCharset[index.Int64()]
	}

	return string(secret), nil
}
