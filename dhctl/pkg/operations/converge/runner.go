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
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manager"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/utils"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	noNodesConfirmationMessage = `Cluster has no nodes created by infrastructure utility. Do you want to continue and create nodes?`
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

func (r *runner) RunConvergeMigration(ctx *context.Context, checkHasTerraformStateBeforeMigration bool) error {
	if r.lockRunner != nil {
		err := r.lockRunner.Run(ctx.Ctx(), func() error {
			return r.convergeMigration(ctx, checkHasTerraformStateBeforeMigration)
		})

		if err != nil {
			return fmt.Errorf("failed to start lock runner: %w", err)
		}

		return nil
	}

	return r.convergeMigration(ctx, checkHasTerraformStateBeforeMigration)
}

func loadNodesState(ctx *context.Context) (map[string]state.NodeGroupInfrastructureState, error) {
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

	nodesState, err := infrastructurestate.GetNodesStateFromCluster(ctx.Ctx(), kubeCl)
	if err != nil {
		return nil, fmt.Errorf("infrastructure nodes state in Kubernetes cluster not found: %w", err)
	}

	return nodesState, nil
}

func populateNodesState(ctx *context.Context) (map[string]state.NodeGroupInfrastructureState, error) {
	var nodesState map[string]state.NodeGroupInfrastructureState
	err := log.Process("converge", "Gather Nodes infrastructure state", func() error {
		var err error
		nodesState, err = loadNodesState(ctx)
		return err
	})

	if err != nil {
		return nil, err
	}

	return nodesState, nil
}

func (r *runner) migrateTerraNodes(ctx *context.Context, metaConfig *config.MetaConfig, nodesState map[string]state.NodeGroupInfrastructureState) error {
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

	var nodeGroupsWithStateInCluster []string

	for _, group := range terraNodeGroups {
		// Skip if node group infrastructure state exists, we will update node group state below
		if _, ok := nodesState[group.Name]; ok {
			nodeGroupsWithStateInCluster = append(nodeGroupsWithStateInCluster, group.Name)
			continue
		}
	}

	for _, nodeGroupName := range utils.SortNodeGroupsStateKeys(nodesState, nodeGroupsWithStateInCluster) {
		ngState := nodesState[nodeGroupName]

		log.DebugF("NodeGroup for converge %v\n", nodeGroupName)

		rr := controller.NewNodeGroupControllerRunner(nodeGroupName, ngState, r.excludedNodes, true)
		err := rr.Run(ctx)
		if err != nil {
			return err
		}
	}

	return ctx.CompleteExecutionPhase(nil)
}

func (r *runner) convergeTerraNodes(ctx *context.Context, metaConfig *config.MetaConfig, nodesState map[string]state.NodeGroupInfrastructureState) error {
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
		// Skip if node group infrastructure state exists, we will update node group state below
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

	if err := bootstrapNewNodeGroups(
		ctx.Ctx(),
		ctx.KubeClient(),
		metaConfig,
		nodeGroupsWithoutStateInCluster,
		ctx.InfrastructureContext(metaConfig),
	); err != nil {
		return err
	}

	for _, nodeGroupName := range utils.SortNodeGroupsStateKeys(nodesState, nodeGroupsWithStateInCluster) {
		ngState := nodesState[nodeGroupName]

		log.DebugF("NodeGroup for converge %v", nodeGroupName)

		rr := controller.NewNodeGroupControllerRunner(nodeGroupName, ngState, r.excludedNodes, false)
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

func (r *runner) convergeMigration(ctx *context.Context, checkHasTerraformStateBeforeMigration bool) error {
	log.InfoF("Converge migration start\n")
	defer log.InfoF("Converge migration finished\n")

	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	providersGetter := ctx.ProviderGetter()
	if providersGetter == nil {
		return fmt.Errorf("Provider getter not set for converge migration")
	}

	provider, err := providersGetter(ctx.Ctx(), metaConfig)
	if err != nil {
		return err
	}

	if !provider.NeedToUseTofu() {
		log.InfoF("Skipping migration. Provider %s does not support opentofu now\n", metaConfig.ProviderName)
		return nil
	}

	if checkHasTerraformStateBeforeMigration {
		stats, hasTerraFormState, err := check.CheckState(ctx.Ctx(), ctx.KubeClient(), metaConfig, ctx.InfrastructureContext(metaConfig), check.CheckStateOptions{
			CommanderMode: ctx.CommanderMode(),
			StateCache:    ctx.StateCache(),
		})

		if err != nil {
			return err
		}

		if !hasTerraFormState {
			log.InfoLn("Cluster do not have terraform state. Skipping migration")
			return nil
		}

		commanderError := ""
		if ctx.CommanderMode() {
			commanderError = " For fix to migrate to opentofy please " +
				"detach cluster from commander, converge on previous installer version manually and attach cluster to commander. " +
				"Show complete guide in D8NeedMigrateStateToOpenTofu alert"
		}

		if stats.Cluster.Status != check.OKStatus {
			return fmt.Errorf("Cluster state has no ok status.%s", commanderError)
		}

		for _, node := range stats.Node {
			if node.Status != check.OKStatus {
				return fmt.Errorf("Node %s state has no ok status.%s", node.Name, commanderError)
			}
		}
	}

	log.DebugLn("Start backup infrastructure states")

	var commanderMode *infrastructurestate.TofuBackupCommanderMode
	if ctx.CommanderMode() {
		commanderMode = &infrastructurestate.TofuBackupCommanderMode{
			Cache:      ctx.StateCache(),
			MetaConfig: metaConfig,
		}
	}
	err = infrastructurestate.NewTofuMigrationStateBackuper(ctx.KubeProvider(), log.GetDefaultLogger()).
		WithCommanderMode(commanderMode).
		BackupStates(ctx.Ctx())
	if err != nil {
		return err
	}

	log.DebugLn("End backup infrastructure states")

	if err := r.updateClusterState(ctx, metaConfig); err != nil {
		return err
	}

	nodesStates, err := populateNodesState(ctx)
	if err != nil {
		return err
	}

	if err := r.migrateTerraNodes(ctx, metaConfig, nodesStates); err != nil {
		return err
	}

	log.DebugLn("Restart infrastructure manager deployments")

	err = manager.RestartStateExporter(ctx.Ctx(), ctx.KubeProvider())
	if err != nil {
		return err
	}

	if !checkHasTerraformStateBeforeMigration {
		err = manager.RestartAutoConverger(ctx.Ctx(), ctx.KubeProvider())
		if err != nil {
			return err
		}
	} else {
		log.InfoF("Skip restarting autoconverger\n")
	}

	log.DebugLn("Restarting infrastructure manager deployments finished")

	return nil
}

func (r *runner) converge(ctx *context.Context) error {
	log.DebugF("Converge start\n")
	defer log.DebugF("Converge finisher\n")
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	skipInfrastructure := metaConfig.ClusterType == config.StaticClusterType

	if !skipInfrastructure && !r.isSkip(phases.BaseInfraPhase) {
		if err := r.updateClusterState(ctx, metaConfig); err != nil {
			return err
		}
	} else {
		log.InfoLn("Skip converge base infrastructure")
	}

	kubeClientSwitched := false

	if !skipInfrastructure && !r.isSkip(phases.AllNodesPhase) {
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

	if !r.isSkip(phases.DeckhouseConfigurationPhase) {
		err = r.convergeDeckhouseConfiguration(ctx, r.commanderUUID)
		if err != nil {
			return err
		}
	} else {
		log.InfoLn("Skip converge deckhouse configuration")
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

	err := log.Process("converge", "Update Cluster infrastructure state", func() error {
		var clusterState []byte
		var err error
		// NOTE: Cluster state loaded from target kubernetes cluster in default dhctl-converge.
		// NOTE: In the commander mode cluster state should exist in the local state cache.
		if !ctx.CommanderMode() {
			clusterState, err = infrastructurestate.GetClusterStateFromCluster(ctx.Ctx(), ctx.KubeClient())
			if err != nil {
				return fmt.Errorf("infrastructure cluster state in Kubernetes cluster not found: %w", err)
			}
			if clusterState == nil {
				return fmt.Errorf("kubernetes cluster has no state")
			}
		}

		baseRunner, err := ctx.InfrastructureContext(metaConfig).GetConvergeBaseInfraRunner(ctx.Ctx(), metaConfig, infrastructure.BaseInfraRunnerOptions{
			StateCache:                       ctx.StateCache(),
			ClusterState:                     clusterState,
			AdditionalStateSaverDestinations: []infrastructure.SaverDestination{infrastructurestate.NewClusterStateSaver(ctx)},
		}, ctx.ChangesSettings().AutomaticSettings)

		if err != nil {
			return err
		}

		outputs, err := infrastructure.ApplyPipeline(ctx.Ctx(), baseRunner, "Kubernetes cluster", infrastructure.GetBaseInfraResult)
		if err != nil {
			return err
		}

		if tomb.IsInterrupted() {
			return global.ErrConvergeInterrupted
		}

		return infrastructurestate.SaveClusterInfrastructureState(ctx.Ctx(), ctx.KubeClient(), outputs)
	})

	if err != nil {
		return err
	}

	return ctx.CompleteExecutionPhase(nil)
}
