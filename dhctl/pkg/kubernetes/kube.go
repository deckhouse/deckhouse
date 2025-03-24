// Copyright 2023 Flant JSC
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

package kubernetes

import (
	"context"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func ConnectToKubernetesAPI(ctx context.Context, nodeInterface node.Interface) (*client.KubernetesClient, error) {
	var kubeCl *client.KubernetesClient
	err := log.Process("common", "Connect to Kubernetes API", func() error {
		if wrapper, ok := nodeInterface.(*ssh.NodeInterfaceWrapper); ok && wrapper != nil {
			if err := wrapper.Client().Check().WithDelaySeconds(1).AwaitAvailability(ctx); err != nil {
				return fmt.Errorf("await master available: %v", err)
			}
		}

		err := retry.NewLoop("Get Kubernetes API client", 45, 5*time.Second).
			RunContext(ctx, func() error {
				kubeCl = client.NewKubernetesClient().WithNodeInterface(nodeInterface)
				if err := kubeCl.InitContext(ctx, client.AppKubernetesInitParams()); err != nil {
					return fmt.Errorf("open kubernetes connection: %v", err)
				}
				return nil
			})
		if err != nil {
			return err
		}

		time.Sleep(50 * time.Millisecond) // tick to prevent first probable fail
		err = deckhouse.WaitForKubernetesAPI(ctx, kubeCl)
		if err != nil {
			return fmt.Errorf("wait kubernetes api: %v", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("start kubernetes proxy: %v", err)
	}

	return kubeCl, nil
}

type KubeClientProvider interface {
	KubeClient() *client.KubernetesClient
}

var _ KubeClientProvider = &SimpleKubeClientGetter{}

type SimpleKubeClientGetter struct {
	kubeCl *client.KubernetesClient
}

func NewSimpleKubeClientGetter(kubeCl *client.KubernetesClient) *SimpleKubeClientGetter {
	return &SimpleKubeClientGetter{kubeCl: kubeCl}
}

func (s *SimpleKubeClientGetter) KubeClient() *client.KubernetesClient {
	return s.kubeCl
}
