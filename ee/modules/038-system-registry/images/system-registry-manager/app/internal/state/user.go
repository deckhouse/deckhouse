/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

const (
	UserROSecretName = "registry-user-ro"
	UserRWSecretName = "registry-user-rw"

	UserMirrorPullerName = "registry-user-mirror-puller"
	UserMirrorPusherName = "registry-user-mirror-pusher"

	userSecretType      = "system-registry/user"
	userSecretTypeLabel = "system-registry-user"
)

var _ encodeDecodeSecret = &User{}

type User struct {
	UserName       string
	Password       string
	HashedPassword string
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

func (u *User) DecodeSecret(secret *corev1.Secret) error {
	if secret == nil {
		return ErrSecretIsNil
	}

	u.UserName = string(secret.Data["name"])
	u.Password = string(secret.Data["password"])
	u.HashedPassword = string(secret.Data["passwordHash"])

	return nil
}

func (u *User) EncodeSecret(secret *corev1.Secret) error {
	if secret == nil {
		return ErrSecretIsNil
	}

	secret.Type = userSecretType

	initSecretLabels(secret)
	secret.Labels[LabelTypeKey] = userSecretTypeLabel

	secret.Data = map[string][]byte{
		"name":         []byte(u.UserName),
		"password":     []byte(u.Password),
		"passwordHash": []byte(u.HashedPassword),
	}

	return nil
}
