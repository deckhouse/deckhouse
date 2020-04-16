package kube

import (
	"fmt"

	sh_app "github.com/flant/shell-operator/pkg/app"
	sh_kube "github.com/flant/shell-operator/pkg/kube"

	"flant/deckhouse-cluster/pkg/app"
	"flant/deckhouse-cluster/pkg/ssh"
)

// KubernetesClient is a wrapper around KubernetesClient from shell-operator which is a wrapper around kubernetes.Interface
// KubernetesClient adds ability to connect to API server through ssh tunnel and kubectl proxy.

type KubernetesClient struct {
	sh_kube.KubernetesClient
	SshClient *ssh.SshClient
	KubeProxy *ssh.KubeProxy
}

func NewKubernetesClient() *KubernetesClient {
	return &KubernetesClient{}
}

func NewFakeKubernetesClient() *KubernetesClient {
	return &KubernetesClient{KubernetesClient: sh_kube.NewFakeKubernetesClient()}
}

// InitKubernetesClient initializes kubernetes client from KUBECONFIG or from ssh tunnel
func (k *KubernetesClient) Init(configSrc string) error {
	startProxy := false

	switch configSrc {
	case "SSH":
		if app.SshHost != "" {
			return fmt.Errorf("no ssh-host to connect to kubernetes via ssh tunnel")
		}
		startProxy = true
	case "KUBECONFIG":
	default:
		// auto detect
		if app.SshHost != "" {
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
		return fmt.Errorf("initialize kube client: %s\n", err)
	}
	k.KubernetesClient = kubeClient
	return nil
}

func (k *KubernetesClient) Stop() {
	if k.KubeProxy != nil {
		k.KubeProxy.Stop()
	}
	if k.SshClient != nil {
		k.SshClient.StopSshAgent()
	}
}

func (k *KubernetesClient) StartKubernetesProxy() (string, error) {
	success := false
	defer func() {
		if !success {
			k.Stop()
		}
	}()

	privateKeys, err := ssh.ParseSshPrivateKeyPaths(app.SshAgentPrivateKeys)
	if err != nil {
		return "", fmt.Errorf("ssh private keys: %v", err)
	}
	k.SshClient = &ssh.SshClient{
		BastionHost: app.SshBastionHost,
		BastionUser: app.SshBastionUser,
		PrivateKeys: privateKeys,
		ExtraArgs:   app.SshExtraArgs,
	}

	app.Debugf("ssh client config: %+v\n", k.SshClient)

	err = k.SshClient.StartSshAgent()
	if err != nil {
		return "", fmt.Errorf("start ssh-agent: %v", err)
	}

	err = k.SshClient.AddKeys()
	if err != nil {
		return "", fmt.Errorf("add keys: %v", err)
	}

	k.SshClient.Host = app.SshHost
	k.SshClient.User = app.SshUser

	k.KubeProxy = k.SshClient.KubeProxy()
	port, err := k.KubeProxy.Start()
	if err != nil {
		return "", fmt.Errorf("start kube proxy: %v", err)
	}

	success = true
	fmt.Printf("Proxy started on port %s\n", port)

	return port, nil
}
