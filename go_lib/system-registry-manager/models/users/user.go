/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package users

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

type User struct {
	UserName       string `json:"name"`
	Password       string `json:"password"`
	HashedPassword string `json:"password_hash"`
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
	if err := u.UpdatePasswordHash(); err != nil {
		return fmt.Errorf("cannot update password hash: %w", err)
	}

	return nil
}

func (u *User) DecodeSecretData(data map[string][]byte) error {
	u.UserName = string(data["name"])
	u.Password = string(data["password"])
	u.HashedPassword = string(data["passwordHash"])

	return nil
}
