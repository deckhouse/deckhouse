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
	GlobalSecretsName = "registry-secrets"
)

type GlobalSecrets struct {
	HTTPSecret string
}

func (gs *GlobalSecrets) Validate() error {
	if gs == nil {
		return ErrIsNil
	}

	if strings.TrimSpace(gs.HTTPSecret) == "" {
		return fmt.Errorf("HttpSecret is empty")
	}

	return nil
}

func (gs *GlobalSecrets) DecodeSecret(secret *corev1.Secret) error {
	if secret == nil {
		return ErrSecretIsNil
	}

	gs.HTTPSecret = string(secret.Data["http"])

	return nil
}
