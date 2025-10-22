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

package users

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

type User struct {
	UserName       string `json:"name"`
	Password       string `json:"password"`
	HashedPassword string `json:"password_hash"`
}

func New(name string) (User, error) {
	ret := User{UserName: name}
	err := ret.GenerateNewPassword()
	if err != nil {
		return ret, fmt.Errorf("failed to generate user password: %w", err)
	}
	return ret, nil
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
