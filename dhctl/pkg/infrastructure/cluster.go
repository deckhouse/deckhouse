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

package infrastructure

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type StateLoader interface {
	PopulateMetaConfig() (*config.MetaConfig, error)
	PopulateClusterState() ([]byte, map[string]converge.NodeGroupTerraformState, error)
}

type NodeGroupController interface {
	DestroyNode(name string, nodeState []byte, sanityCheck bool) error
}

type BaseInfraController interface {
	Destroy(clusterState []byte, sanityCheck bool) error
}

type ClusterInfra struct {
	stateLoader StateLoader
	cache       state.Cache

	*phases.PhasedExecutionContext
}

func NewClusterInfra(terraState StateLoader, cache state.Cache) *ClusterInfra {
	return NewClusterInfraWithOptions(terraState, cache, ClusterInfraOptions{})
}

type ClusterInfraOptions struct {
	PhasedExecutionContext *phases.PhasedExecutionContext
}

func NewClusterInfraWithOptions(terraState StateLoader, cache state.Cache, opts ClusterInfraOptions) *ClusterInfra {
	return &ClusterInfra{
		stateLoader: terraState,
		cache:       cache,

		PhasedExecutionContext: opts.PhasedExecutionContext,
	}
}

func (r *ClusterInfra) DestroyCluster(autoApprove bool) error {
	metaConfig, err := r.stateLoader.PopulateMetaConfig()
	if err != nil {
		return err
	}

	clusterState, nodesState, err := r.stateLoader.PopulateClusterState()
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
		ngController, err := NewNodesController(metaConfig, r.cache, nodeGroupName, nodeGroupStates.Settings)
		if err != nil {
			return err
		}
		for name, ngState := range nodeGroupStates.State {
			err := ngController.DestroyNode(name, ngState, autoApprove)
			if err != nil {
				return err
			}
		}
	}

	if r.PhasedExecutionContext != nil {
		if shouldStop, err := r.PhasedExecutionContext.SwitchPhase(phases.BaseInfraPhase, true, r.cache); err != nil {
			return err
		} else if shouldStop {
			return nil
		}
	}

	if err := NewBaseInfraController(metaConfig, r.cache).Destroy(clusterState, autoApprove); err != nil {
		return err
	}

	if r.PhasedExecutionContext != nil {
		return r.PhasedExecutionContext.CompletePhase(r.cache)
	} else {
		return nil
	}
}
