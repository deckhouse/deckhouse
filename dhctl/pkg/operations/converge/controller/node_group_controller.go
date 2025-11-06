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
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	gcmp "github.com/google/go-cmp/cmp"
	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type NodeGroupController struct {
	excludedNodes map[string]bool

	name  string
	state state.NodeGroupInfrastructureState

	nodeGroup nodeGroupController

	cloudConfig     string
	desiredReplicas int
	layoutStep      infrastructure.Step
}

func NewNodeGroupController(name string, state state.NodeGroupInfrastructureState, excludeNodes map[string]bool) *NodeGroupController {
	controller := &NodeGroupController{
		excludedNodes: excludeNodes,
		name:          name,
		state:         state,
	}

	return controller
}

func (c *NodeGroupController) Run(ctx *context.Context) error {
	// we hide deckhouse logs because we always have config
	nodeCloudConfig, err := entity.GetCloudConfig(ctx.Ctx(), ctx.KubeClient(), c.name, global.HideDeckhouseLogs, log.GetDefaultLogger())
	if err != nil {
		return err
	}

	c.cloudConfig = nodeCloudConfig

	if c.desiredReplicas > len(c.state.State) {
		err := log.Process("converge", fmt.Sprintf("Add Nodes to NodeGroup %s (replicas: %v)", c.name, c.desiredReplicas), func() error {
			return c.nodeGroup.addNodes(ctx)
		})
		if err != nil {
			return err
		}
	}

	nodesToDeleteInfo, err := getNodesToDeleteInfo(c.desiredReplicas, c.state.State)
	if err != nil {
		return err
	}

	log.DebugF("nodes to delete %v\n", len(nodesToDeleteInfo))

	if !ctx.CommanderMode() {
		sshClient := ctx.KubeClient().NodeInterfaceAsSSHClient()
		log.DebugF("sshClient: %v\n", sshClient)
		if sshClient != nil {
			availableHosts := sshClient.Session().AvailableHosts()
			needReconnect := false
			for _, host := range availableHosts {
				for _, dhost := range nodesToDeleteInfo {
					if host.Name == dhost.name {
						ctx.KubeClient().NodeInterfaceAsSSHClient().Session().RemoveAvailableHosts(host)
						if host.Host == ctx.KubeClient().NodeInterfaceAsSSHClient().Session().Host() {
							needReconnect = true
						}
					}
				}
			}

			log.DebugF("list of available host: %-v\n", ctx.KubeClient().NodeInterfaceAsSSHClient().Session().AvailableHosts())

			if len(nodesToDeleteInfo) > 0 && needReconnect {
				err = retry.NewSilentLoop("reconnecting to SSH", 10, 10).Run(func() error {
					ctx.KubeClient().NodeInterfaceAsSSHClient().Stop()
					err = ctx.KubeClient().NodeInterfaceAsSSHClient().Start()
					return err
				})
				if err != nil {
					return err
				}

				kubeCl, err := kubernetes.ConnectToKubernetesAPI(ctx.Ctx(), ssh.NewNodeInterfaceWrapper(ctx.KubeClient().NodeInterfaceAsSSHClient()))
				if err != nil {
					return fmt.Errorf("unable to connect to Kubernetes over ssh tunnel: %w", err)
				}

				newCtx := context.NewContext(ctx.Ctx(), context.Params{
					KubeClient:     kubeCl,
					Cache:          ctx.StateCache(),
					ChangeParams:   ctx.ChangesSettings(),
					ProviderGetter: ctx.ProviderGetter(),
					Logger:         ctx.Logger(),
				})
				ctx = newCtx
			}

		}
	}

	log.DebugF("starting update nodes\n")

	err = c.updateNodes(ctx)
	if err != nil {
		return err
	}

	log.DebugF("starting delete nodes\n")

	err = c.tryDeleteNodes(ctx, nodesToDeleteInfo)
	if err != nil {
		return err
	}

	groupSpec, err := c.getSpec(ctx, c.name)
	if err != nil {
		return err
	}

	log.DebugF("starting converge node template\n")

	if groupSpec != nil {
		return c.tryUpdateNodeTemplate(ctx, groupSpec.NodeTemplate)
	}

	return c.tryDeleteNodeGroup(ctx)
}

func (c *NodeGroupController) tryDeleteNodes(ctx *context.Context, nodesToDeleteInfo []nodeToDeleteInfo) error {
	if len(nodesToDeleteInfo) == 0 {
		log.DebugLn("No nodes to delete")
		return nil
	}

	if ctx.ChangesSettings().AutoDismissDestructive {
		log.DebugLn("Skip delete nodes because destructive operations are disabled")
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
			log.ErrorLn(err)
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

	var allErrs *multierror.Error
	for _, nodeToDeleteInfo := range nodesToDeleteInfo {
		if _, ok := c.excludedNodes[nodeToDeleteInfo.name]; ok {
			log.InfoF("Skip delete excluded node %v\n", nodeToDeleteInfo.name)
			continue
		}

		nodeIndex, err := config.GetIndexFromNodeName(nodeToDeleteInfo.name)
		if err != nil {
			log.ErrorF("can't extract index from infrastructure state secret (%v), skip %s\n", err, nodeToDeleteInfo.name)
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

		if err := entity.DeleteNode(ctx.Ctx(), ctx.KubeClient(), nodeToDeleteInfo.name); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", nodeToDeleteInfo.name, err))
			continue
		}

		if err := infrastructurestate.DeleteNodeInfrastructureStateFromCache(nodeToDeleteInfo.name, ctx.StateCache()); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("unable to delete node %s infrastructure state from cache: %w", nodeToDeleteInfo.name, err))
			continue
		}

		if err := infrastructurestate.DeleteInfrastructureState(ctx.Ctx(), ctx.KubeClient(), fmt.Sprintf("d8-node-terraform-state-%s", nodeToDeleteInfo.name)); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %w", nodeToDeleteInfo.name, err))
			continue
		}
	}

	return allErrs.ErrorOrNil()
}

func (c *NodeGroupController) tryUpdateNodeTemplate(ctx *context.Context, nodeTemplate map[string]interface{}) error {
	nodeTemplatePath := []string{"spec", "nodeTemplate"}
	for {
		ng, err := entity.GetNodeGroup(ctx.Ctx(), ctx.KubeClient(), c.name)
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

		if !ctx.ChangesSettings().AutoApprove && !input.NewConfirmation().WithMessage(msg).Ask() {
			log.InfoLn("Updating node group template was skipped")
			return nil
		}

		err = unstructured.SetNestedMap(ng.Object, nodeTemplate, nodeTemplatePath...)
		if err != nil {
			return err
		}

		err = entity.UpdateNodeGroup(ctx.Ctx(), ctx.KubeClient(), c.name, ng)

		if err == nil {
			return nil
		}

		if errors.Is(err, global.ErrNodeGroupChanged) {
			log.WarnLn(err.Error())
			continue
		}

		return err
	}
}

func (c *NodeGroupController) tryDeleteNodeGroup(ctx *context.Context) error {
	if ctx.ChangesSettings().AutoDismissDestructive {
		log.DebugF("Skip delete %s node group because destructive operations are disabled\n", c.name)
		return nil
	}

	if c.name == global.MasterNodeGroupName {
		log.DebugF("Skip delete %s node group because it is master\n", c.name)
		return nil
	}

	return log.Process("converge", fmt.Sprintf("Delete NodeGroup %s", c.name), func() error {
		return entity.DeleteNodeGroup(ctx.Ctx(), ctx.KubeClient(), c.name)
	})
}

func (c *NodeGroupController) getSpec(ctx *context.Context, name string) (*config.TerraNodeGroupSpec, error) {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return nil, err
	}
	for _, terranodeGroup := range metaConfig.GetTerraNodeGroups() {
		if terranodeGroup.Name == name {
			cc := terranodeGroup
			return &cc, nil
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

	for _, nodeName := range nodeNames {
		processTitle := fmt.Sprintf("Update Node %s in NodeGroup %s (replicas: %v)", nodeName, c.name, replicas)

		err := log.Process("converge", processTitle, func() error {
			if _, ok := c.excludedNodes[nodeName]; ok {
				log.InfoF("Skip update excluded node %v\n", nodeName)
				return nil
			}

			err = c.nodeGroup.updateNode(ctx, nodeName)
			if err != nil {
				return err
			}

			// we hide deckhouse logs because we always have config
			nodeCloudConfig, err := entity.GetCloudConfig(ctx.Ctx(), ctx.KubeClient(), c.name, global.HideDeckhouseLogs, log.GetDefaultLogger())
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

func getNodesToDeleteInfo(desiredReplicas int, state map[string][]byte) ([]nodeToDeleteInfo, error) {
	if desiredReplicas >= len(state) {
		log.DebugF("desired replicas >= in state. skip nodes info\n")
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
			log.DebugF("stopping getting deletes nodes info. count %v\n", count)
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
	updateNode(ctx *context.Context, name string) error
	deleteNodes(ctx *context.Context, nodesToDeleteInfo []nodeToDeleteInfo) error
}
