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
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/maputil"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type MasterNodeGroupController struct {
	*NodeGroupController

	nodeToHost    map[string]string
	convergeState *context.State

	skipChecks bool
}

func NewMasterNodeGroupController(controller *NodeGroupController, skipChecks bool) *MasterNodeGroupController {
	masterNodeGroupController := &MasterNodeGroupController{
		NodeGroupController: controller,
		skipChecks:          skipChecks,
	}
	masterNodeGroupController.layoutStep = infrastructure.MasterNodeStep
	masterNodeGroupController.nodeGroup = masterNodeGroupController

	return masterNodeGroupController
}

func (c *MasterNodeGroupController) populateNodeToHost(ctx *context.Context) error {
	if c.nodeToHost != nil {
		return nil
	}

	var userPassedHosts []session.Host
	sshCl := ctx.KubeClient().NodeInterfaceAsSSHClient()
	if sshCl != nil {
		userPassedHosts = append(make([]session.Host, 0), sshCl.Session().AvailableHosts()...)
	}

	nodesNames := make([]string, 0, len(c.state.State))
	for nodeName := range c.state.State {
		nodesNames = append(nodesNames, nodeName)
	}

	nodeToHost, err := ssh.CheckSSHHosts(userPassedHosts, nodesNames, string(c.convergeState.Phase), func(msg string) bool {
		if ctx.CommanderMode() || ctx.ChangesSettings().AutoApprove {
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

func (c *MasterNodeGroupController) Run(ctx *context.Context) error {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	c.desiredReplicas = metaConfig.GetReplicasByNodeGroupName(c.name)

	log.DebugF("Desired replicas for masters %v\n", c.desiredReplicas)

	c.convergeState, err = ctx.ConvergeState()
	if err != nil {
		return err
	}

	if ctx.ChangesSettings().AutoDismissDestructive {
		log.DebugF("AutoDismissDestructive run normal\n")
		return c.runWithReplicas(ctx, metaConfig.MasterNodeGroupSpec.Replicas)
	}

	log.DebugF("run with destructive changes\n")

	return c.run(ctx)
}

func (c *MasterNodeGroupController) run(ctx *context.Context) (err error) {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	if c.convergeState.Phase == phases.ScaleToMultiMasterPhase {
		log.DebugF("scale to multi master\n")
		replicas := 3

		err = c.runWithReplicas(ctx, replicas)
		if err != nil {
			return fmt.Errorf("failed to converge with 3 replicas: %w", err)
		}

		log.DebugF("to multi master scaled. saving state...\n")

		c.convergeState.Phase = phases.ScaleToSingleMasterPhase

		err := ctx.SetConvergeState(c.convergeState)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}
	}

	if c.convergeState.Phase == phases.ScaleToSingleMasterPhase {
		log.DebugF("scale to single master\n")

		replicas := 1

		err := c.runWithReplicas(ctx, replicas)
		if err != nil {
			return fmt.Errorf("failed to converge with 1 replica: %w", err)
		}

		c.convergeState.Phase = ""

		log.DebugF("to single master scaled. saving state...\n")

		err = ctx.SetConvergeState(c.convergeState)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}

		log.DebugF("converge master nodegroup finished\n")

		return nil
	}

	return c.runWithReplicas(ctx, metaConfig.MasterNodeGroupSpec.Replicas)
}

func (c *MasterNodeGroupController) runWithReplicas(ctx *context.Context, replicas int) error {
	log.DebugF("run with replicas %v\n", c.desiredReplicas)

	c.desiredReplicas = replicas
	c.nodeToHost = nil

	return c.NodeGroupController.Run(ctx)
}

func (c *MasterNodeGroupController) addNodes(ctx *context.Context) error {
	count := len(c.state.State)
	index := 0

	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	var (
		nodesToWait        []string
		masterIPForSSHList []session.Host
		nodeInternalIPList []string
	)

	for c.desiredReplicas > count {
		candidateName := fmt.Sprintf("%s-%s-%v", metaConfig.ClusterPrefix, c.name, index)

		if _, ok := c.state.State[candidateName]; !ok {
			output, err := operations.BootstrapAdditionalMasterNode(
				ctx.Ctx(),
				ctx.KubeClient(),
				metaConfig,
				index,
				c.cloudConfig,
				true, ctx.InfrastructureContext(metaConfig),
			)
			if err != nil {
				return err
			}

			masterIPForSSHList = append(masterIPForSSHList, session.Host{Host: output.MasterIPForSSH, Name: candidateName})
			nodeInternalIPList = append(nodeInternalIPList, output.NodeInternalIP)

			count++
			c.state.State[candidateName] = output.InfrastructureState
			nodesToWait = append(nodesToWait, candidateName)
		}
		index++
	}

	err = entity.WaitForNodesListBecomeReady(ctx.Ctx(), ctx.KubeClient(), nodesToWait, controlplane.NewManagerReadinessChecker(ctx))
	if err != nil {
		return err
	}

	if len(masterIPForSSHList) > 0 {
		if !ctx.CommanderMode() {
			sshCl := ctx.KubeClient().NodeInterfaceAsSSHClient()
			if sshCl == nil {
				panic("NodeInterface is not ssh")
			}

			sshCl.Session().AddAvailableHosts(masterIPForSSHList...)
		}

		// we hide deckhouse logs because we always have config
		nodeCloudConfig, err := entity.GetCloudConfig(ctx.Ctx(), ctx.KubeClient(), c.name, global.HideDeckhouseLogs, log.GetDefaultLogger(), nodeInternalIPList...)
		if err != nil {
			return err
		}

		c.cloudConfig = nodeCloudConfig
	}

	// Update master hosts cache with all newly created masters
	if len(masterIPForSSHList) > 0 {
		log.DebugF("Updating master hosts cache with %d new masters\n", len(masterIPForSSHList))

		// Get current master hosts from cache
		stateCache := ctx.StateCache()
		currentHosts, err := state.GetMasterHostsIPs(stateCache)
		if err != nil {
			log.DebugF("Could not load current master hosts from cache (this is OK for first master): %v\n", err)
			currentHosts = []session.Host{}
		}

		hostsMap := make(map[string]string)
		for _, host := range currentHosts {
			hostsMap[host.Name] = host.Host
		}

		for _, newHost := range masterIPForSSHList {
			hostsMap[newHost.Name] = newHost.Host
			log.DebugF("Adding new master to cache: %s -> %s\n", newHost.Name, newHost.Host)
		}

		log.DebugF("Saving updated master hosts to cache: %v\n", hostsMap)

		state.SaveMasterHostsToCache(stateCache, hostsMap)

		log.DebugF("Successfully updated master hosts cache with %d new masters. hostsMap: %v\n", len(masterIPForSSHList), hostsMap)
	}

	return nil
}

func (c *MasterNodeGroupController) updateNode(ctx *context.Context, nodeName string) error {
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

	hook := c.newHookForUpdatePipeline(ctx, nodeName, metaConfig)

	var nodeGroupSettingsFromConfig []byte

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
			infrastructurestate.NewNodeStateSaver(ctx, nodeName, global.MasterNodeGroupName, nodeGroupSettingsFromConfig),
		},
		Hook: hook,
	}, ctx.ChangesSettings().AutomaticSettings)
	if err != nil {
		return err
	}

	outputs, err := infrastructure.ApplyPipeline(ctx.Ctx(), nodeRunner, nodeName, infrastructure.GetMasterNodeResult)
	if err != nil {
		if errors.Is(err, controlplane.ErrSingleMasterClusterInfrastructurePlanHasDestructiveChanges) {
			confirmation := input.NewConfirmation().WithMessage("A single-master cluster has disruptive changes in the infrastructure plan. Trying to migrate to a multi-master cluster and back to a single-master cluster. Do you want to continue?")
			if !ctx.ChangesSettings().AutoApprove && !confirmation.Ask() {
				log.InfoLn("Aborted")
				return nil
			}

			log.DebugF("Destructive change single master. Scale to multimaster and converge\n")

			c.convergeState.Phase = phases.ScaleToMultiMasterPhase

			err := ctx.SetConvergeState(c.convergeState)
			if err != nil {
				return fmt.Errorf("failed to set converge state: %w", err)
			}

			err = c.run(ctx)
			if err != nil {
				return fmt.Errorf("failed to converge to multi-master: %w", err)
			}

			return nil
		}

		log.ErrorF("Infrastructure utility exited with an error:\n%s\n", err.Error())

		return err
	}

	if tomb.IsInterrupted() {
		return global.ErrConvergeInterrupted
	}

	err = infrastructurestate.SaveMasterNodeInfrastructureState(ctx.Ctx(), ctx.KubeClient(), nodeName, outputs.InfrastructureState, []byte(outputs.KubeDataDevicePath))
	if err != nil {
		return err
	}

	c.state.State[nodeName] = outputs.InfrastructureState

	// Update master hosts IP cache after successful master node creation/update
	if outputs.MasterIPForSSH != "" {
		log.DebugF("Updating master hosts cache: node %s got IP %s\n", nodeName, outputs.MasterIPForSSH)

		// Get current master hosts from cache
		stateCache := ctx.StateCache()
		currentHosts, err := state.GetMasterHostsIPs(stateCache)
		if err != nil {
			log.DebugF("Could not load current master hosts from cache (this is OK for first master): %v\n", err)
			currentHosts = []session.Host{}
		}

		// Create map from current hosts for easier manipulation
		hostsMap := make(map[string]string)
		for _, host := range currentHosts {
			hostsMap[host.Name] = host.Host
		}

		hostsMap[nodeName] = outputs.MasterIPForSSH

		log.DebugF("Saving updated master hosts to cache: %v\n", hostsMap)

		state.SaveMasterHostsToCache(stateCache, hostsMap)

		log.DebugF("Successfully updated master hosts cache with node %s IP %s. hostsMap: %v\n", nodeName, outputs.MasterIPForSSH, hostsMap)
	} else {
		log.WarnF("No SSH IP received for master node %s, cache not updated\n", nodeName)
	}

	return entity.WaitForSingleNodeBecomeReady(ctx.Ctx(), ctx.KubeClient(), nodeName)
}

func (c *MasterNodeGroupController) newHookForUpdatePipeline(ctx *context.Context, convergedNode string, metaConfig *config.MetaConfig) infrastructure.InfraActionHook {
	err := c.populateNodeToHost(ctx)
	if err != nil {
		return nil
	}

	nodesToCheck := maputil.ExcludeKeys(c.nodeToHost, convergedNode)

	confirm := func(msg string) bool {
		return input.NewConfirmation().WithMessage(msg).Ask()
	}

	if ctx.ChangesSettings().AutoApprove {
		confirm = func(_ string) bool {
			return true
		}
	}

	return controlplane.NewHookForUpdatePipeline(ctx, nodesToCheck, metaConfig.UUID, ctx.CommanderMode(), c.skipChecks).
		WithSourceCommandName("converge").
		WithNodeToConverge(convergedNode).
		WithConfirm(confirm)
}

func (c *MasterNodeGroupController) deleteNodes(ctx *context.Context, nodesToDeleteInfo []nodeToDeleteInfo) error {
	if c.desiredReplicas < 1 {
		return fmt.Errorf(`Cannot delete ALL master nodes. If you want to remove cluster use 'dhctl destroy' command`)
	}

	needToQuorum := c.totalReplicas()/2 + 1

	noQuorum := c.desiredReplicas < needToQuorum
	msg := fmt.Sprintf("Desired master replicas count (%d) can break cluster. Need minimum replicas (%d). Do you want to continue?", c.desiredReplicas, needToQuorum)
	confirm := input.NewConfirmation().WithMessage(msg)
	if noQuorum && !confirm.Ask() {
		return fmt.Errorf("Skip delete master nodes")
	}

	title := fmt.Sprintf("Delete Nodes from NodeGroup %s (replicas: %v)", global.MasterNodeGroupName, c.desiredReplicas)
	return log.Process("converge", title, func() error {
		// Collect names of nodes to be deleted for cache cleanup
		nodesToDelete := make([]string, 0, len(nodesToDeleteInfo))
		for _, nodeInfo := range nodesToDeleteInfo {
			nodesToDelete = append(nodesToDelete, nodeInfo.name)
		}

		err := c.deleteRedundantNodes(ctx, c.state.Settings, nodesToDeleteInfo, func(nodeName string) infrastructure.InfraActionHook {
			return controlplane.NewHookForDestroyPipeline(ctx, nodeName, ctx.CommanderMode())
		})

		// If deletion was successful, update master hosts cache
		if err == nil && len(nodesToDelete) > 0 {
			log.DebugF("Updating master hosts cache after deleting %d masters: %v\n", len(nodesToDelete), nodesToDelete)

			// Get current master hosts from cache
			stateCache := ctx.StateCache()
			currentHosts, cacheErr := state.GetMasterHostsIPs(stateCache)
			if cacheErr != nil {
				log.DebugF("Could not load current master hosts from cache: %v\n", cacheErr)
				return err
			}

			hostsMap := make(map[string]string)
			for _, host := range currentHosts {
				hostsMap[host.Name] = host.Host
			}

			for _, deletedNode := range nodesToDelete {
				if _, exists := hostsMap[deletedNode]; exists {
					delete(hostsMap, deletedNode)
					log.DebugF("Removed deleted master from cache: %s\n", deletedNode)
				}
			}

			log.DebugF("Saving updated master hosts to cache after deletion: %v\n", hostsMap)

			state.SaveMasterHostsToCache(stateCache, hostsMap)

			log.DebugF("Successfully updated master hosts cache after deleting %d masters. hostsMap: %v\n", len(nodesToDelete), hostsMap)
		}

		return err
	})
}

func (c *MasterNodeGroupController) totalReplicas() int {
	return len(c.state.State)
}
