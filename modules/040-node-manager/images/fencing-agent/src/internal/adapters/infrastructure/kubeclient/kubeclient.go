package kubeclient

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func GetCurrentNodeIP(ctx context.Context, kubeClient kubernetes.Interface, nodeName string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, v1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get node=%s InternalIp for memberlist: %w", nodeName, err)
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {

			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("node %s has no InternalIP address", nodeName)
}

func GetClientset(kubeconfigPath string, timeout time.Duration, qps float32, burst int) (*kubernetes.Clientset, error) {
	var restConfig *rest.Config
	var kubeClient *kubernetes.Clientset
	var err error

	restConfig, err = buildConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	restConfig.QPS = qps
	restConfig.Burst = burst
	restConfig.Timeout = timeout

	kubeClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}

// Reimplementation of clientcmd.buildConfig to avoid default warn message
func buildConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		kubeconfig, err := rest.InClusterConfig()
		if err == nil {
			return kubeconfig, nil
		}
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}}).ClientConfig()
}
