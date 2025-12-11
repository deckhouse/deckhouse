// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
