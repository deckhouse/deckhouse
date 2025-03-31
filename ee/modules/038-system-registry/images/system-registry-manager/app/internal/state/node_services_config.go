/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"

	"embeded-registry-manager/internal/staticpod"
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
	staticpod.NodeServicesConfigModel
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
