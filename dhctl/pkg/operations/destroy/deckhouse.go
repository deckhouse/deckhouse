// Copyright 2021 Flant JSC
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

	"github.com/google/uuid"
	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

type DeckhouseDestroyerOptions struct {
	CommanderMode bool
	CommanderUUID uuid.UUID
}

type DeckhouseDestroyer struct {
	convergeUnlocker  func(fullUnlock bool)
	sshClientProvider SSHProvider
	sshClient         node.SSHClient
	kubeCl            *client.KubernetesClient
	state             *State

	DeckhouseDestroyerOptions
}

func NewDeckhouseDestroyer(sshClientProvider SSHProvider, state *State, opts DeckhouseDestroyerOptions) *DeckhouseDestroyer {
	return &DeckhouseDestroyer{
		sshClientProvider:         sshClientProvider,
		state:                     state,
		DeckhouseDestroyerOptions: opts,
	}
}

func (g *DeckhouseDestroyer) UnlockConverge(fullUnlock bool) {
	if g.convergeUnlocker != nil {
		g.convergeUnlocker(fullUnlock)
		g.convergeUnlocker = nil
	}
}

func (g *DeckhouseDestroyer) GetKubeClient(ctx context.Context) (*client.KubernetesClient, error) {
	if g.kubeCl != nil {
		return g.kubeCl, nil
	}

	sshClient, err := g.sshClientProvider()
	if err != nil {
		return nil, err
	}

	g.sshClient = sshClient

	kubeCl, err := kubernetes.ConnectToKubernetesAPI(ctx, ssh.NewNodeInterfaceWrapper(sshClient))
	if err != nil {
		return nil, err
	}
	g.kubeCl = kubeCl

	return kubeCl, err
}

func (g *DeckhouseDestroyer) LockConverge(ctx context.Context) error {
	kubeCl, err := g.GetKubeClient(ctx)
	if err != nil {
		return err
	}

	unlockConverge, err := lock.LockConverge(ctx, kubernetes.NewSimpleKubeClientGetter(kubeCl), "local-destroyer")
	if err != nil {
		return err
	}
	g.convergeUnlocker = unlockConverge

	return nil
}

func (g *DeckhouseDestroyer) KubeClient() *client.KubernetesClient {
	kubeClient, _ := g.GetKubeClient(context.Background())
	return kubeClient
}

func (g *DeckhouseDestroyer) DeleteResources(ctx context.Context, cloudType string) error {
	resourcesDestroyed, err := g.state.IsResourcesDestroyed()
	if err != nil {
		return err
	}

	if resourcesDestroyed {
		log.WarnLn("Resources was destroyed. Skip it")
		return nil
	}

	kubeCl, err := g.GetKubeClient(ctx)
	if err != nil {
		return err
	}

	return log.Process("common", "Delete resources from the Kubernetes cluster", func() error {
		return g.deleteEntities(ctx, kubeCl)
	})
}

func (g *DeckhouseDestroyer) deleteEntities(ctx context.Context, kubeCl *client.KubernetesClient) error {
	err := deckhouse.DeleteDeckhouseDeployment(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForDeckhouseDeploymentDeletion(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeletePDBs(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteServices(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForServicesDeletion(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteAllD8StorageResources(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteStorageClasses(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeletePVC(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeletePods(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForPVCDeletion(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForPVDeletion(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteMachinesIfResourcesExist(ctx, kubeCl)
	if err != nil {
		return err
	}

	return nil
}

func (g *DeckhouseDestroyer) Cleanup(stopSSH bool) {
	// why only unwatch lock without request unlock
	// user may not delete resources and converge still working in cluster
	// all node groups removing may still in long time run and
	// we get race (destroyer destroy node group, auto applayer create nodes)
	g.UnlockConverge(false)

	if !govalue.IsNil(g.kubeCl) {
		g.kubeCl.KubeProxy.StopAll()
		g.kubeCl = nil
	}

	if !stopSSH || govalue.IsNil(g.sshClient) {
		return
	}

	g.sshClient.Stop()
	g.sshClient = nil
}
