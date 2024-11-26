/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	userPasswordLength  = 16
	userPasswordCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	UserROSecretName = "registry-user-ro"
	UserRWSecretName = "registry-user-rw"

	UserSecretType      = "system-registry/user"
	UserSecretTypeLabel = "system-registry-user"
)

type User struct {
	UserName       string
	Password       string
	HashedPassword string
}

func (u *User) IsEqual(other *User) bool {
	if u == nil || other == nil {
		return u == other
	}
	if u.UserName != other.UserName {
		return false
	}
	if u.Password != other.Password {
		return false
	}
	if u.HashedPassword != other.HashedPassword {
		return false
	}
	return true
}

func (u *User) IsValid() bool {
	if u == nil {
		return false
	}

	if strings.TrimSpace(u.UserName) == "" {
		return false
	}

	if u.Password == "" {
		return false
	}

	return true
}

func (u *User) IsPasswordHashValid() bool {
	err := bcrypt.CompareHashAndPassword(
		[]byte(u.HashedPassword),
		[]byte(u.Password),
	)
	return err == nil
}

func (u *User) UpdatePasswordHash() error {
	if u.Password == "" {
		u.HashedPassword = ""
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("bcryp error: %w", err)
	}

	u.HashedPassword = string(hash)

	return nil
}

func (u *User) GenerateNewPassword() error {
	password := make([]byte, userPasswordLength)
	charsetLength := big.NewInt(int64(len(userPasswordCharset)))

	for i := range password {
		index, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			return fmt.Errorf("random error: %w", err)
		}
		password[i] = userPasswordCharset[index.Int64()]
	}

	u.Password = string(password)

	u.UpdatePasswordHash()

	return nil
}
