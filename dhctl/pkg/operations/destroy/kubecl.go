package destroy

import (
	"context"
	"fmt"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
)

type kubeClientProvider struct {
	sshClientProvider sshclient.SSHProvider

	sshClient node.SSHClient
	kubeCl    *client.KubernetesClient
}

func newKubeClientProvider(sshClientProvider sshclient.SSHProvider) *kubeClientProvider {
	return &kubeClientProvider{
		sshClientProvider: sshClientProvider,
	}
}

func (p *kubeClientProvider) KubeClientCtx(ctx context.Context) (*client.KubernetesClient, error) {
	if !govalue.IsNil(p.kubeCl) {
		return p.kubeCl, nil
	}

	if govalue.IsNil(p.sshClientProvider) {
		return nil, fmt.Errorf("sshClientProvider did not pass")
	}

	sshClient, err := p.sshClientProvider()
	if err != nil {
		return nil, err
	}

	p.sshClient = sshClient

	kubeCl, err := kubernetes.ConnectToKubernetesAPI(ctx, ssh.NewNodeInterfaceWrapper(sshClient))
	if err != nil {
		return nil, err
	}
	p.kubeCl = kubeCl

	return kubeCl, err
}

func (p *kubeClientProvider) Cleanup(stopSSH bool) {
	if !govalue.IsNil(p.kubeCl) {
		p.kubeCl.KubeProxy.StopAll()
		p.kubeCl = nil
	}

	if stopSSH && !govalue.IsNil(p.sshClient) {
		p.sshClient.Stop()
		p.sshClient = nil
	}
}

type kubeClientErrorProvider struct {
	msg string
}

func newKubeClientErrorProvider(msg string) *kubeClientErrorProvider {
	return &kubeClientErrorProvider{
		msg: msg,
	}
}
func (p *kubeClientErrorProvider) KubeClientCtx(context.Context) (*client.KubernetesClient, error) {
	return nil, fmt.Errorf("Unable to get kube client: '%s'", p.msg)
}
func (p *kubeClientErrorProvider) Cleanup(bool) {}
