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

package cloud

import (
	"context"
	"fmt"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
)

type ClusterInfraDestroyer interface {
	DestroyCluster(ctx context.Context, autoApprove bool) error
}

type DestroyerParams struct {
	LoggerProvider log.LoggerProvider
	KubeProvider   kube.ClientProviderWithCleanup
	State          *State

	StateLoader  controller.StateLoader
	ClusterInfra ClusterInfraDestroyer

	CommanderMode bool
	SkipResources bool
}

type Destroyer struct {
	params *DestroyerParams

	convergeUnlocker func(fullUnlock bool)
}

func NewDestroyer(params *DestroyerParams) *Destroyer {
	return &Destroyer{params: params}
}

func (d *Destroyer) Prepare(ctx context.Context) error {
	logger := log.SafeProvideLogger(d.params.LoggerProvider)

	if d.params.CommanderMode {
		logger.LogDebugLn("Locking converge skipped for commander")
		return nil
	}

	if d.params.SkipResources {
		logger.LogDebugLn("Locking converge skipped because resources should skip")
		return nil
	}

	locked, err := d.params.State.IsConvergeLocked()
	if err != nil {
		return err
	}

	if locked {
		logger.LogDebugLn("Locking converge skipped because locked in previous run")
		return nil
	}

	if err := d.lockConverge(ctx); err != nil {
		return err
	}

	if err := d.params.State.SetConvergeLocked(); err != nil {
		// try to unlock because we cannot save in state
		d.unlockConverge(true)
		return err
	}

	logger.LogDebugLn("Converge was locked successfully and write to state")

	return nil
}

func (d *Destroyer) AfterResourcesDelete(ctx context.Context) error {
	_, err := d.params.StateLoader.PopulateMetaConfig(ctx)
	if err != nil {
		return err
	}
	_, _, err = d.params.StateLoader.PopulateClusterState(ctx)
	return err
}

func (d *Destroyer) CleanupBeforeDestroy(context.Context) error {
	// why only unwatch lock without request unlock
	// user may not delete resources and converge still working in cluster
	// all node groups removing may still in long time run and
	// we get race (destroyer destroy node group, auto applayer create nodes)
	d.unlockConverge(false)

	// stop ssh because master nodes will delete and we lost connection
	d.params.KubeProvider.Cleanup(true)

	return nil
}

func (d *Destroyer) DestroyCluster(ctx context.Context, autoApprove bool) error {
	if govalue.IsNil(d.params.ClusterInfra) {
		return fmt.Errorf("Internal error. Cluster infra destroy is nil")
	}

	return d.params.ClusterInfra.DestroyCluster(ctx, autoApprove)
}

func (d *Destroyer) unlockConverge(fullUnlock bool) {
	if d.convergeUnlocker != nil {
		d.convergeUnlocker(fullUnlock)
		d.convergeUnlocker = nil
	}
}

func (d *Destroyer) lockConverge(ctx context.Context) error {
	kubeCl, err := d.params.KubeProvider.KubeClientCtx(ctx)
	if err != nil {
		return err
	}

	// todo refactor lock converge with ctx
	unlockConverge, err := lock.LockConverge(ctx, kubernetes.NewSimpleKubeClientGetter(kubeCl), "local-destroyer")
	if err != nil {
		return err
	}
	d.convergeUnlocker = unlockConverge

	return nil
}
