package client

import (
	"fmt"

	sh_app "github.com/flant/shell-operator/pkg/app"
	sh_kube "github.com/flant/shell-operator/pkg/kube"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/system/ssh"
	"flant/candictl/pkg/system/ssh/frontend"
)

// KubernetesClient is a wrapper around KubernetesClient from shell-operator which is a wrapper around kubernetes.Interface
// KubernetesClient adds ability to connect to API server through ssh tunnel and kubectl proxy.

type KubernetesClient struct {
	sh_kube.KubernetesClient
	SSHClient *ssh.SSHClient
	KubeProxy *frontend.KubeProxy
}

func NewKubernetesClient() *KubernetesClient {
	return &KubernetesClient{}
}

func NewFakeKubernetesClient() *KubernetesClient {
	return &KubernetesClient{KubernetesClient: sh_kube.NewFakeKubernetesClient()}
}

func (k *KubernetesClient) WithSSHClient(client *ssh.SSHClient) *KubernetesClient {
	k.SSHClient = client
	return k
}

// InitKubernetesClient initializes kubernetes client from KUBECONFIG or from ssh tunnel
func (k *KubernetesClient) Init(configSrc string) error {
	startProxy := false

	switch configSrc {
	case "SSH":
		if app.SSHHost == "" {
			return fmt.Errorf("no ssh-host to connect to kubernetes via ssh tunnel")
		}
		startProxy = true
	case "KUBECONFIG":
	default:
		// auto detect
		if app.SSHHost != "" {
			startProxy = true
		}
	}

	kubeClient := sh_kube.NewKubernetesClient()
	kubeClient.WithRateLimiterSettings(sh_app.KubeClientQps, sh_app.KubeClientBurst)

	if startProxy {
		port, err := k.StartKubernetesProxy()
		if err != nil {
			return err
		}
		kubeClient.WithServer("http://localhost:" + port)
	} else {
		kubeClient.WithContextName(sh_app.KubeContext)
		kubeClient.WithConfigPath(sh_app.KubeConfig)
		kubeClient.WithServer(sh_app.KubeServer)
	}

	// Initialize kube client for kube events hooks.
	err := kubeClient.Init()
	if err != nil {
		return fmt.Errorf("initialize kube client: %s", err)
	}

	k.KubernetesClient = kubeClient
	return nil
}

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
