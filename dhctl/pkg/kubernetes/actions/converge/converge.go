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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	gcmp "github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	multierror "github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infra/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	state_terraform "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/maputil"
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
		err := r.lockRunner.Run()
		if err != nil {
			return fmt.Errorf("failed to start lock runner: %w", err)
		}
	}

	return r.converge()
}

func (r *Runner) converge() (err error) {
	state, err := r.stateStore.GetState()
	if err != nil {
		return fmt.Errorf("failed to get converge state: %w", err)
	}

	r.state = state

	if r.state.NodeUserCredentials == nil {
		nodeUser, nodeUserCredentials, err := generateNodeUser()
		if err != nil {
			return fmt.Errorf("failed to generate NodeUser: %w", err)
		}

		err = createNodeUser(context.TODO(), r.kubeCl, nodeUser)
		if err != nil {
			return fmt.Errorf("failed to create or update NodeUser: %w", err)
		}

		r.state.NodeUserCredentials = nodeUserCredentials

		err = r.stateStore.SetState(r.state)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}
	}

	err = r.convergePhase("")
	if err != nil {
		return err
	}

	err = r.convergeWithReplicas(nil)
	if err != nil {
		if errors.Is(err, controlplane.ErrSingleMasterClusterTerraformPlanHasDestructiveChanges) {
			confirmation := input.NewConfirmation().WithMessage("A single-master cluster has disruptive changes in the Terraform plan. Trying to migrate to a multi-master cluster and back to a single-master cluster. Do you want to continue?")
			if !r.changeSettings.AutoApprove && !confirmation.Ask() {
				log.InfoLn("Aborted")
				return nil
			}

			err := r.convergePhase(PhaseScaleToMultiMaster)
			if err != nil {
				return fmt.Errorf("failed to converge to multi-master: %w", err)
			}
		}

		return err
	}

	return nil
}

func (r *Runner) convergePhase(phase Phase) (err error) {
	if phase != "" {
		r.state.Phase = phase

		err := r.stateStore.SetState(r.state)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}
	}

	if r.state.Phase == PhaseScaleToMultiMaster {
		replicas := 3

		err = r.convergeWithReplicas(&replicas)
		if err != nil {
			return fmt.Errorf("failed to converge with 3 replicas: %w", err)
		}

		r.state.Phase = PhaseScaleToSingleMaster

		err := r.stateStore.SetState(r.state)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}
	}

	if r.state.Phase == PhaseScaleToSingleMaster {
		replicas := 1

		err := r.convergeWithReplicas(&replicas)
		if err != nil {
			return fmt.Errorf("failed to converge with 1 replica: %w", err)
		}

		r.state.Phase = ""

		err = r.stateStore.SetState(r.state)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}
	}

	return nil
}

func (r *Runner) convergeWithReplicas(replicas *int) error {
	var metaConfig *config.MetaConfig
	var err error
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

	if replicas != nil {
		metaConfig.MasterNodeGroupSpec.Replicas = *replicas
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

		if len(nodesState) > 0 {
			state, ok := nodesState[MasterNodeGroupName]
			if ok {
				err := r.replaceKubeClient(context.TODO(), state.State)
				if err != nil {
					return fmt.Errorf("failed to replace kube client: %w", err)
				}
			}
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

		for _, group := range terraNodeGroups {
			// Skip if node group terraform state exists, we will update node group state below
			if _, ok := nodesState[group.Name]; ok {
				nodeGroupsWithStateInCluster = append(nodeGroupsWithStateInCluster, group.Name)
				continue
			}
			if err := r.createPreviouslyNotExistedNodeGroup(group, metaConfig); err != nil {
				return err
			}
		}

		for _, nodeGroupName := range sortNodeGroupsStateKeys(nodesState, nodeGroupsWithStateInCluster) {
			ngState := nodesState[nodeGroupName]
			controller := NewConvergeController(r.kubeCl, metaConfig, nodeGroupName, ngState, r.stateCache, r.terraformContext)
			controller.WithChangeSettings(r.changeSettings)
			controller.WithCommanderMode(r.commanderMode)
			controller.WithExcludedNodes(r.excludedNodes)

			if err := controller.Run(); err != nil {
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

		clusterUUID, err := uuid.Parse(metaConfig.UUID)
		if err != nil {
			return fmt.Errorf("unable to parse cluster uuid %q: %w", metaConfig.UUID, err)
		}

		if err := deckhouse.ConvergeDeckhouseConfiguration(context.TODO(), r.kubeCl, clusterUUID, r.commanderUUID, clusterConfigurationData, providerClusterConfigurationData); err != nil {
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

func (r *Runner) replaceKubeClient(ctx context.Context, state map[string][]byte) (err error) {
	tmpDir := filepath.Join(app.CacheDir, "converge")

	err = os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create cache directory for NodeUser: %w", err)
	}

	privateKeyPath := filepath.Join(tmpDir, "id_rsa")

	err = os.WriteFile(privateKeyPath, []byte(r.state.NodeUserCredentials.PrivateKey), 0o600)
	if err != nil {
		return fmt.Errorf("failed to write private key for NodeUser: %w", err)
	}

	settings := r.kubeCl.SSHClient.Settings

	for nodeName, stateBytes := range state {
		statePath := filepath.Join(tmpDir, fmt.Sprintf("%s.tfstate", nodeName))

		err := os.WriteFile(statePath, stateBytes, 0o644)
		if err != nil {
			return fmt.Errorf("failed to write terraform state: %w", err)
		}

		ipAddress, err := terraform.GetMasterIPAddressForSSH(statePath)
		if err != nil {
			return fmt.Errorf("failed to get master IP address: %w", err)
		}

		settings.AddAvailableHosts(ipAddress)
	}

	if r.lockRunner != nil {
		r.lockRunner.Stop()
	}

	r.kubeCl.KubeProxy.StopAll()
	r.kubeCl.SSHClient.Stop()

	sshClient, err := ssh.NewClient(session.NewSession(session.Input{
		User:           r.state.NodeUserCredentials.Name,
		Port:           settings.Port,
		BastionHost:    settings.BastionHost,
		BastionPort:    settings.BastionPort,
		BastionUser:    settings.BastionUser,
		ExtraArgs:      settings.ExtraArgs,
		AvailableHosts: settings.AvailableHosts(),
		BecomePass:     r.state.NodeUserCredentials.Password,
	}), []session.AgentPrivateKey{
		{
			Key:        privateKeyPath,
			Passphrase: r.state.NodeUserCredentials.Password,
		},
	}).Start()
	if err != nil {
		return fmt.Errorf("failed to start SSH client: %w", err)
	}

	kubeClient, err := kubernetes.ConnectToKubernetesAPI(sshClient)
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes API: %w", err)
	}

	r.kubeCl = kubeClient

	if r.lockRunner != nil {
		r.lockRunner = NewInLockLocalRunner(r.kubeCl, "local-converger")

		err := r.lockRunner.Run()
		if err != nil {
			return fmt.Errorf("failed to start lock runner: %w", err)
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

func (r *Runner) createPreviouslyNotExistedNodeGroup(group config.TerraNodeGroupSpec, metaConfig *config.MetaConfig) error {
	return log.Process("converge", fmt.Sprintf("Add NodeGroup %s (replicas: %v)️", group.Name, group.Replicas), func() error {
		err := CreateNodeGroup(r.kubeCl, group.Name, metaConfig.NodeGroupManifest(group))
		if err != nil {
			return err
		}

		nodeCloudConfig, err := GetCloudConfig(r.kubeCl, group.Name, ShowDeckhouseLogs)
		if err != nil {
			return err
		}

		for i := 0; i < group.Replicas; i++ {
			err = BootstrapAdditionalNode(r.kubeCl, metaConfig, i, "static-node", group.Name, nodeCloudConfig, true, r.terraformContext)
			if err != nil {
				return err
			}
		}

		return WaitForNodesBecomeReady(r.kubeCl, group.Name, group.Replicas)
	})
}

type NodeGroupController struct {
	client         *client.KubernetesClient
	config         *config.MetaConfig
	changeSettings *terraform.ChangeActionSettings

	excludedNodes map[string]bool

	stateCache dstate.Cache

	nodeToHost map[string]string
	name       string
	state      state.NodeGroupTerraformState

	commanderMode    bool
	terraformContext *terraform.TerraformContext
}

type NodeGroupGroupOptions struct {
	Name            string
	Step            string
	CloudConfig     string
	DesiredReplicas int
	State           map[string][]byte
}

func (n *NodeGroupGroupOptions) CurReplicas() int {
	return len(n.State)
}

func NewConvergeController(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, name string, state state.NodeGroupTerraformState, stateCache dstate.Cache, terraformContext *terraform.TerraformContext) *NodeGroupController {
	return &NodeGroupController{
		client:           kubeCl,
		config:           metaConfig,
		changeSettings:   &terraform.ChangeActionSettings{},
		excludedNodes:    make(map[string]bool),
		stateCache:       stateCache,
		terraformContext: terraformContext,

		name:  name,
		state: state,
	}
}

func (c *NodeGroupController) WithCommanderMode(commanderMode bool) *NodeGroupController {
	c.commanderMode = commanderMode
	return c
}

func (c *NodeGroupController) WithChangeSettings(changeSettings *terraform.ChangeActionSettings) *NodeGroupController {
	c.changeSettings = changeSettings
	return c
}

func (c *NodeGroupController) WithExcludedNodes(nodesMap map[string]bool) *NodeGroupController {
	c.excludedNodes = nodesMap
	return c
}

func (c *NodeGroupController) populateNodeToHost() error {
	if c.name != MasterNodeGroupName {
		c.nodeToHost = make(map[string]string)
		return nil
	}

	var userPassedHosts []string
	if c.client.SSHClient != nil {
		userPassedHosts = c.client.SSHClient.Settings.AvailableHosts()
	}

	nodesNames := make([]string, 0, len(c.state.State))
	for nodeName := range c.state.State {
		nodesNames = append(nodesNames, nodeName)
	}

	nodeToHost, err := ssh.CheckSSHHosts(userPassedHosts, nodesNames, func(msg string) bool {
		if c.commanderMode {
			return true
		}
		return input.NewConfirmation().WithMessage(msg).Ask()
	})

	if err != nil {
		return err
	}

	c.nodeToHost = nodeToHost

	return nil
}

func (c *NodeGroupController) newHookForUpdatePipeline(convergedNode string) terraform.InfraActionHook {
	if c.name != MasterNodeGroupName {
		// for non-master node groups return a stub
		return &terraform.DummyHook{}
	}

	nodesToCheck := maputil.ExcludeKeys(c.nodeToHost, convergedNode)

	confirm := func(msg string) bool {
		return input.NewConfirmation().WithMessage(msg).Ask()
	}

	if c.changeSettings.AutoApprove {
		confirm = func(_ string) bool {
			return true
		}
	}

	return controlplane.NewHookForUpdatePipeline(c.client, nodesToCheck, c.config.UUID).
		WithSourceCommandName("converge").
		WithNodeToConverge(convergedNode).
		WithConfirm(confirm)
}

func (c *NodeGroupController) newHookForDestroyPipeline(convergedNode string) terraform.InfraActionHook {
	if c.name != MasterNodeGroupName {
		// for non-master node groups return a stub
		return &terraform.DummyHook{}
	}

	return controlplane.NewHookForDestroyPipeline(c.client, convergedNode)
}

func (c *NodeGroupController) Run() error {
	nodeGroupName := c.name

	replicas := getReplicasByNodeGroupName(c.config, nodeGroupName)
	step := getStepByNodeGroupName(nodeGroupName)

	// we hide deckhouse logs because we always have config
	nodeCloudConfig, err := GetCloudConfig(c.client, nodeGroupName, HideDeckhouseLogs)
	if err != nil {
		return err
	}

	nodeGroup := &NodeGroupGroupOptions{
		Name:            nodeGroupName,
		Step:            step,
		DesiredReplicas: replicas,
		CloudConfig:     nodeCloudConfig,
		State:           c.state.State,
	}

	if nodeGroup.DesiredReplicas > len(nodeGroup.State) {
		err := log.Process("converge", fmt.Sprintf("Add Nodes to NodeGroup %s (replicas: %v)", nodeGroupName, replicas), func() error {
			return c.addNewNodesToGroup(nodeGroup)
		})
		if err != nil {
			return err
		}
	}

	nodesToDeleteInfo, err := c.getNodesToDeleteInfo(nodeGroup)
	if err != nil {
		return err
	}

	err = c.updateNodes(nodeGroup)
	if err != nil {
		return err
	}

	err = c.tryDeleteNodes(nodesToDeleteInfo, nodeGroup)
	if err != nil {
		return err
	}

	groupSpec := c.getSpec(nodeGroup.Name)
	if groupSpec == nil {
		return c.tryDeleteNodeGroup(nodeGroup)
	}

	return c.tryUpdateNodeTemplate(nodeGroup, groupSpec.NodeTemplate)
}

func (c *NodeGroupController) addNewNodesToGroup(nodeGroup *NodeGroupGroupOptions) error {
	count := len(nodeGroup.State)
	index := 0

	var (
		nodesToWait        []string
		masterIPForSSHList []string
		nodeInternalIPList []string
	)

	for nodeGroup.DesiredReplicas > count {
		candidateName := fmt.Sprintf("%s-%s-%v", c.config.ClusterPrefix, nodeGroup.Name, index)

		if _, ok := nodeGroup.State[candidateName]; !ok {
			var err error
			var output *terraform.PipelineOutputs
			if nodeGroup.Name == MasterNodeGroupName {
				output, err = BootstrapAdditionalMasterNode(c.client, c.config, index, nodeGroup.CloudConfig, true, c.terraformContext)
				if err != nil {
					return err
				}

				masterIPForSSHList = append(masterIPForSSHList, output.MasterIPForSSH)
				nodeInternalIPList = append(nodeInternalIPList, output.NodeInternalIP)
			} else {
				err = BootstrapAdditionalNode(c.client, c.config, index, nodeGroup.Step, nodeGroup.Name, nodeGroup.CloudConfig, true, c.terraformContext)
				if err != nil {
					return err
				}
			}

			count++
			if output != nil {
				nodeGroup.State[candidateName] = output.TerraformState
			}
			nodesToWait = append(nodesToWait, candidateName)
		}
		index++
	}

	if nodeGroup.Name == MasterNodeGroupName {
		err := WaitForNodesListBecomeReady(c.client, nodesToWait, controlplane.NewManagerReadinessChecker(c.client))
		if err != nil {
			return err
		}

		if len(masterIPForSSHList) > 0 {
			c.client.SSHClient.Settings.AddAvailableHosts(masterIPForSSHList...)

			// we hide deckhouse logs because we always have config
			nodeCloudConfig, err := GetCloudConfig(c.client, nodeGroup.Name, HideDeckhouseLogs, nodeInternalIPList...)
			if err != nil {
				return err
			}

			nodeGroup.CloudConfig = nodeCloudConfig
		}
	}

	return WaitForNodesListBecomeReady(c.client, nodesToWait, nil)
}

func (c *NodeGroupController) updateNode(nodeGroup *NodeGroupGroupOptions, nodeName string) error {
	if _, ok := c.excludedNodes[nodeName]; ok {
		log.InfoF("Skip update excluded node %v\n", nodeName)
		return nil
	}

	// NOTE: In the commander mode nodes state should exist in the local state cache, no need to pass state explicitly.
	var nodeState []byte
	if !c.commanderMode {
		nodeState = nodeGroup.State[nodeName]
	}

	nodeIndex, err := config.GetIndexFromNodeName(nodeName)
	if err != nil {
		log.ErrorF("can't extract index from terraform state secret (%v), skip %s\n", err, nodeName)
		return nil
	}

	hook := c.newHookForUpdatePipeline(nodeName)

	pipelineForMaster := nodeGroup.Step == "master-node"

	extractOutputFunc := terraform.OnlyState
	nodeGroupName := nodeGroup.Name
	var nodeGroupSettingsFromConfig []byte
	if pipelineForMaster {
		extractOutputFunc = terraform.GetMasterNodeResult
		nodeGroupName = MasterNodeGroupName
	} else {
		// Node group settings are only for the static node.
		nodeGroupSettingsFromConfig = c.config.FindTerraNodeGroup(nodeGroup.Name)
	}

	nodeRunner := c.terraformContext.GetConvergeNodeRunner(c.config, terraform.NodeRunnerOptions{
		AutoDismissDestructive: c.changeSettings.AutoDismissDestructive,
		AutoApprove:            c.changeSettings.AutoApprove,
		NodeName:               nodeName,
		NodeGroupName:          nodeGroup.Name,
		NodeGroupStep:          nodeGroup.Step,
		NodeIndex:              nodeIndex,
		NodeState:              nodeState,
		NodeCloudConfig:        nodeGroup.CloudConfig,
		CommanderMode:          c.commanderMode,
		StateCache:             c.stateCache,
		AdditionalStateSaverDestinations: []terraform.SaverDestination{
			NewNodeStateSaver(c.client, nodeName, nodeGroupName, nodeGroupSettingsFromConfig),
		},
		Hook: hook,
	})

	outputs, err := terraform.ApplyPipeline(nodeRunner, nodeName, extractOutputFunc)
	if err != nil {
		log.ErrorF("Terraform exited with an error:\n%s\n", err.Error())
		return err
	}

	if tomb.IsInterrupted() {
		return ErrConvergeInterrupted
	}

	if pipelineForMaster {
		err = SaveMasterNodeTerraformState(c.client, nodeName, outputs.TerraformState, []byte(outputs.KubeDataDevicePath))
		if err != nil {
			return err
		}
	} else {
		err = SaveNodeTerraformState(c.client, nodeName, nodeGroup.Name, outputs.TerraformState, nodeGroupSettingsFromConfig)
		if err != nil {
			return err
		}
	}

	return WaitForSingleNodeBecomeReady(c.client, nodeName)
}

func (c *NodeGroupController) deleteRedundantNodes(nodeGroup *NodeGroupGroupOptions, settings []byte, nodesToDeleteInfo []nodeToDeleteInfo) error {
	if c.changeSettings.AutoDismissDestructive {
		return nil
	}

	cfg := c.config
	if settings != nil {
		nodeGroupsSettings, err := json.Marshal([]json.RawMessage{settings})
		if err != nil {
			log.ErrorLn(err)
		} else {
			cfg, err = c.config.DeepCopy().Prepare()
			if err != nil {
				return fmt.Errorf("unable to prepare copied config: %v", err)
			}
			cfg.ProviderClusterConfig["nodeGroups"] = nodeGroupsSettings
		}
	}

	var allErrs *multierror.Error
	for _, nodeToDeleteInfo := range nodesToDeleteInfo {
		if _, ok := c.excludedNodes[nodeToDeleteInfo.name]; ok {
			log.InfoF("Skip delete excluded node %v\n", nodeToDeleteInfo.name)
			continue
		}

		nodeIndex, err := config.GetIndexFromNodeName(nodeToDeleteInfo.name)
		if err != nil {
			log.ErrorF("can't extract index from terraform state secret (%v), skip %s\n", err, nodeToDeleteInfo.name)
			return nil
		}

		// NOTE: In the commander mode nodes state should exist in the local state cache, no need to pass state explicitly.
		var nodeState []byte
		if !c.commanderMode {
			nodeState = nodeToDeleteInfo.state
		}

		hook := c.newHookForDestroyPipeline(nodeToDeleteInfo.name)

		nodeRunner := c.terraformContext.GetConvergeNodeDeleteRunner(cfg, terraform.NodeDeleteRunnerOptions{
			AutoDismissDestructive: c.changeSettings.AutoDismissDestructive,
			AutoApprove:            c.changeSettings.AutoApprove,
			NodeName:               nodeToDeleteInfo.name,
			NodeGroupName:          nodeGroup.Name,
			NodeGroupStep:          nodeGroup.Step,
			NodeIndex:              nodeIndex,
			NodeState:              nodeState,
			NodeCloudConfig:        nodeGroup.CloudConfig,
			CommanderMode:          c.commanderMode,
			StateCache:             c.stateCache,
			AdditionalStateSaverDestinations: []terraform.SaverDestination{
				NewNodeStateSaver(c.client, nodeToDeleteInfo.name, nodeGroup.Name, nil),
			},
			Hook: hook,
		})

		if err := terraform.DestroyPipeline(nodeRunner, nodeToDeleteInfo.name); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", nodeToDeleteInfo.name, err))
			continue
		}

		if tomb.IsInterrupted() {
			allErrs = multierror.Append(allErrs, ErrConvergeInterrupted)
			return allErrs.ErrorOrNil()
		}

		if err := DeleteNode(c.client, nodeToDeleteInfo.name); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", nodeToDeleteInfo.name, err))
			continue
		}

		if err := state_terraform.DeleteNodeTerraformStateFromCache(nodeToDeleteInfo.name, c.stateCache); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("unable to delete node %s terraform state from cache: %w", nodeToDeleteInfo.name, err))
			continue
		}

		if err := DeleteTerraformState(c.client, fmt.Sprintf("d8-node-terraform-state-%s", nodeToDeleteInfo.name)); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", nodeToDeleteInfo.name, err))
			continue
		}
	}
	return allErrs.ErrorOrNil()
}

func (c *NodeGroupController) tryDeleteNodes(nodesToDeleteInfo []nodeToDeleteInfo, nodeGroup *NodeGroupGroupOptions) error {
	if len(nodesToDeleteInfo) == 0 {
		log.DebugLn("No nodes to delete")
		return nil
	}

	if c.changeSettings.AutoDismissDestructive {
		log.DebugLn("Skip delete nodes because destructive operations are disabled")
		return nil
	}

	if c.name == MasterNodeGroupName {
		if nodeGroup.DesiredReplicas < 1 {
			return fmt.Errorf(`Cannot delete ALL master nodes. If you want to remove cluster use 'dhctl destroy' command`)
		}

		needToQuorum := nodeGroup.CurReplicas()/2 + 1

		noQuorum := nodeGroup.DesiredReplicas < needToQuorum
		msg := fmt.Sprintf("Desired master replicas count (%d) can break cluster. Need minimum replicas (%d). Do you want to continue?", nodeGroup.DesiredReplicas, needToQuorum)
		confirm := input.NewConfirmation().WithMessage(msg)
		if noQuorum && !confirm.Ask() {
			return fmt.Errorf("Skip delete master nodes")
		}
	}

	title := fmt.Sprintf("Delete Nodes from NodeGroup %s (replicas: %v)", c.name, nodeGroup.DesiredReplicas)
	return log.Process("converge", title, func() error {
		return c.deleteRedundantNodes(nodeGroup, c.state.Settings, nodesToDeleteInfo)
	})
}

func (c *NodeGroupController) tryUpdateNodeTemplate(nodeGroup *NodeGroupGroupOptions, nodeTemplate map[string]interface{}) error {
	nodeTemplatePath := []string{"spec", "nodeTemplate"}
	for {
		ng, err := GetNodeGroup(c.client, c.name)
		if err != nil {
			return err
		}

		templateInCluster, _, err := unstructured.NestedMap(ng.Object, nodeTemplatePath...)
		if err != nil {
			return err
		}

		diff := gcmp.Diff(templateInCluster, nodeTemplate)
		if diff == "" {
			log.DebugF("Node template of the %s NodeGroup is not changed", c.name)
			return nil
		}

		msg := fmt.Sprintf("Node template diff:\n\n%s\n", diff)

		if !c.changeSettings.AutoApprove && !input.NewConfirmation().WithMessage(msg).Ask() {
			log.InfoLn("Updating node group template was skipped")
			return nil
		}

		err = unstructured.SetNestedMap(ng.Object, nodeTemplate, nodeTemplatePath...)
		if err != nil {
			return err
		}

		err = UpdateNodeGroup(c.client, nodeGroup.Name, ng)

		if err == nil {
			return nil
		}

		if errors.Is(err, ErrNodeGroupChanged) {
			log.WarnLn(err.Error())
			continue
		}

		return err
	}
}

func (c *NodeGroupController) tryDeleteNodeGroup(nodeGroup *NodeGroupGroupOptions) error {
	if c.changeSettings.AutoDismissDestructive {
		log.DebugF("Skip delete %s node group because destructive operations are disabled\n", c.name)
		return nil
	}

	if nodeGroup.Name == MasterNodeGroupName {
		log.DebugLn("Skip delete master node group")
		return nil
	}

	return log.Process("converge", fmt.Sprintf("Delete NodeGroup %s", c.name), func() error {
		return DeleteNodeGroup(c.client, nodeGroup.Name)
	})
}

func (c *NodeGroupController) getSpec(name string) *config.TerraNodeGroupSpec {
	for _, terranodeGroup := range c.config.GetTerraNodeGroups() {
		if terranodeGroup.Name == name {
			cc := terranodeGroup
			return &cc
		}
	}

	return nil
}

func (c *NodeGroupController) updateNodes(nodeGroup *NodeGroupGroupOptions) error {
	replicas := nodeGroup.DesiredReplicas
	if replicas == 0 {
		return nil
	}

	var allErrs *multierror.Error

	if err := c.populateNodeToHost(); err != nil {
		return err
	}

	nodeNames, err := sortNodeNames(nodeGroup)
	if err != nil {
		return err
	}

	for _, nodeName := range nodeNames {
		processTitle := fmt.Sprintf("Update Node %s in NodeGroup %s (replicas: %v)", nodeName, c.name, replicas)

		err := log.Process("converge", processTitle, func() error {
			return c.updateNode(nodeGroup, nodeName)
		})

		if err != nil {
			// We do not return an error immediately for the following reasons:
			// - some nodes cannot be converged for some reason, but other nodes must be converged
			// - after making a plan, before converging a node, we get confirmation from user for start converge
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", nodeName, err))
		}
	}

	return allErrs.ErrorOrNil()
}

func (c *NodeGroupController) getNodesToDeleteInfo(nodeGroup *NodeGroupGroupOptions) ([]nodeToDeleteInfo, error) {
	var nodesToDeleteInfo []nodeToDeleteInfo

	if nodeGroup.DesiredReplicas < len(nodeGroup.State) {
		count := len(nodeGroup.State)

		nodeNames, err := sortNodeNames(nodeGroup)
		if err != nil {
			return nil, err
		}

		for _, nodeName := range nodeNames {
			nodesToDeleteInfo = append(nodesToDeleteInfo, nodeToDeleteInfo{
				name:  nodeName,
				state: nodeGroup.State[nodeName],
			})
			delete(nodeGroup.State, nodeName)
			count--

			if count == nodeGroup.DesiredReplicas {
				break
			}
		}
	}

	return nodesToDeleteInfo, nil
}

type nodeToDeleteInfo struct {
	name  string
	state []byte
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

// sortNodeNames sorts node names in descending order
func sortNodeNames(nodeGroup *NodeGroupGroupOptions) ([]string, error) {
	index := make([]nodeNameWithIndex, 0, len(nodeGroup.State))

	for nodeName := range nodeGroup.State {
		nodeIndex, err := config.GetIndexFromNodeName(nodeName)
		if err != nil {
			return nil, err
		}

		index = append(index, nodeNameWithIndex{name: nodeName, index: nodeIndex})
	}

	// Descending order to delete nodes with bigger numbers first
	// Need to use index instead of a name to prevent string sorting and decimals problem
	slices.SortFunc(index, func(i, j nodeNameWithIndex) int {
		return cmp.Compare(j.index, i.index)
	})

	nodeNames := make([]string, len(index))

	for i, nodeName := range index {
		nodeNames[i] = nodeName.name
	}

	return nodeNames, nil
}

type nodeNameWithIndex struct {
	name  string
	index int
}
