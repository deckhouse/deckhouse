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

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

type kubeClientProvider struct {
	kubeProvider libcon.KubeProvider
}

func newKubeClientProvider(kubeProvider libcon.KubeProvider) *kubeClientProvider {
	return &kubeClientProvider{
		kubeProvider: kubeProvider,
	}
}

func (p *kubeClientProvider) KubeClientCtx(ctx context.Context) (*client.KubernetesClient, error) {
	if p.kubeProvider == nil {
		return nil, fmt.Errorf("kube provider in nil")
	}
	kubeCl, err := p.kubeProvider.Client(ctx)
	if err != nil {
		return nil, err
	}
	return &client.KubernetesClient{KubeClient: kubeCl}, nil
}

func (p *kubeClientProvider) Cleanup(stopSSH bool) {
	p.kubeProvider.Cleanup(context.Background())
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
