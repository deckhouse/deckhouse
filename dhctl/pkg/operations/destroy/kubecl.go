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
	"strings"

	libcon "github.com/deckhouse/lib-connection/pkg"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

type kubeClientProvider struct {
	sshProvider  libcon.SSHProvider
	kubeProvider libcon.KubeProvider
}

func newKubeClientProvider(kubeProvider libcon.KubeProvider, sshProvider libcon.SSHProvider) *kubeClientProvider {
	return &kubeClientProvider{
		kubeProvider: kubeProvider,
		sshProvider:  sshProvider,
	}
}

func (p *kubeClientProvider) KubeClientCtx(ctx context.Context) (*client.KubernetesClient, error) {
	if p.kubeProvider == nil {
		return nil, fmt.Errorf("kube provider is nil")
	}
	kubeCl, err := p.kubeProvider.Client(ctx)
	if err != nil {
		return nil, err
	}

	// The connection the API is reached over is carried along, because destroying a cluster that keeps
	// its own registry means pulling the infrastructure tooling out of that registry — and the only
	// address the cluster knows for it is one that resolves nowhere but inside. A local forward on this
	// same connection is what makes it reachable; see registrydata.mirrorThroughNode.
	//
	// Absent when destroying without SSH (a kubeconfig against a static cluster), which changes nothing:
	// the tunnel path is skipped and the caller behaves as it did.
	return (&client.KubernetesClient{KubeClient: kubeCl}).WithSSHClient(p.sshClient(ctx)), nil
}

// sshClient returns the connection to the cluster, or nil when this destroy has none.
func (p *kubeClientProvider) sshClient(ctx context.Context) libcon.SSHClient {
	if p.sshProvider == nil {
		return nil
	}

	sshCl, err := p.sshProvider.Client(ctx)
	if err != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf(
			"no ssh connection to reach the cluster store through: %v", err))
		return nil
	}

	return sshCl
}

func (p *kubeClientProvider) Cleanup(ctx context.Context, stopSSH bool) {
	err := p.kubeProvider.Cleanup(ctx)
	if err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, strings.TrimRight(fmt.Sprintf("failed to clean up kube provider: %v", err), "\n"))
	}

	if stopSSH {
		err := p.sshProvider.Cleanup(ctx)
		if err != nil {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("failed to clean up ssh provider: %v", err))
		}
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
func (p *kubeClientErrorProvider) Cleanup(context.Context, bool) {}
