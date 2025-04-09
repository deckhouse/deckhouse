/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/users"
)

const (
	UserROSecretName = "registry-user-ro"
	UserRWSecretName = "registry-user-rw"

	UserMirrorPullerName = "registry-user-mirror-puller"
	UserMirrorPusherName = "registry-user-mirror-pusher"

	userSecretType      = "system-registry/user"
	userSecretTypeLabel = "system-registry-user"
)

type User struct {
	users.User
}

func (u *User) DecodeSecret(secret *corev1.Secret) error {
	if secret == nil {
		return ErrSecretIsNil
	}

	return u.DecodeSecretData(secret.Data)
}
