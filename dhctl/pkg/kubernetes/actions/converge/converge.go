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
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infra/hook/controlplane"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
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

type Phase string

const (
	PhaseBaseInfra = Phase("base-infrastructure")
	PhaseAllNodes  = Phase("all-nodes")
)

var (
	ErrConvergeInterrupted = errors.New("Interrupted.")
)

type Runner struct {
	kubeCl         *client.KubernetesClient
	changeSettings *terraform.ChangeActionSettings
	lockRunner     *InLockRunner

	excludedNodes map[string]bool
	skipPhases    map[Phase]bool

	stateCache dstate.Cache
}

func NewRunner(kubeCl *client.KubernetesClient, lockRunner *InLockRunner) *Runner {
	return &Runner{
		kubeCl:         kubeCl,
		changeSettings: &terraform.ChangeActionSettings{},
		lockRunner:     lockRunner,

		excludedNodes: make(map[string]bool),
		skipPhases:    make(map[Phase]bool),
		stateCache:    cache.Global(),
	}
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
	return r.lockRunner.Run(r.converge)
}

func (r *Runner) converge() error {
	metaConfig, err := GetMetaConfig(r.kubeCl)
	if err != nil {
		return err
	}

	if !r.isSkip(PhaseBaseInfra) {
		if err := r.updateClusterState(metaConfig); err != nil {
			return err
		}
	} else {
		log.InfoLn("Skip converge base infrastructure")
	}

	if r.isSkip(PhaseAllNodes) {
		log.InfoLn("Skip converge nodes")
		return nil
	}

	var nodesState map[string]NodeGroupTerraformState

	err = log.Process("converge", "Gather Nodes Terraform state", func() error {
		nodesState, err = GetNodesStateFromCluster(r.kubeCl)
		if err != nil {
			return fmt.Errorf("terraform nodes state in Kubernetes cluster not found: %w", err)
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
		controller := NewConvergeController(r.kubeCl, metaConfig, nodeGroupName, ngState, r.stateCache)
		controller.WithChangeSettings(r.changeSettings)
		controller.WithExcludedNodes(r.excludedNodes)

		if err := controller.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) updateClusterState(metaConfig *config.MetaConfig) error {
	return log.Process("converge", "Update Cluster Terraform state", func() error {
		clusterState, err := GetClusterStateFromCluster(r.kubeCl)
		if err != nil {
			return fmt.Errorf("terraform cluster state in Kubernetes cluster not found: %w", err)
		}

		if clusterState == nil {
			return fmt.Errorf("kubernetes cluster has no state")
		}

		baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure", r.stateCache).
			WithVariables(metaConfig.MarshalConfig()).
			WithState(clusterState).
			WithAutoDismissDestructiveChanges(r.changeSettings.AutoDismissDestructive).
			WithAutoApprove(r.changeSettings.AutoApprove)
		tomb.RegisterOnShutdown("base-infrastructure", baseRunner.Stop)

		baseRunner.WithAdditionalStateSaverDestination(NewClusterStateSaver(r.kubeCl))

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

		nodeCloudConfig, err := GetCloudConfig(r.kubeCl, group.Name)
		if err != nil {
			return err
		}

		for i := 0; i < group.Replicas; i++ {
			err = BootstrapAdditionalNode(r.kubeCl, metaConfig, i, "static-node", group.Name, nodeCloudConfig, true)
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

	nodeExternalIPs map[string]string
	name            string
	state           NodeGroupTerraformState
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

func NewConvergeController(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, name string, state NodeGroupTerraformState, stateCache dstate.Cache) *NodeGroupController {
	return &NodeGroupController{
		client:         kubeCl,
		config:         metaConfig,
		changeSettings: &terraform.ChangeActionSettings{},
		excludedNodes:  make(map[string]bool),
		stateCache:     stateCache,

		name:  name,
		state: state,
	}
}

func (c *NodeGroupController) WithChangeSettings(changeSettings *terraform.ChangeActionSettings) *NodeGroupController {
	c.changeSettings = changeSettings
	return c
}

func (c *NodeGroupController) WithExcludedNodes(nodesMap map[string]bool) *NodeGroupController {
	c.excludedNodes = nodesMap
	return c
}

func (c *NodeGroupController) getNodeGroupReadinessChecker(nodeGroup *NodeGroupGroupOptions, convergedNode string) (terraform.InfraActionHook, error) {
	if c.name != MasterNodeGroupName {
		// for not master node groups do not need readiness check
		return &terraform.DummyHook{}, nil
	}

	// single master do no need readiness check
	// it doesn't make sense
	// but single master can converge for updating
	if nodeGroup.CurReplicas() == 1 {
		return &terraform.DummyHook{}, nil
	}

	if c.nodeExternalIPs == nil {
		ips := make(map[string]string)
		for nodeName, st := range nodeGroup.State {
			r := terraform.NewRunnerFromConfig(c.config, "get-master-ip", c.stateCache).
				WithState(st)

			out, err := terraform.GetMasterNodeResult(r)
			if err != nil {
				return nil, err
			}

			ips[nodeName] = out.MasterIPForSSH
		}

		c.nodeExternalIPs = ips
	}

	if c.client.SSHClient != nil {
		userPassedHosts := c.client.SSHClient.Settings.AvailableHosts()
		setFromState, err := ssh.CheckSSHHosts(userPassedHosts, c.nodeExternalIPs, func(msg string) bool {
			return input.NewConfirmation().WithMessage(msg).Ask()
		})

		if err != nil {
			return nil, err
		}

		if setFromState {
			hostnames := maputil.Values(c.nodeExternalIPs)
			foundCurrentHostInNew := c.client.SSHClient.Settings.ReplaceAvailableHosts(hostnames)
			if !foundCurrentHostInNew {
				// we can not find current if we want to delete master
				// in case, when we already connect to node for delete, we should reconnect to another node
				log.InfoF("Need to restart kube proxy with new host %s ...\n", c.client.SSHClient.Settings.Host())
				err := c.client.KubeProxy.Restart()
				if err != nil {
					return nil, err
				}

				log.InfoLn("Proxy was restarted")
			}
		}
	}

	nodesToCheck := maputil.ExcludeKeys(c.nodeExternalIPs, convergedNode)

	h := controlplane.NewHook(c.client, nodesToCheck, c.config.UUID).
		WithSourceCommandName("converge").
		WithNodeToConverge(convergedNode)

	return h, nil
}

func (c *NodeGroupController) Run() error {
	nodeGroupName := c.name

	replicas := getReplicasByNodeGroupName(c.config, nodeGroupName)
	step := getStepByNodeGroupName(nodeGroupName)

	nodeCloudConfig, err := GetCloudConfig(c.client, nodeGroupName)
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

	deleteNodesNames := c.getNodesToDelete(nodeGroup)

	err = c.updateNodes(nodeGroup)
	if err != nil {
		return err
	}

	err = c.tryDeleteNodes(deleteNodesNames, nodeGroup)
	if err != nil {
		return err
	}

	return c.tryDeleteNodeGroup(nodeGroup)
}

func (c *NodeGroupController) addNewNodesToGroup(nodeGroup *NodeGroupGroupOptions) error {
	count := len(nodeGroup.State)
	index := 0

	var nodesToWait []string

	for nodeGroup.DesiredReplicas > count {
		candidateName := fmt.Sprintf("%s-%s-%v", c.config.ClusterPrefix, nodeGroup.Name, index)

		if _, ok := nodeGroup.State[candidateName]; !ok {
			var err error
			if nodeGroup.Name == MasterNodeGroupName {
				_, err = BootstrapAdditionalMasterNode(c.client, c.config, index, nodeGroup.CloudConfig, true)
			} else {
				err = BootstrapAdditionalNode(c.client, c.config, index, nodeGroup.Step, nodeGroup.Name, nodeGroup.CloudConfig, true)
			}
			if err != nil {
				return err
			}
			count++
			nodesToWait = append(nodesToWait, candidateName)
		}
		index++
	}

	return WaitForNodesListBecomeReady(c.client, nodesToWait)
}

func (c *NodeGroupController) updateNode(nodeGroup *NodeGroupGroupOptions, nodeName string) error {
	if _, ok := c.excludedNodes[nodeName]; ok {
		log.InfoF("Skip update excluded node %v\n", nodeName)
		return nil
	}

	state := nodeGroup.State[nodeName]
	index, ok := getIndexFromNodeName(nodeName)
	if !ok {
		log.ErrorF("can't extract index from terraform state secret, skip %s\n", nodeName)
		return nil
	}

	checker, err := c.getNodeGroupReadinessChecker(nodeGroup, nodeName)
	if err != nil {
		return err
	}

	nodeRunner := terraform.NewRunnerFromConfig(c.config, nodeGroup.Step, c.stateCache).
		WithVariables(c.config.NodeGroupConfig(nodeGroup.Name, int(index), nodeGroup.CloudConfig)).
		WithSkipChangesOnDeny(true).
		WithState(state).
		WithName(nodeName).
		WithAutoDismissDestructiveChanges(c.changeSettings.AutoDismissDestructive).
		WithAutoApprove(c.changeSettings.AutoApprove).
		WithHook(checker)

	tomb.RegisterOnShutdown(nodeName, nodeRunner.Stop)

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

	nodeRunner.WithAdditionalStateSaverDestination(NewNodeStateSaver(c.client, nodeName, nodeGroupName, nodeGroupSettingsFromConfig))

	outputs, err := terraform.ApplyPipeline(nodeRunner, nodeName, extractOutputFunc)
	if err != nil {
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

func (c *NodeGroupController) deleteRedundantNodes(nodeGroup *NodeGroupGroupOptions, settings []byte, deleteNodesNames map[string][]byte) error {
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
	for name, state := range deleteNodesNames {
		if _, ok := c.excludedNodes[name]; ok {
			log.InfoF("Skip delete excluded node %v\n", name)
			continue
		}

		index, ok := getIndexFromNodeName(name)
		if !ok {
			log.ErrorF("can't extract index from terraform state secret, skip %s\n", name)
			continue
		}

		nodeRunner := terraform.NewRunnerFromConfig(c.config, nodeGroup.Step, c.stateCache).
			WithVariables(cfg.NodeGroupConfig(nodeGroup.Name, int(index), nodeGroup.CloudConfig)).
			WithState(state).
			WithName(name).
			WithAllowedCachedState(true).
			WithSkipChangesOnDeny(true).
			WithAutoDismissDestructiveChanges(c.changeSettings.AutoDismissDestructive)

		tomb.RegisterOnShutdown(name, nodeRunner.Stop)

		nodeRunner.WithAdditionalStateSaverDestination(NewNodeStateSaver(c.client, name, nodeGroup.Name, nil))

		if err := terraform.DestroyPipeline(nodeRunner, name); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", name, err))
			continue
		}

		if tomb.IsInterrupted() {
			allErrs = multierror.Append(allErrs, ErrConvergeInterrupted)
			return allErrs.ErrorOrNil()
		}

		if err := DeleteNode(c.client, name); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", name, err))
			continue
		}

		if err := DeleteTerraformState(c.client, fmt.Sprintf("d8-node-terraform-state-%s", name)); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", name, err))
			continue
		}
	}
	return allErrs.ErrorOrNil()
}

func (c *NodeGroupController) tryDeleteNodes(deleteNodesNames map[string][]byte, nodeGroup *NodeGroupGroupOptions) error {
	if len(deleteNodesNames) == 0 {
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
		return c.deleteRedundantNodes(nodeGroup, c.state.Settings, deleteNodesNames)
	})
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

	groupInConfig := false

	for _, terranodeGroup := range c.config.GetTerraNodeGroups() {
		if terranodeGroup.Name == c.name {
			groupInConfig = true
			break
		}
	}

	if groupInConfig {
		log.DebugF("Do not delete %s node group, because it present in config\n")
		return nil
	}

	return log.Process("converge", fmt.Sprintf("Delete NodeGroup %s", c.name), func() error {
		return DeleteNodeGroup(c.client, nodeGroup.Name)
	})
}

func (c *NodeGroupController) updateNodes(nodeGroup *NodeGroupGroupOptions) error {
	replicas := nodeGroup.DesiredReplicas
	if replicas == 0 {
		return nil
	}

	var allErrs *multierror.Error

	for nodeName := range nodeGroup.State {
		processTitle := fmt.Sprintf("Update Node %s in NodeGroup %s (replicas: %v)", nodeName, c.name, replicas)

		err := log.Process("converge", processTitle, func() error {
			return c.updateNode(nodeGroup, nodeName)
		})

		if err != nil {
			if errors.Is(err, ssh.ErrNotEnoughMastersSSHHosts) {
				return err
			}

			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %v", nodeName, err))
		}
	}

	return allErrs.ErrorOrNil()
}

func (c *NodeGroupController) getNodesToDelete(nodeGroup *NodeGroupGroupOptions) map[string][]byte {
	deleteNodesNames := make(map[string][]byte)

	if nodeGroup.DesiredReplicas < len(nodeGroup.State) {
		count := len(nodeGroup.State)

		// Descending order to delete nodes with bigger numbers first
		// Need to use index instead of a name to prevent string sorting and decimals problem
		keys := make([]string, 0, len(nodeGroup.State))
		for k := range nodeGroup.State {
			keys = append(keys, k)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))

		for _, name := range keys {
			state := nodeGroup.State[name]

			deleteNodesNames[name] = state
			delete(nodeGroup.State, name)
			count--

			if count == nodeGroup.DesiredReplicas {
				break
			}
		}
	}

	return deleteNodesNames
}

func GetMetaConfig(kubeCl *client.KubernetesClient) (*config.MetaConfig, error) {
	metaConfig, err := config.ParseConfigFromCluster(kubeCl)
	if err != nil {
		return nil, err
	}

	metaConfig.UUID, err = GetClusterUUID(kubeCl)
	if err != nil {
		return nil, err
	}

	return metaConfig, nil
}
