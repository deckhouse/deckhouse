package config

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type SystemRegistryManagerConfig struct {
	K8sClient *kubernetes.Clientset
}

func NewSystemRegistryManagerConfig() (*SystemRegistryManagerConfig, error) {
	k8sClient, err := NewK8sClient()
	if err != nil {
		return nil, err
	}
	config := SystemRegistryManagerConfig{
		K8sClient: k8sClient,
	}
	return &config, nil
}

func NewK8sClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	k8sClient, err := kubernetes.NewForConfig(config)
	return k8sClient, nil
}
