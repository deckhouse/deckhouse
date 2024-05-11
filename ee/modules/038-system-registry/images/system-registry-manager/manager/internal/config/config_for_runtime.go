/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	// "k8s.io/client-go/tools/clientcmd"
	// "os"
	// "path/filepath"
)

type RuntimeConfig struct {
	K8sClient      *kubernetes.Clientset
	ShouldUpdateBy ShouldUpdateBy
}

type ShouldUpdateBy struct {
	NeedChangeFileByExist          bool
	NeedChangeFileByCheckSum       bool
	NeedChangeSeaweedfsCerts       bool
	NeedChangeDockerAuthTokenCerts bool
}

func NewRuntimeConfig() (*RuntimeConfig, error) {
	k8sClient, err := NewK8sClient()
	if err != nil {
		return nil, err
	}

	config := RuntimeConfig{
		K8sClient:      k8sClient,
		ShouldUpdateBy: ShouldUpdateBy{},
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

// func NewK8sClient() (*kubernetes.Clientset, error) {
// 	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")

// 	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
// 	if err != nil {
// 		return nil, err
// 	}

// 	clientset, err := kubernetes.NewForConfig(config)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return clientset, nil
// }
