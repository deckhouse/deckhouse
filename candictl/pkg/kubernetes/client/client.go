package client

import (
	"fmt"

	sh_kube "github.com/flant/shell-operator/pkg/kube"
	// oidc allows using oidc provider in kubeconfig
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/system/ssh"
	"flant/candictl/pkg/system/ssh/frontend"
)

// KubernetesClient is a wrapper around KubernetesClient from shell-operator which is a wrapper around kubernetes.Interface
// KubernetesClient adds ability to connect to API server through ssh tunnel and kubectl proxy.
type KubernetesClient struct {
	sh_kube.KubernetesClient
	SSHClient *ssh.Client
	KubeProxy *frontend.KubeProxy
}

func NewKubernetesClient() *KubernetesClient {
	return &KubernetesClient{}
}

func NewFakeKubernetesClient() *KubernetesClient {
	return &KubernetesClient{KubernetesClient: sh_kube.NewFakeKubernetesClient()}
}

func (k *KubernetesClient) WithSSHClient(client *ssh.Client) *KubernetesClient {
	k.SSHClient = client
	return k
}

// Init initializes kubernetes client
func (k *KubernetesClient) Init() error {
	kubeClient := sh_kube.NewKubernetesClient()
	kubeClient.WithRateLimiterSettings(5, 10)

	switch {
	case app.KubeConfigInCluster:
	case app.KubeConfig != "":
		kubeClient.WithContextName(app.KubeConfigContext)
		kubeClient.WithConfigPath(app.KubeConfig)
	default:
		port, err := k.StartKubernetesProxy()
		if err != nil {
			return err
		}
		kubeClient.WithServer("http://localhost:" + port)
	}

	// Initialize kube client for kube events hooks.
	err := kubeClient.Init()
	if err != nil {
		return fmt.Errorf("initialize kube client: %s", err)
	}

	k.KubernetesClient = kubeClient
	return nil
}

// StartKubernetesProxy initializes kubectl-proxy on remote host and establishes ssh tunnel to it
func (k *KubernetesClient) StartKubernetesProxy() (port string, err error) {
	if k.SSHClient == nil {
		k.SSHClient, err = ssh.NewClientFromFlags().Start()
		if err != nil {
			return "", err
		}
	}

	k.KubeProxy = k.SSHClient.KubeProxy()
	port, err = k.KubeProxy.Start()
	if err != nil {
		return "", fmt.Errorf("start kube proxy: %v", err)
	}

	log.InfoF("Proxy started on port %s\n", port)
	return port, nil
}
