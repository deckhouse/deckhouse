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
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	gcmp "github.com/google/go-cmp/cmp"
	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	state_terraform "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type NodeGroupController struct {
	client         *client.KubernetesClient
	config         *config.MetaConfig
	changeSettings *terraform.ChangeActionSettings

	excludedNodes map[string]bool

	stateCache dstate.Cache

	name  string
	state state.NodeGroupTerraformState

	commanderMode    bool
	terraformContext *terraform.TerraformContext

	nodeGroup nodeGroupController

	cloudConfig     string
	desiredReplicas int
	layoutStep      string
}

func NewNodeGroupController(
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	name string,
	state state.NodeGroupTerraformState,
	stateCache dstate.Cache,
	terraformContext *terraform.TerraformContext,
	commanderMode bool,
	changeSettings *terraform.ChangeActionSettings,
	nodesMap map[string]bool) *NodeGroupController {
	controller := &NodeGroupController{
		client:           kubeCl,
		config:           metaConfig,
		changeSettings:   changeSettings,
		excludedNodes:    nodesMap,
		stateCache:       stateCache,
		terraformContext: terraformContext,
		commanderMode:    commanderMode,

		name:  name,
		state: state,
	}

	return controller
}

func (c *NodeGroupController) Run() error {
	// we hide deckhouse logs because we always have config
	nodeCloudConfig, err := GetCloudConfig(c.client, c.name, HideDeckhouseLogs)
	if err != nil {
		return err
	}

	c.cloudConfig = nodeCloudConfig

	if c.desiredReplicas > len(c.state.State) {
		err := log.Process("converge", fmt.Sprintf("Add Nodes to NodeGroup %s (replicas: %v)", c.name, c.desiredReplicas), func() error {
			return c.nodeGroup.addNodes()
		})
		if err != nil {
			return err
		}
	}

	nodesToDeleteInfo, err := getNodesToDeleteInfo(c.desiredReplicas, c.state.State)
	if err != nil {
		return err
	}

	err = c.updateNodes()
	if err != nil {
		return err
	}

	err = c.tryDeleteNodes(nodesToDeleteInfo)
	if err != nil {
		return err
	}

	groupSpec := c.getSpec(c.name)
	if groupSpec != nil {
		return c.tryUpdateNodeTemplate(groupSpec.NodeTemplate)
	}

	return c.tryDeleteNodeGroup()
}

func (c *NodeGroupController) tryDeleteNodes(nodesToDeleteInfo []nodeToDeleteInfo) error {
	if len(nodesToDeleteInfo) == 0 {
		log.DebugLn("No nodes to delete")
		return nil
	}

	if c.changeSettings.AutoDismissDestructive {
		log.DebugLn("Skip delete nodes because destructive operations are disabled")
		return nil
	}

	return c.nodeGroup.deleteNodes(nodesToDeleteInfo)
}

func (c *NodeGroupController) deleteRedundantNodes(
	settings []byte,
	nodesToDeleteInfo []nodeToDeleteInfo,
	getHookByNodeName func(nodeName string) terraform.InfraActionHook,
) error {
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

		nodeRunner := c.terraformContext.GetConvergeNodeDeleteRunner(cfg, terraform.NodeDeleteRunnerOptions{
			AutoDismissDestructive: c.changeSettings.AutoDismissDestructive,
			AutoApprove:            c.changeSettings.AutoApprove,
			NodeName:               nodeToDeleteInfo.name,
			NodeGroupName:          c.name,
			LayoutStep:             c.layoutStep,
			NodeIndex:              nodeIndex,
			NodeState:              nodeState,
			NodeCloudConfig:        c.cloudConfig,
			CommanderMode:          c.commanderMode,
			StateCache:             c.stateCache,
			AdditionalStateSaverDestinations: []terraform.SaverDestination{
				NewNodeStateSaver(c.client, nodeToDeleteInfo.name, c.name, nil),
			},
			Hook: getHookByNodeName(nodeToDeleteInfo.name),
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

func (c *NodeGroupController) tryUpdateNodeTemplate(nodeTemplate map[string]interface{}) error {
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

		err = UpdateNodeGroup(c.client, c.name, ng)

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

func (c *NodeGroupController) tryDeleteNodeGroup() error {
	if c.changeSettings.AutoDismissDestructive {
		log.DebugF("Skip delete %s node group because destructive operations are disabled\n", c.name)
		return nil
	}

	if c.name == MasterNodeGroupName {
		log.DebugF("Skip delete %s node group because it is master\n", c.name)
		return nil
	}

	return log.Process("converge", fmt.Sprintf("Delete NodeGroup %s", c.name), func() error {
		return DeleteNodeGroup(c.client, c.name)
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

func (c *NodeGroupController) updateNodes() error {
	replicas := c.desiredReplicas
	if replicas == 0 {
		return nil
	}

	var allErrs *multierror.Error

	nodeNames, err := sortNodeNames(c.state.State)
	if err != nil {
		return err
	}

	for _, nodeName := range nodeNames {
		processTitle := fmt.Sprintf("Update Node %s in NodeGroup %s (replicas: %v)", nodeName, c.name, replicas)

		err := log.Process("converge", processTitle, func() error {
			if _, ok := c.excludedNodes[nodeName]; ok {
				log.InfoF("Skip update excluded node %v\n", nodeName)
				return nil
			}

			err = c.nodeGroup.updateNode(nodeName)
			if err != nil {
				return err
			}

			// we hide deckhouse logs because we always have config
			nodeCloudConfig, err := GetCloudConfig(c.client, c.name, HideDeckhouseLogs)
			if err != nil {
				return err
			}

			c.cloudConfig = nodeCloudConfig

			return nil
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

func getNodesToDeleteInfo(desiredReplicas int, state map[string][]byte) ([]nodeToDeleteInfo, error) {
	if desiredReplicas >= len(state) {
		return nil, nil
	}

	var nodesToDeleteInfo []nodeToDeleteInfo

	count := len(state)

	nodeNames, err := sortNodeNames(state)
	if err != nil {
		return nil, err
	}

	for _, nodeName := range nodeNames {
		nodesToDeleteInfo = append(nodesToDeleteInfo, nodeToDeleteInfo{
			name:  nodeName,
			state: state[nodeName],
		})
		delete(state, nodeName)
		count--

		if count == desiredReplicas {
			break
		}
	}

	return nodesToDeleteInfo, nil
}

type nodeToDeleteInfo struct {
	name  string
	state []byte
}

// sortNodeNames sorts node names in descending order
func sortNodeNames(state map[string][]byte) ([]string, error) {
	index := make([]nodeNameWithIndex, 0, len(state))

	for nodeName := range state {
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

type nodeGroupController interface {
	addNodes() error
	updateNode(name string) error
	deleteNodes(nodesToDeleteInfo []nodeToDeleteInfo) error
}
