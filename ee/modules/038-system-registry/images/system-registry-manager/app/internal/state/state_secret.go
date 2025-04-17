/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import corev1 "k8s.io/api/core/v1"

const (
	StateSecretName = "registry-state-tmp"
)

type StateSecret struct {
	Version string
}

func (cfg *StateSecret) InitWithDefaults() {
	cfg.Version = "default"
}

func (cfg *StateSecret) Validate() error {
	if cfg == nil {
		return ErrIsNil
	}

	return nil
}

func (cfg *StateSecret) DecodeSecret(secret *corev1.Secret) error {
	if secret == nil {
		return ErrSecretIsNil
	}

	cfg.Version = string(secret.Data["staticpod_version"])

	return nil
}
