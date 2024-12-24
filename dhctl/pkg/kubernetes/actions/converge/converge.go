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

package converge

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	state_terraform "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	MasterNodeGroupName = "master"

	noNodesConfirmationMessage = `Cluster has no nodes created by Terraform. Do you want to continue and create nodes?`

	AutoConvergerIdentity = "terraform-auto-converger"
)

var (
	ErrConvergeInterrupted = errors.New("Interrupted.")
)

type Runner struct {
	PhasedExecutionContext phases.DefaultPhasedExecutionContext
	terraformContext       *terraform.TerraformContext

	kubeCl         *client.KubernetesClient
	changeSettings *terraform.ChangeActionSettings
	lockRunner     *InLockRunner

	excludedNodes map[string]bool
	skipPhases    map[Phase]bool

	commanderMode       bool
	commanderUUID       uuid.UUID
	commanderModeParams *commander.CommanderModeParams

	stateCache dstate.Cache
	stateStore StateStore
	state      *State
}

func NewRunner(kubeCl *client.KubernetesClient, lockRunner *InLockRunner, stateCache dstate.Cache, terraformContext *terraform.TerraformContext) *Runner {
	return &Runner{
		kubeCl:         kubeCl,
		changeSettings: &terraform.ChangeActionSettings{},
		lockRunner:     lockRunner,

		excludedNodes: make(map[string]bool),
		skipPhases:    make(map[Phase]bool),
		stateCache:    stateCache,

		terraformContext: terraformContext,
		stateStore:       NewInSecretStateStore(kubeCl.KubeClient),
	}
}

func (r *Runner) WithCommanderModeParams(params *commander.CommanderModeParams) *Runner {
	r.commanderModeParams = params
	return r
}

func (r *Runner) WithCommanderMode(commanderMode bool) *Runner {
	r.commanderMode = commanderMode
	return r
}

func (r *Runner) WithCommanderUUID(commanderUUID uuid.UUID) *Runner {
	r.commanderUUID = commanderUUID
	return r
}

func (r *Runner) WithPhasedExecutionContext(pec phases.DefaultPhasedExecutionContext) *Runner {
	r.PhasedExecutionContext = pec
	return r
}

func (r *Runner) WithChangeSettings(changeSettings *terraform.ChangeActionSettings) *Runner {
	r.changeSettings = changeSettings
	return r
}

func (r *Runner) WithExcludedNodes(nodes []string) *Runner {
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

func (r *Runner) WithSkipPhases(phases []Phase) *Runner {
	newMap := make(map[Phase]bool)
	for _, n := range phases {
		if n == "" {
			continue
		}
		newMap[n] = true
	}

	r.skipPhases = newMap
	return r
}

func (r *Runner) isSkip(phase Phase) bool {
	_, ok := r.skipPhases[phase]
	return ok
}

func (r *Runner) RunConverge() error {
	if r.lockRunner != nil {
		err := r.lockRunner.Run(r.converge)
		if err != nil {
			return fmt.Errorf("failed to start lock runner: %w", err)
		}

		return nil
	}

	return r.converge()
}

func (r *Runner) converge() error {
	convergeState, err := r.stateStore.GetState()
	if err != nil {
		return fmt.Errorf("failed to get converge state: %w", err)
	}

	r.state = convergeState

	var metaConfig *config.MetaConfig
	if r.commanderMode {
		metaConfig, err = commander.ParseMetaConfig(r.stateCache, r.commanderModeParams)
		if err != nil {
			return fmt.Errorf("unable to parse meta configuration: %w", err)
		}
	} else {
		metaConfig, err = GetMetaConfig(r.kubeCl)
		if err != nil {
			return err
		}
	}

	skipTerraform := metaConfig.ClusterType == config.StaticClusterType

	if !skipTerraform && !r.isSkip(PhaseBaseInfra) {
		if r.PhasedExecutionContext != nil {
			if shouldStop, err := r.PhasedExecutionContext.StartPhase(phases.BaseInfraPhase, true, r.stateCache); err != nil {
				return err
			} else if shouldStop {
				return nil
			}
		}

		if err := r.updateClusterState(metaConfig); err != nil {
			return err
		}

		if r.PhasedExecutionContext != nil {
			if err := r.PhasedExecutionContext.CompletePhase(r.stateCache, nil); err != nil {
				return err
			}
		}
	} else {
		log.InfoLn("Skip converge base infrastructure")
	}

	if !skipTerraform && !r.isSkip(PhaseAllNodes) {
		if r.PhasedExecutionContext != nil {
			if shouldStop, err := r.PhasedExecutionContext.StartPhase(phases.AllNodesPhase, true, r.stateCache); err != nil {
				return err
			} else if shouldStop {
				return nil
			}
		}

		var nodesState map[string]state.NodeGroupTerraformState
		err = log.Process("converge", "Gather Nodes Terraform state", func() error {
			// NOTE: Nodes state loaded from target kubernetes cluster in default dhctl-converge.
			// NOTE: In the commander mode nodes state should exist in the local state cache.
			if r.commanderMode {
				nodesState, err = LoadNodesStateForCommanderMode(r.stateCache, metaConfig, r.kubeCl)
				if err != nil {
					return fmt.Errorf("unable to load nodes state: %w", err)
				}
			} else {
				nodesState, err = state_terraform.GetNodesStateFromCluster(r.kubeCl)
				if err != nil {
					return fmt.Errorf("terraform nodes state in Kubernetes cluster not found: %w", err)
				}
			}

			return nil
		})
		if err != nil {
			return err
		}

		terraNodeGroups := metaConfig.GetTerraNodeGroups()

		desiredQuantity := metaConfig.MasterNodeGroupSpec.Replicas
		for _, group := range terraNodeGroups {
			desiredQuantity += group.Replicas
		}

		// dhctl has nodes to create, and there are no nodes in the cluster.
		if len(nodesState) == 0 && desiredQuantity > 0 {
			confirmation := input.NewConfirmation().WithYesByDefault().WithMessage(noNodesConfirmationMessage)
			if !r.changeSettings.AutoApprove && !confirmation.Ask() {
				log.InfoLn("Aborted")
				return nil
			}
		}

		var nodeGroupsWithStateInCluster []string
		var nodeGroupsWithoutStateInCluster []config.TerraNodeGroupSpec

		type checkResult struct {
			name    string
			buffLog *bytes.Buffer
			err     error
		}

		for _, group := range terraNodeGroups {
			// Skip if node group terraform state exists, we will update node group state below
			if _, ok := nodesState[group.Name]; ok {
				nodeGroupsWithStateInCluster = append(nodeGroupsWithStateInCluster, group.Name)
				continue
			}

			nodeGroupsWithoutStateInCluster = append(nodeGroupsWithoutStateInCluster, group)
		}
		if err := r.parallelCreatePreviouslyNotExistedNodeGroup(nodeGroupsWithoutStateInCluster, metaConfig); err != nil {
			return err
		}

		for _, nodeGroupName := range sortNodeGroupsStateKeys(nodesState, nodeGroupsWithStateInCluster) {
			ngState := nodesState[nodeGroupName]

			runner := NewNodeGroupControllerRunner(
				r.kubeCl,
				metaConfig,
				nodeGroupName,
				ngState,
				r.stateCache,
				r.terraformContext,
				r.commanderMode,
				r.changeSettings,
				r.excludedNodes,
				r.lockRunner,
				r.stateStore,
				r.state)

			if err := runner.Run(); err != nil {
				return err
			}
		}

		if r.PhasedExecutionContext != nil {
			if err := r.PhasedExecutionContext.CompletePhase(r.stateCache, nil); err != nil {
				return err
			}
		}
	} else {
		log.InfoLn("Skip converge nodes")
	}

	if r.commanderMode {
		if r.PhasedExecutionContext != nil {
			if shouldStop, err := r.PhasedExecutionContext.StartPhase(phases.InstallDeckhousePhase, false, r.stateCache); err != nil {
				return err
			} else if shouldStop {
				return nil
			}
		}

		clusterConfigurationData, err := metaConfig.ClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("unable to get cluster config yaml: %w", err)
		}

		providerClusterConfigurationData, err := metaConfig.ProviderClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("unable to get provider cluster config yaml: %w", err)
		}

		systemRegistryConfigurationData, err := metaConfig.SystemRegistryConfig.ToYAML()
		if err != nil {
			return fmt.Errorf("unable to get provider system registry config yaml: %w", err)
		}

		clusterUUID, err := uuid.Parse(metaConfig.UUID)
		if err != nil {
			return fmt.Errorf("unable to parse cluster uuid %q: %w", metaConfig.UUID, err)
		}

		if err := deckhouse.ConvergeDeckhouseConfiguration(context.TODO(), r.kubeCl, clusterUUID, r.commanderUUID, clusterConfigurationData, providerClusterConfigurationData, systemRegistryConfigurationData); err != nil {
			return fmt.Errorf("unable to update deckhouse configuration: %w", err)
		}

		if r.PhasedExecutionContext != nil {
			if err := r.PhasedExecutionContext.CompletePhase(r.stateCache, nil); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Runner) updateClusterState(metaConfig *config.MetaConfig) error {
	return log.Process("converge", "Update Cluster Terraform state", func() error {
		var clusterState []byte
		var err error
		// NOTE: Cluster state loaded from target kubernetes cluster in default dhctl-converge.
		// NOTE: In the commander mode cluster state should exist in the local state cache.
		if !r.commanderMode {
			clusterState, err = state_terraform.GetClusterStateFromCluster(r.kubeCl)
			if err != nil {
				return fmt.Errorf("terraform cluster state in Kubernetes cluster not found: %w", err)
			}
			if clusterState == nil {
				return fmt.Errorf("kubernetes cluster has no state")
			}
		}

		baseRunner := r.terraformContext.GetConvergeBaseInfraRunner(metaConfig, terraform.BaseInfraRunnerOptions{
			AutoDismissDestructive:           r.changeSettings.AutoDismissDestructive,
			AutoApprove:                      r.changeSettings.AutoApprove,
			CommanderMode:                    r.commanderMode,
			StateCache:                       r.stateCache,
			ClusterState:                     clusterState,
			AdditionalStateSaverDestinations: []terraform.SaverDestination{NewClusterStateSaver(r.kubeCl)},
		})

		outputs, err := terraform.ApplyPipeline(baseRunner, "Kubernetes cluster", terraform.GetBaseInfraResult)
		if err != nil {
			return err
		}

		if tomb.IsInterrupted() {
			return ErrConvergeInterrupted
		}

		return SaveClusterTerraformState(r.kubeCl, outputs)
	})
}

func (r *Runner) parallelCreatePreviouslyNotExistedNodeGroup(groups []config.TerraNodeGroupSpec, metaConfig *config.MetaConfig) error {
	return ParallelCreateNodeGroup(r.kubeCl, metaConfig, groups, r.terraformContext)
}

func GetMetaConfig(kubeCl *client.KubernetesClient) (*config.MetaConfig, error) {
	metaConfig, err := config.ParseConfigFromCluster(kubeCl)
	if err != nil {
		return nil, err
	}

	metaConfig.UUID, err = state_terraform.GetClusterUUID(kubeCl)
	if err != nil {
		return nil, err
	}

	return metaConfig, nil
}
