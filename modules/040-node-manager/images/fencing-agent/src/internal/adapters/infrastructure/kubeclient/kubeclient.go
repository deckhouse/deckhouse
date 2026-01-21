package kubeclient

import (
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func NewClient(kubeconfigPath string, timeout time.Duration, qps float32, burst int) (*kubernetes.Clientset, error) {
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
