/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package client

// import (
// 	"k8s.io/client-go/kubernetes"
// 	"k8s.io/client-go/tools/clientcmd"
// 	"os"
// 	"path/filepath"
// )

// func NewK8sClient() (*kubernetes.Clientset, error) {
// 	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")

// 	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
// 	if err != nil {
// 		return nil, err
// 	}

// 	clientset, err := kubernetes.NewForConfig(config)
// 	return clientset, err
// }
