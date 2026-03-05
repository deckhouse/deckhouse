package kubeadmapp

import (
	"fmt"
	"path/filepath"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/constants"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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
func MyNewKubernetesClient() (clientset.Interface, error) {
	pathAdmin := filepath.Join(constants.KubernetesDir, constants.AdminKubeConfigFileName)

	client, err := ClientSetFromFile(pathAdmin)
	if err != nil {
		return nil, fmt.Errorf("[preflight] couldn't create Kubernetes client: %w", err)
	}
	return client, nil
}
