/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"

	nodeservices "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/node-services"
)

const (
	NodeServicesConfigSecretNamePrefix = "registry-node-config-"

	nodeServicesConfigSecretType      = "registry/node-services-config"
	nodeServicesConfigSecretTypeLabel = "node-services-config"
)

func NodeServicesConfigSecretName(nodeName string) string {
	return fmt.Sprintf("%s%s", NodeServicesConfigSecretNamePrefix, nodeName)
}

type NodeServicesConfig struct {
	Version string
	Config  nodeservices.Config
}

func (nsc *NodeServicesConfig) Validate() error {
	return validation.ValidateStruct(nsc,
		validation.Field(&nsc.Config, validation.Required),
	)
}

func (nsc *NodeServicesConfig) EncodeSecret(secret *corev1.Secret) error {
	if secret == nil {
		return ErrSecretIsNil
	}

	secret.Type = nodeServicesConfigSecretType

	initSecretLabels(secret)
	secret.Labels[LabelTypeKey] = nodeServicesConfigSecretTypeLabel

	configBytes, err := yaml.Marshal(&nsc.Config)
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	secret.Data = map[string][]byte{
		"version": []byte(nsc.Version),
		"config":  configBytes,
	}

	return nil
}

func (nsc *NodeServicesConfig) DecodeSecret(secret *corev1.Secret) error {
	if secret == nil {
		return ErrSecretIsNil
	}

	nsc.Version = string(secret.Data["version"])

	if err := yaml.Unmarshal(secret.Data["config"], &nsc.Config); err != nil {
		return fmt.Errorf("config unmarshal error: %w", err)
	}

	return nil
}
