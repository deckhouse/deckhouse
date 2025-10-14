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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type CloudPermanentNodeGroupController struct {
	*NodeGroupController
}

func NewCloudPermanentNodeGroupController(controller *NodeGroupController) *CloudPermanentNodeGroupController {
	cloudPermanentNodeGroupController := &CloudPermanentNodeGroupController{NodeGroupController: controller}
	cloudPermanentNodeGroupController.layoutStep = infrastructure.StaticNodeStep
	cloudPermanentNodeGroupController.nodeGroup = cloudPermanentNodeGroupController

	return cloudPermanentNodeGroupController
}

func (c *CloudPermanentNodeGroupController) Run(ctx *context.Context) error {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	c.desiredReplicas = metaConfig.GetReplicasByNodeGroupName(c.name)

	return c.NodeGroupController.Run(ctx)
}

func (c *CloudPermanentNodeGroupController) addNodes(ctx *context.Context) error {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	count := len(c.state.State)
	index := 0

	var (
		nodesToWait        []string
		nodesIndexToCreate []int
	)

	for c.desiredReplicas > count {
		candidateName := fmt.Sprintf("%s-%s-%v", metaConfig.ClusterPrefix, c.name, index)
		if _, ok := c.state.State[candidateName]; !ok {
			nodesIndexToCreate = append(nodesIndexToCreate, index)
			count++
		}
		index++
	}

	err = log.Process("infrastructure", fmt.Sprintf("Pipelines %s for %s-%s-%v", c.layoutStep, metaConfig.ClusterPrefix, c.name, nodesIndexToCreate), func() error {
		var err error
		nodesToWait, err = operations.ParallelBootstrapAdditionalNodes(
			ctx.Ctx(),
			ctx.KubeClient(),
			metaConfig,
			nodesIndexToCreate,
			c.layoutStep,
			c.name,
			c.cloudConfig,
			true,
			ctx.InfrastructureContext(metaConfig),
			log.GetDefaultLogger(),
			false,
		)
		return err
	})
	if err != nil {
		return err
	}
	return entity.WaitForNodesListBecomeReady(ctx.Ctx(), ctx.KubeClient(), nodesToWait, nil)
}

func (c *CloudPermanentNodeGroupController) updateNode(ctx *context.Context, nodeName string) error {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	// NOTE: In the commander mode nodes state should exist in the local state cache, no need to pass state explicitly.
	var nodeState []byte
	if !ctx.CommanderMode() {
		nodeState = c.state.State[nodeName]
	}

	nodeIndex, err := config.GetIndexFromNodeName(nodeName)
	if err != nil {
		log.ErrorF("can't extract index from infrastructure state secret (%v), skip %s\n", err, nodeName)
		return nil
	}

	nodeGroupName := c.name
	var nodeGroupSettingsFromConfig []byte

	// Node group settings are only for the static node.
	nodeGroupSettingsFromConfig = metaConfig.FindTerraNodeGroup(c.name)

	nodeRunner, err := ctx.InfrastructureContext(metaConfig).GetConvergeNodeRunner(ctx.Ctx(), metaConfig, infrastructure.NodeRunnerOptions{
		NodeName:        nodeName,
		NodeGroupName:   c.name,
		NodeGroupStep:   c.layoutStep,
		NodeIndex:       nodeIndex,
		NodeState:       nodeState,
		NodeCloudConfig: c.cloudConfig,
		CommanderMode:   ctx.CommanderMode(),
		StateCache:      ctx.StateCache(),
		AdditionalStateSaverDestinations: []infrastructure.SaverDestination{
			infrastructurestate.NewNodeStateSaver(ctx, nodeName, nodeGroupName, nodeGroupSettingsFromConfig),
		},
		Hook: &infrastructure.DummyHook{},
	}, ctx.ChangesSettings().AutomaticSettings)

	if err != nil {
		return err
	}

	outputs, err := infrastructure.ApplyPipeline(ctx.Ctx(), nodeRunner, nodeName, infrastructure.OnlyState)
	if err != nil {
		log.ErrorF("Infrastructure utility exited with an error:\n%s\n", err.Error())
		return err
	}

	if tomb.IsInterrupted() {
		return global.ErrConvergeInterrupted
	}

	err = infrastructurestate.SaveNodeInfrastructureState(ctx.Ctx(), ctx.KubeClient(), nodeName, c.name, outputs.InfrastructureState, nodeGroupSettingsFromConfig, log.GetDefaultLogger())
	if err != nil {
		return err
	}

	return entity.WaitForSingleNodeBecomeReady(ctx.Ctx(), ctx.KubeClient(), nodeName)
}

func (c *CloudPermanentNodeGroupController) deleteNodes(ctx *context.Context, nodesToDeleteInfo []nodeToDeleteInfo) error {
	title := fmt.Sprintf("Delete Nodes from NodeGroup %s (replicas: %v)", c.name, c.desiredReplicas)
	return log.Process("converge", title, func() error {
		return c.deleteRedundantNodes(ctx, c.state.Settings, nodesToDeleteInfo, func(nodeName string) infrastructure.InfraActionHook {
			return NewHookForDestroyPipeline(ctx, nodeName, ctx.CommanderMode())
		})
	})
}
