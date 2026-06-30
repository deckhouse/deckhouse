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

package controller

import (
	"cmp"
	gocontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	gcmp "github.com/google/go-cmp/cmp"
	"github.com/hashicorp/go-multierror"
	"github.com/name212/govalue"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

type NodeGroupController struct {
	excludedNodes map[string]bool

	name  string
	state state.NodeGroupInfrastructureState

	nodeGroup nodeGroupController

	cloudConfig     string
	desiredReplicas int
	layoutStep      infrastructure.Step

	globalOptions *options.GlobalOptions
}

func NewNodeGroupController(name string, state state.NodeGroupInfrastructureState, excludeNodes map[string]bool, globalOptions *options.GlobalOptions) *NodeGroupController {
	controller := &NodeGroupController{
		excludedNodes: excludeNodes,
		name:          name,
		state:         state,
		globalOptions: globalOptions,
	}

	return controller
}

func (c *NodeGroupController) Run(ctx *context.Context) error {
	// we hide deckhouse logs because we always have config
	kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return fmt.Errorf("Could not get kube client: %w", err)
	}

	nodeCloudConfig, err := entity.GetCloudConfig(ctx.Ctx(), kubeClient, c.name, global.HideDeckhouseLogs)
	if err != nil {
		return err
	}

	c.cloudConfig = nodeCloudConfig

	if c.desiredReplicas > len(c.state.State) {
		err := dhlog.RunProcess(ctx.Ctx(), dhlog.FromContext(ctx.Ctx()), fmt.Sprintf("Add Nodes to NodeGroup %s (replicas: %v)", c.name, c.desiredReplicas), func(gocontext.Context) error {
			return c.nodeGroup.addNodes(ctx)
		})
		if err != nil {
			return err
		}
	}

	nodesToDeleteInfo, err := getNodesToDeleteInfo(ctx.Ctx(), c.desiredReplicas, c.state.State)
	if err != nil {
		return err
	}

	dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Nodes to delete: %d. Starting to update nodes", len(nodesToDeleteInfo)))

	if err := c.nodeGroup.beforeUpdateNodes(ctx); err != nil {
		return err
	}

	err = c.updateNodes(ctx)
	if err != nil {
		return err
	}

	dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "starting to delete nodes")

	if err := c.switchClientBeforeDeleteNodesIfNeed(ctx, nodesToDeleteInfo); err != nil {
		return err
	}

	err = c.tryDeleteNodes(ctx, nodesToDeleteInfo)
	if err != nil {
		return err
	}

	groupSpec, err := c.getSpec(ctx, c.name)
	if err != nil {
		return err
	}

	dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "Starting to converge node template")

	if groupSpec != nil {
		return c.tryUpdateNodeTemplate(ctx, groupSpec.NodeTemplate)
	}

	return c.tryDeleteNodeGroup(ctx)
}

func (c *NodeGroupController) switchClientBeforeDeleteNodesIfNeed(ctx *context.Context, nodesToDeleteInfo []nodeToDeleteInfo) error {
	clientSwitcher := ctx.ClientSwitcher()
	if govalue.IsNil(clientSwitcher) {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "Skipping switch of client before deleting nodes. Got empty switcher")
		return nil
	}

	nodesStates := make([]*context.NodeState, 0, len(nodesToDeleteInfo))
	for _, dn := range nodesToDeleteInfo {
		nodesStates = append(nodesStates, &context.NodeState{
			Name:  dn.name,
			State: dn.state,
		})
	}

	// all checks to skip switching realised in method
	return clientSwitcher.SwitchWhenDecreaseMastersIfNeed(ctx.Ctx(), c.name, nodesStates)
}

func (c *NodeGroupController) tryDeleteNodes(ctx *context.Context, nodesToDeleteInfo []nodeToDeleteInfo) error {
	if len(nodesToDeleteInfo) == 0 {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "No nodes to delete")
		return nil
	}

	if ctx.ChangesSettings().AutoDismissDestructive {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "Skipping node deletion because destructive operations are disabled")
		return nil
	}

	return c.nodeGroup.deleteNodes(ctx, nodesToDeleteInfo)
}

func (c *NodeGroupController) deleteRedundantNodes(
	ctx *context.Context,
	settings []byte,
	nodesToDeleteInfo []nodeToDeleteInfo,
	getHookByNodeName func(nodeName string) infrastructure.InfraActionHook,
) error {
	cfg, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	if settings != nil {
		nodeGroupsSettings, err := json.Marshal([]json.RawMessage{settings})
		if err != nil {
			dhlog.FromContext(ctx.Ctx()).ErrorContext(ctx.Ctx(), fmt.Sprint(err))
		} else {
			mc, err := ctx.MetaConfig()
			if err != nil {
				return err
			}
			// we use dummy preparator because metaConfig was prepared early
			cfg, err = mc.DeepCopy().Prepare(ctx.Ctx(), config.DummyPreparatorProvider())
			if err != nil {
				return fmt.Errorf("unable to prepare copied config: %v", err)
			}
			cfg.ProviderClusterConfig["nodeGroups"] = nodeGroupsSettings
		}
	}

	kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return fmt.Errorf("Could not get kube client: %w", err)
	}

	var allErrs *multierror.Error
	for _, nodeToDeleteInfo := range nodesToDeleteInfo {
		if _, ok := c.excludedNodes[nodeToDeleteInfo.name]; ok {
			dhlog.FromContext(ctx.Ctx()).InfoContext(ctx.Ctx(), fmt.Sprintf("Skipping deletion of excluded node %v", nodeToDeleteInfo.name))
			continue
		}

		nodeIndex, err := config.GetIndexFromNodeName(nodeToDeleteInfo.name)
		if err != nil {
			dhlog.FromContext(ctx.Ctx()).ErrorContext(ctx.Ctx(), fmt.Sprintf("can't extract index from infrastructure state secret (%v), skipping %s", err, nodeToDeleteInfo.name))
			return nil
		}

		// NOTE: In the commander mode nodes state should exist in the local state cache, no need to pass state explicitly.
		var nodeState []byte
		if !ctx.CommanderMode() {
			nodeState = nodeToDeleteInfo.state
		}

		nodeRunner, err := ctx.InfrastructureContext(cfg).GetConvergeNodeDeleteRunner(ctx.Ctx(), cfg, infrastructure.NodeDeleteRunnerOptions{
			NodeName:        nodeToDeleteInfo.name,
			NodeGroupName:   c.name,
			LayoutStep:      c.layoutStep,
			NodeIndex:       nodeIndex,
			NodeState:       nodeState,
			NodeCloudConfig: c.cloudConfig,
			CommanderMode:   ctx.CommanderMode(),
			StateCache:      ctx.StateCache(),
			AdditionalStateSaverDestinations: []infrastructure.SaverDestination{
				infrastructurestate.NewNodeStateSaver(ctx, nodeToDeleteInfo.name, c.name, nil),
			},
			Hook: getHookByNodeName(nodeToDeleteInfo.name),
		}, ctx.ChangesSettings().AutomaticSettings)
		if err != nil {
			return err
		}

		if err := infrastructure.DestroyPipeline(ctx.Ctx(), nodeRunner, nodeToDeleteInfo.name); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", nodeToDeleteInfo.name, err))
			continue
		}

		if tomb.IsInterrupted() {
			allErrs = multierror.Append(allErrs, global.ErrConvergeInterrupted)
			return allErrs.ErrorOrNil()
		}

		if err := entity.DeleteNode(ctx.Ctx(), kubeClient, nodeToDeleteInfo.name); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", nodeToDeleteInfo.name, err))
			continue
		}

		if err := infrastructurestate.DeleteNodeInfrastructureStateFromCache(ctx.Ctx(), nodeToDeleteInfo.name, ctx.StateCache()); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("unable to delete node %s infrastructure state from cache: %w", nodeToDeleteInfo.name, err))
			continue
		}

		if err := infrastructurestate.DeleteInfrastructureState(ctx.Ctx(), kubeClient, fmt.Sprintf("d8-node-terraform-state-%s", nodeToDeleteInfo.name)); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", nodeToDeleteInfo.name, err))
			continue
		}
	}

	return allErrs.ErrorOrNil()
}

func getNodeTemplateDiff(ctx gocontext.Context, fromNG, fromConfig map[string]any) string {
	// prevent compare nil and empty map
	// this case generates diff for gcmp.Diff
	if len(fromNG) == 0 && len(fromConfig) == 0 {
		dhlog.FromContext(ctx).DebugContext(ctx, "Node templates have no keys. Returning no diff")
		return ""
	}

	return gcmp.Diff(fromNG, fromConfig)
}

func (c *NodeGroupController) tryUpdateNodeTemplate(ctx *context.Context, nodeTemplate map[string]any) error {
	nodeTemplatePath := []string{"spec", "nodeTemplate"}
	kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return fmt.Errorf("Could not get kube client: %w", err)
	}

	for {
		ng, err := entity.GetNodeGroup(ctx.Ctx(), kubeClient, c.name)
		if err != nil {
			return err
		}

		templateInCluster, _, err := unstructured.NestedMap(ng.Object, nodeTemplatePath...)
		if err != nil {
			return err
		}

		diff := getNodeTemplateDiff(ctx.Ctx(), templateInCluster, nodeTemplate)
		if diff == "" {
			dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), strings.TrimRight(fmt.Sprintf("Node template of the %s NodeGroup has not changed", c.name), "\n"))
			return nil
		}

		msg := fmt.Sprintf("Node template diff:\n\n%s\n", diff)

		if !ctx.ChangesSettings().AutoApprove && !input.NewConfirmation().WithMessage(msg).Ask() {
			dhlog.FromContext(ctx.Ctx()).InfoContext(ctx.Ctx(), "Updating node group template was skipped")
			return nil
		}

		err = unstructured.SetNestedMap(ng.Object, nodeTemplate, nodeTemplatePath...)
		if err != nil {
			return err
		}

		err = entity.UpdateNodeGroup(ctx.Ctx(), kubeClient, c.name, ng)

		if err == nil {
			return nil
		}

		if errors.Is(err, global.ErrNodeGroupChanged) {
			dhlog.FromContext(ctx.Ctx()).WarnContext(ctx.Ctx(), fmt.Sprint(err.Error()))
			continue
		}

		return err
	}
}

func (c *NodeGroupController) tryDeleteNodeGroup(ctx *context.Context) error {
	if ctx.ChangesSettings().AutoDismissDestructive {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Skipping deletion of %s node group because destructive operations are disabled", c.name))
		return nil
	}

	if c.name == global.MasterNodeGroupName {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Skipping deletion of %s node group because it is master", c.name))
		return nil
	}

	kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return fmt.Errorf("Could not get kube client: %w", err)
	}

	return dhlog.RunProcess(ctx.Ctx(), dhlog.FromContext(ctx.Ctx()), fmt.Sprintf("Delete NodeGroup %s", c.name), func(gocontext.Context) error {
		return entity.DeleteNodeGroup(ctx.Ctx(), kubeClient, c.name)
	})
}

func (c *NodeGroupController) getSpec(ctx *context.Context, name string) (*config.TerraNodeGroupSpec, error) {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return nil, err
	}
	for _, terranodeGroup := range metaConfig.GetTerraNodeGroups() {
		if terranodeGroup.Name == name {
			return new(terranodeGroup), nil
		}
	}

	return nil, nil
}

func (c *NodeGroupController) updateNodes(ctx *context.Context) error {
	replicas := c.desiredReplicas
	if replicas == 0 {
		return nil
	}

	var allErrs *multierror.Error

	nodeNames, err := sortNodeNames(c.state.State)
	if err != nil {
		return err
	}

	kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return fmt.Errorf("Could not get kube client: %w", err)
	}

	for _, nodeName := range nodeNames {
		processTitle := fmt.Sprintf("Update Node %s in NodeGroup %s (replicas: %v)", nodeName, c.name, replicas)

		err := dhlog.RunProcess(ctx.Ctx(), dhlog.FromContext(ctx.Ctx()), processTitle, func(gocontext.Context) error {
			if _, ok := c.excludedNodes[nodeName]; ok {
				dhlog.FromContext(ctx.Ctx()).InfoContext(ctx.Ctx(), fmt.Sprintf("Skipping update of excluded node %v", nodeName))
				return nil
			}

			err = c.nodeGroup.updateNode(ctx, nodeName)
			if err != nil {
				return err
			}

			// we hide deckhouse logs because we always have config
			nodeCloudConfig, err := entity.GetCloudConfig(ctx.Ctx(), kubeClient, c.name, global.HideDeckhouseLogs)
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

			// in commander mode returns immediately, because we can break all master nodes
			if ctx.CommanderMode() {
				break
			}
		}
	}

	return allErrs.ErrorOrNil()
}

func getNodesToDeleteInfo(ctx gocontext.Context, desiredReplicas int, state map[string][]byte) ([]nodeToDeleteInfo, error) {
	if desiredReplicas >= len(state) {
		dhlog.FromContext(ctx).DebugContext(ctx, "desired replicas >= replicas in state. skipping nodes info")
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
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("stopping collection of nodes-to-delete info. count %v", count))
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
	addNodes(ctx *context.Context) error
	beforeUpdateNodes(ctx *context.Context) error
	updateNode(ctx *context.Context, name string) error
	deleteNodes(ctx *context.Context, nodesToDeleteInfo []nodeToDeleteInfo) error
}
