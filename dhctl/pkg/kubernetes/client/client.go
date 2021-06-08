package client

import (
	"fmt"
	"time"

	klient "github.com/flant/kube-client/client"

	// oidc allows using oidc provider in kubeconfig
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type KubeClient = klient.Client

// KubernetesClient connects to kubernetes API server through ssh tunnel and kubectl proxy.
type KubernetesClient struct {
	KubeClient
	SSHClient *ssh.Client
	KubeProxy *frontend.KubeProxy
}

func NewKubernetesClient() *KubernetesClient {
	return &KubernetesClient{}
}

func NewFakeKubernetesClient() *KubernetesClient {
	return &KubernetesClient{KubeClient: klient.NewFake(nil)}
}

func (k *KubernetesClient) WithSSHClient(client *ssh.Client) *KubernetesClient {
	k.SSHClient = client
	return k
}

// Init initializes kubernetes client
func (k *KubernetesClient) Init() error {
	kubeClient := klient.New()
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

	k.KubeClient = kubeClient
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

	err = retry.NewLoop("Starting kube proxy", k.SSHClient.Settings.CountHosts(), 1*time.Second).Run(func() error {
		log.InfoF("Using host %s\n", k.SSHClient.Settings.Host())

		k.KubeProxy = k.SSHClient.KubeProxy()
		port, err = k.KubeProxy.Start(-1)

		if err != nil {
			k.SSHClient.Settings.ChoiceNewHost()
			return fmt.Errorf("start kube proxy: %v", err)
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	log.InfoF("Proxy started on port %s\n", port)
	return port, nil
}
