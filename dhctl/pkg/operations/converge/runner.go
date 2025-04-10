// Copyright 2024 Flant JSC
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

package converge

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/utils"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	state_terraform "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	noNodesConfirmationMessage = `Cluster has no nodes created by Terraform. Do you want to continue and create nodes?`
)

type runner struct {
	excludedNodes map[string]bool
	skipPhases    map[phases.OperationPhase]bool
	commanderUUID uuid.UUID

	lockRunner *lock.InLockRunner
	switcher   *context.KubeClientSwitcher
}

func newRunner(inLockRunner *lock.InLockRunner, switcher *context.KubeClientSwitcher) *runner {
	return &runner{
		excludedNodes: make(map[string]bool),
		skipPhases:    make(map[phases.OperationPhase]bool),

		lockRunner: inLockRunner,
		switcher:   switcher,
	}
}

func (r *runner) WithExcludedNodes(nodes []string) *runner {
	newMap := make(map[string]bool)
	for _, n := range nodes {
		if n == "" {
			continue
		}
		newMap[n] = true
	}

	r.excludedNodes = newMap
	return r
}

func (r *runner) WithCommanderUUID(id uuid.UUID) *runner {
	r.commanderUUID = id
	return r
}

func (r *runner) WithSkipPhases(phs []phases.OperationPhase) *runner {
	newMap := make(map[phases.OperationPhase]bool)
	for _, n := range phs {
		if n == "" {
			continue
		}
		newMap[n] = true
	}

	r.skipPhases = newMap
	return r
}

func (r *runner) isSkip(phase phases.OperationPhase) bool {
	_, ok := r.skipPhases[phase]
	return ok
}

func (r *runner) RunConverge(ctx *context.Context) error {
	if r.lockRunner != nil {
		err := r.lockRunner.Run(ctx.Ctx(), func() error {
			return r.converge(ctx)
		})

		if err != nil {
			return fmt.Errorf("failed to start lock runner: %w", err)
		}

		return nil
	}

	return r.converge(ctx)
}

func loadNodesState(ctx *context.Context) (map[string]state.NodeGroupTerraformState, error) {
	kubeCl := ctx.KubeClient()
	// NOTE: Nodes state loaded from target kubernetes cluster in default dhctl-converge.
	// NOTE: In the commander mode nodes state should exist in the local state cache.
	if ctx.CommanderMode() {
		metaConfig, err := ctx.MetaConfig()
		if err != nil {
			return nil, nil
		}

		nodesState, err := check.LoadNodesStateForCommanderMode(ctx.Ctx(), ctx.StateCache(), metaConfig, kubeCl)
		if err != nil {
			return nil, fmt.Errorf("unable to load nodes state: %w", err)
		}

		return nodesState, nil
	}

	nodesState, err := state_terraform.GetNodesStateFromCluster(ctx.Ctx(), kubeCl)
	if err != nil {
		return nil, fmt.Errorf("terraform nodes state in Kubernetes cluster not found: %w", err)
	}

	return nodesState, nil
}

func populateNodesState(ctx *context.Context) (map[string]state.NodeGroupTerraformState, error) {
	var nodesState map[string]state.NodeGroupTerraformState
	err := log.Process("converge", "Gather Nodes Terraform state", func() error {
		var err error
		nodesState, err = loadNodesState(ctx)
		return err
	})

	if err != nil {
		return nil, err
	}

	return nodesState, nil
}

func (r *runner) convergeTerraNodes(ctx *context.Context, metaConfig *config.MetaConfig, nodesState map[string]state.NodeGroupTerraformState) error {
	if shouldStop, err := ctx.StarExecutionPhase(phases.AllNodesPhase, true); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	terraNodeGroups := metaConfig.GetTerraNodeGroups()

	desiredQuantity := metaConfig.MasterNodeGroupSpec.Replicas
	for _, group := range terraNodeGroups {
		desiredQuantity += group.Replicas
	}

	// dhctl has nodes to create, and there are no nodes in the cluster.
	if len(nodesState) == 0 && desiredQuantity > 0 {
		confirmation := input.NewConfirmation().WithYesByDefault().WithMessage(noNodesConfirmationMessage)
		if !ctx.ChangesSettings().AutoApprove && !confirmation.Ask() {
			log.InfoLn("Aborted")
			return nil
		}
	}

	var nodeGroupsWithStateInCluster []string
	var nodeGroupsWithoutStateInCluster []config.TerraNodeGroupSpec

	for _, group := range terraNodeGroups {
		// Skip if node group terraform state exists, we will update node group state below
		if _, ok := nodesState[group.Name]; ok {
			nodeGroupsWithStateInCluster = append(nodeGroupsWithStateInCluster, group.Name)
			continue
		}

		nodeGroupsWithoutStateInCluster = append(nodeGroupsWithoutStateInCluster, group)
	}

	log.DebugF("NodeGroups for creating %v\n", nodeGroupsWithoutStateInCluster)

	bootstrapNewNodeGroups := operations.ParallelCreateNodeGroup
	if operations.IsSequentialNodesBootstrap() || metaConfig.ProviderName == "vcd" {
		// vcd doesn't support parrallel creating nodes in same vapp
		// https://github.com/vmware/terraform-provider-vcd/issues/530
		bootstrapNewNodeGroups = operations.BootstrapSequentialTerraNodes
	}

	if err := bootstrapNewNodeGroups(ctx.Ctx(), ctx.KubeClient(), metaConfig, nodeGroupsWithoutStateInCluster, ctx.Terraform()); err != nil {
		return err
	}

	for _, nodeGroupName := range utils.SortNodeGroupsStateKeys(nodesState, nodeGroupsWithStateInCluster) {
		ngState := nodesState[nodeGroupName]

		log.DebugF("NodeGroup for converge %v", nodeGroupName)

		rr := controller.NewNodeGroupControllerRunner(nodeGroupName, ngState, r.excludedNodes)
		err := rr.Run(ctx)
		if err != nil {
			return err
		}
	}

	return ctx.CompleteExecutionPhase(nil)
}

func (r *runner) convergeDeckhouseConfiguration(ctx *context.Context, commanderUUID uuid.UUID) error {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	if shouldStop, err := ctx.StarExecutionPhase(phases.InstallDeckhousePhase, false); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	clusterConfigurationData, err := metaConfig.ClusterConfigYAML()
	if err != nil {
		return fmt.Errorf("unable to get cluster config yaml: %w", err)
	}
	providerClusterConfigurationData, err := metaConfig.ProviderClusterConfigYAML()
	if err != nil {
		return fmt.Errorf("unable to get provider cluster config yaml: %w", err)
	}

	clusterUUID, err := uuid.Parse(metaConfig.UUID)
	if err != nil {
		return fmt.Errorf("unable to parse cluster uuid %q: %w", metaConfig.UUID, err)
	}

	if err := deckhouse.ConvergeDeckhouseConfiguration(ctx.Ctx(), ctx.KubeClient(), clusterUUID, commanderUUID, clusterConfigurationData, providerClusterConfigurationData); err != nil {
		return fmt.Errorf("unable to update deckhouse configuration: %w", err)
	}

	return ctx.CompleteExecutionPhase(nil)
}

func (r *runner) converge(ctx *context.Context) error {
	log.DebugF("Converge start\n")
	defer log.DebugF("Converge finisher\n")
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	skipTerraform := metaConfig.ClusterType == config.StaticClusterType

	if !skipTerraform && !r.isSkip(phases.BaseInfraPhase) {
		if err := r.updateClusterState(ctx, metaConfig); err != nil {
			return err
		}
	} else {
		log.InfoLn("Skip converge base infrastructure")
	}

	kubeClientSwitched := false

	if !skipTerraform && !r.isSkip(phases.AllNodesPhase) {
		nodesStates, err := populateNodesState(ctx)
		if err != nil {
			return err
		}

		err = r.switcher.SwitchToNodeUser(nodesStates[global.MasterNodeGroupName].State)
		if err != nil {
			return err
		}

		kubeClientSwitched = true

		if err := r.convergeTerraNodes(ctx, metaConfig, nodesStates); err != nil {
			return err
		}
	} else {
		log.InfoLn("Skip converge nodes")
	}

	err = r.convergeDeckhouseConfiguration(ctx, r.commanderUUID)
	if err != nil {
		return err
	}

	if kubeClientSwitched {
		return r.switcher.CleanupNodeUser()
	}

	return nil
}

func (r *runner) updateClusterState(ctx *context.Context, metaConfig *config.MetaConfig) error {
	if shouldStop, err := ctx.StarExecutionPhase(phases.BaseInfraPhase, true); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	err := log.Process("converge", "Update Cluster Terraform state", func() error {
		var clusterState []byte
		var err error
		// NOTE: Cluster state loaded from target kubernetes cluster in default dhctl-converge.
		// NOTE: In the commander mode cluster state should exist in the local state cache.
		if !ctx.CommanderMode() {
			clusterState, err = state_terraform.GetClusterStateFromCluster(ctx.Ctx(), ctx.KubeClient())
			if err != nil {
				return fmt.Errorf("terraform cluster state in Kubernetes cluster not found: %w", err)
			}
			if clusterState == nil {
				return fmt.Errorf("kubernetes cluster has no state")
			}
		}

		changeSettings := ctx.ChangesSettings()

		baseRunner := ctx.Terraform().GetConvergeBaseInfraRunner(metaConfig, terraform.BaseInfraRunnerOptions{
			AutoDismissDestructive:           changeSettings.AutoDismissDestructive,
			AutoApprove:                      changeSettings.AutoApprove,
			StateCache:                       ctx.StateCache(),
			ClusterState:                     clusterState,
			AdditionalStateSaverDestinations: []terraform.SaverDestination{entity.NewClusterStateSaver(ctx)},
		})

		outputs, err := terraform.ApplyPipeline(ctx.Ctx(), baseRunner, "Kubernetes cluster", terraform.GetBaseInfraResult)
		if err != nil {
			return err
		}

		if tomb.IsInterrupted() {
			return global.ErrConvergeInterrupted
		}

		return entity.SaveClusterTerraformState(ctx.Ctx(), ctx.KubeClient(), outputs)
	})

	if err != nil {
		return err
	}

	return ctx.CompleteExecutionPhase(nil)
}
