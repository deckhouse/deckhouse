/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
