/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	"k8s.io/client-go/kubernetes"
	kube_client "system-registry-manager/pkg/kubernetes/client"
)

type RuntimeConfig struct {
	K8sClient *kubernetes.Clientset
}

func NewRuntimeConfig() (*RuntimeConfig, error) {
	k8sClient, err := kube_client.NewK8sClient()
	if err != nil {
		return nil, err
	}

	config := RuntimeConfig{
		K8sClient: k8sClient,
	}
	return &config, nil
}
