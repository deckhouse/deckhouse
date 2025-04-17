/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pki

import "fmt"

type User struct {
	Name         string `json:"name"`
	Password     string `json:"password"`
	PasswordHash string `json:"passwordHash"`
}

func NewUser(name string) (User, error) {
	password, err := GenerateUserPassword()
	if err != nil {
		return User{}, fmt.Errorf("failed to generate user password: %w", err)
	}

	passwordHash, err := GeneratePasswordHash(password)
	if err != nil {
		return User{}, fmt.Errorf("failed to generate password hash: %w", err)
	}

	return User{Name: name, Password: password, PasswordHash: passwordHash}, nil
}
