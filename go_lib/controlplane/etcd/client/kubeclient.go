/*
Copyright 2026 Flant JSC

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

package client

import (
	"fmt"
	"path/filepath"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
)

// ToClientSet converts a KubeConfig object to a client
func ToClientSet(config *clientcmdapi.Config) (clientset.Interface, error) {
	overrides := clientcmd.ConfigOverrides{Timeout: "10s"}
	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create API client configuration from kubeconfig: %w", err)
	}

	client, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}
	return client, nil
}

// ClientSetFromFile returns a ready-to-use client from a kubeconfig file
func ClientSetFromFile(path string) (clientset.Interface, error) {
	config, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load admin kubeconfig: %w", err)
	}
	return ToClientSet(config)
}

// Client returns the Client for accessing the cluster with the identity defined in admin.conf.
func NewKubernetesClient() (clientset.Interface, error) {
	pathAdmin := filepath.Join(constants.KubernetesDir, constants.AdminKubeConfigFileName)

	client, err := ClientSetFromFile(pathAdmin)
	if err != nil {
		return nil, fmt.Errorf("couldn't create Kubernetes client: %w", err)
	}
	return client, nil
}
