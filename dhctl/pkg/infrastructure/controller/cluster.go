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

package controller

import (
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type StateLoader interface {
	PopulateMetaConfig(ctx context.Context) (*config.MetaConfig, error)
	PopulateClusterState(ctx context.Context) ([]byte, map[string]state.NodeGroupInfrastructureState, error)
}

type NodeGroupController interface {
	DestroyNode(name string, nodeState []byte, sanityCheck bool) error
}

type BaseInfraControllerInterface interface {
	Destroy(clusterState []byte, sanityCheck bool) error
}

type ClusterInfra struct {
	stateLoader           StateLoader
	cache                 state.Cache
	infrastructureContext *infrastructure.Context

	PhasedExecutionContext phases.DefaultPhasedExecutionContext
}

func NewClusterInfra(terraState StateLoader, cache state.Cache, infrastructureContext *infrastructure.Context) *ClusterInfra {
	return NewClusterInfraWithOptions(terraState, cache, infrastructureContext, ClusterInfraOptions{})
}

type ClusterInfraOptions struct {
	PhasedExecutionContext phases.DefaultPhasedExecutionContext
}

func NewClusterInfraWithOptions(terraState StateLoader, cache state.Cache, infrastructureContext *infrastructure.Context, opts ClusterInfraOptions) *ClusterInfra {
	return &ClusterInfra{
		stateLoader:           terraState,
		cache:                 cache,
		infrastructureContext: infrastructureContext,

		PhasedExecutionContext: opts.PhasedExecutionContext,
	}
}

func (r *ClusterInfra) DestroyCluster(ctx context.Context, autoApprove bool) error {
	metaConfig, err := r.stateLoader.PopulateMetaConfig(ctx)
	if err != nil {
		return err
	}

	if r.infrastructureContext == nil {
		r.infrastructureContext = infrastructure.NewContextWithProvider(infrastructureprovider.ExecutorProvider(metaConfig))
	}

	clusterState, nodesState, err := r.stateLoader.PopulateClusterState(ctx)
	if err != nil {
		return err
	}

	if r.PhasedExecutionContext != nil {
		if shouldStop, err := r.PhasedExecutionContext.StartPhase(phases.AllNodesPhase, true, r.cache); err != nil {
			return err
		} else if shouldStop {
			return nil
		}
	}

	for nodeGroupName, nodeGroupStates := range nodesState {
		ngController, err := NewNodesController(metaConfig, r.cache, nodeGroupName, nodeGroupStates.Settings, r.infrastructureContext)
		if err != nil {
			return err
		}
		for name, ngState := range nodeGroupStates.State {
			err := ngController.DestroyNode(ctx, name, ngState, autoApprove)
			if err != nil {
				return err
			}
		}
	}

	if r.PhasedExecutionContext != nil {
		if shouldStop, err := r.PhasedExecutionContext.SwitchPhase(phases.BaseInfraPhase, true, r.cache, nil); err != nil {
			return err
		} else if shouldStop {
			return nil
		}
	}

	if err := NewBaseInfraController(metaConfig, r.cache, r.infrastructureContext).
		Destroy(ctx, clusterState, autoApprove); err != nil {
		return err
	}

	if r.PhasedExecutionContext != nil {
		return r.PhasedExecutionContext.CompletePhase(r.cache, nil)
	} else {
		return nil
	}
}
