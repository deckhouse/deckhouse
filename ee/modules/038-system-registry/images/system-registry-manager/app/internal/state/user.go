/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"embeded-registry-manager/internal/utils/pki"
)

const (
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
	password, err := pki.GenerateUserPassword()
	if err != nil {
		return err
	}

	u.Password = password
	u.UpdatePasswordHash()

	return nil
}
