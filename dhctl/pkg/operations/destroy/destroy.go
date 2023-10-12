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
	"fmt"
	infra "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

type Params struct {
	SSHClient   *ssh.Client
	StateCache  dhctlstate.Cache
	OnPhaseFunc phases.OnPhaseFunc

	SkipResources bool

	CommanderMode bool
	*commander.CommanderModeParams
}

type ClusterDestroyer struct {
	state           *State
	stateCache      dhctlstate.Cache
	terrStateLoader infra.StateLoader

	d8Destroyer  *DeckhouseDestroyer
	clusterInfra *infra.ClusterInfra

	skipResources bool

	*phases.PhasedExecutionContext
}

func NewClusterDestroyer(params *Params) (*ClusterDestroyer, error) {
	state := NewDestroyState(params.StateCache)
	pec := phases.NewPhasedExecutionContext(params.OnPhaseFunc)
	d8Destroyer := NewDeckhouseDestroyer(params.SSHClient, state)

	var terraStateLoader terraform.StateLoader
	if params.CommanderMode {
		metaConfig, err := commander.ParseMetaConfig(state.cache, params.CommanderModeParams)
		if err != nil {
			return nil, fmt.Errorf("unable to parse meta configuration: %w", err)
		}
		terraStateLoader = terraform.NewFileTerraStateLoader(state.cache, metaConfig)
	} else {
		terraStateLoader = terraform.NewLazyTerraStateLoader(terraform.NewCachedTerraStateLoader(d8Destroyer, state.cache))
	}

	clusterInfra := infra.NewClusterInfraWithOptions(terraStateLoader, state.cache, infra.ClusterInfraOptions{PhasedExecutionContext: pec})

	return &ClusterDestroyer{
		state:           state,
		stateCache:      params.StateCache,
		terrStateLoader: terraStateLoader,

		d8Destroyer:  d8Destroyer,
		clusterInfra: clusterInfra,

		skipResources: params.SkipResources,

		PhasedExecutionContext: pec,
	}, nil
}

func (d *ClusterDestroyer) DestroyCluster(autoApprove bool) error {
	var err error

	defer d.d8Destroyer.UnlockConverge(true)

	if err := d.PhasedExecutionContext.InitPipeline(d.stateCache); err != nil {
		return err
	}
	defer d.PhasedExecutionContext.Finalize(d.stateCache)

	if !d.skipResources {
		if shouldStop, err := d.PhasedExecutionContext.StartPhase(phases.DeleteResourcesPhase, false, d.stateCache); err != nil {
			return err
		} else if shouldStop {
			return nil
		}
		if err := d.d8Destroyer.DeleteResources(); err != nil {
			return err
		}
		if err := d.PhasedExecutionContext.CompletePhase(d.stateCache); err != nil {
			return err
		}
	}

	// populate cluster state in cache
	_, err = d.terrStateLoader.PopulateMetaConfig()
	if err != nil {
		return err
	}

	_, _, err = d.terrStateLoader.PopulateClusterState()
	if err != nil {
		return err
	}

	// only after load and save all states into cache
	// set resources as deleted
	if err := d.state.SetResourcesDestroyed(); err != nil {
		return err
	}

	// why only unwatch lock without request unlock
	// user may not delete resources and converge still working in cluster
	// all node groups removing may still in long time run and
	// we get race (destroyer destroy node group, auto applayer create nodes)
	d.d8Destroyer.UnlockConverge(false)
	// Stop proxy because we have already got all info from kubernetes-api
	d.d8Destroyer.StopProxy()

	if err := d.clusterInfra.DestroyCluster(autoApprove); err != nil {
		return err
	}

	d.state.Clean()
	return d.PhasedExecutionContext.CompletePipeline(d.stateCache)
}
