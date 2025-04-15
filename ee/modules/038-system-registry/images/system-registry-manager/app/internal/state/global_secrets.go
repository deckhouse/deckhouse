/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	SecretsName = "registry-secrets"
)

type Secrets struct {
	HTTP string
}

func (gs *Secrets) Validate() error {
	if gs == nil {
		return ErrIsNil
	}

	if strings.TrimSpace(gs.HTTP) == "" {
		return fmt.Errorf("HttpSecret is empty")
	}

	return nil
}

func (gs *Secrets) DecodeSecret(secret *corev1.Secret) error {
	if secret == nil {
		return ErrSecretIsNil
	}

	gs.HTTP = string(secret.Data["http"])

	return nil
}
