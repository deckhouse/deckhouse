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
	gocontext "context"
	"errors"
	"fmt"

	"github.com/name212/govalue"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/utils"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
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

	// An immutable master is addressed by name, never by SSH. The map still has to
	// list the other masters: an empty one reads as "single-master cluster" and turns
	// every destructive plan into the scale dance.
	if c.immutable {
		c.nodeToHost = make(map[string]string, len(c.state.State))
		for nodeName := range c.state.State {
			c.nodeToHost[nodeName] = ""
		}

		return nil
	}

	if ctx.SSHProviderInitializer == nil {
		return nil
	}

	var userPassedHosts []session.Host
	sshProvider, err := ctx.SSHProviderInitializer.GetSSHProvider(ctx.Ctx())
	if err != nil {
		return err
	}

	sshCl, err := sshProvider.Client(ctx.Ctx())
	if err != nil {
		return err
	}

	if sshCl != nil {
		userPassedHosts = append(make([]session.Host, 0), sshCl.Session().AvailableHosts()...)
	}

	nodesNames := make([]string, 0, len(c.state.State))
	for nodeName := range c.state.State {
		nodesNames = append(nodesNames, nodeName)
	}

	nodeToHost, err := utils.CheckSSHHosts(userPassedHosts, nodesNames, string(c.convergeState.Phase), confirmHostsMapping(ctx))
	if err != nil {
		return err
	}

	c.nodeToHost = nodeToHost

	return nil
}

// confirmHostsMapping answers the node-to-host confirmation CheckSSHHosts asks on every
// run. With no terminal nobody is there to answer, and a silent "no" used to cost the
// whole control plane hook — the master was then recreated with no guard at all. The
// mapping is accepted and written to the log instead; a destructive plan still has its
// own approval.
func confirmHostsMapping(ctx *context.Context) func(string) bool {
	return func(msg string) bool {
		if ctx.CommanderMode() || ctx.ChangesSettings().AutoApprove {
			return true
		}

		if !input.IsTerminal() {
			dhlog.FromContext(ctx.Ctx()).WarnContext(ctx.Ctx(),
				fmt.Sprintf("Node to host mapping accepted without confirmation: no TTY.\n%s", msg))
			return true
		}

		return input.NewConfirmation().WithMessage(msg).Ask()
	}
}

func (c *MasterNodeGroupController) Run(ctx *context.Context) error {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	c.desiredReplicas = metaConfig.GetReplicasByNodeGroupName(c.name)

	dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Desired replicas for masters %v", c.desiredReplicas))

	c.convergeState, err = ctx.ConvergeState()
	if err != nil {
		return err
	}

	if ctx.ChangesSettings().AutoDismissDestructive {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "AutoDismissDestructive run normal")
		return c.runWithReplicas(ctx, metaConfig.MasterNodeGroupSpec.Replicas)
	}

	dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "run with destructive changes")

	return c.run(ctx)
}

func (c *MasterNodeGroupController) run(ctx *context.Context) error {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	if c.convergeState.Phase == phases.ScaleToMultiMasterPhase {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "Scale to multi master")

		replicas := 3

		err = c.runWithReplicas(ctx, replicas)
		if err != nil {
			return fmt.Errorf("failed to converge with 3 replicas: %w", err)
		}

		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "scaled to multi-master, saving state...")

		c.convergeState.Phase = phases.ScaleToSingleMasterPhase

		err := ctx.SetConvergeState(c.convergeState)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}
	}

	if c.convergeState.Phase == phases.ScaleToSingleMasterPhase {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "Scale to single master")

		if err := c.switchClientToFirstMaster(ctx); err != nil {
			return err
		}

		replicas := 1

		err := c.runWithReplicas(ctx, replicas)
		if err != nil {
			return fmt.Errorf("failed to converge with 1 replica: %w", err)
		}

		c.convergeState.Phase = ""

		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "scaled to single-master, saving state...")

		err = ctx.SetConvergeState(c.convergeState)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}

		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "converge master nodegroup finished")

		return nil
	}

	return c.runWithReplicas(ctx, metaConfig.MasterNodeGroupSpec.Replicas)
}

func (c *MasterNodeGroupController) switchClientToNotFirstMaster(ctx *context.Context) error {
	clientSwitcher := ctx.ClientSwitcher()
	if govalue.IsNil(clientSwitcher) {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "Skipping switch of client to not-first master. Got empty client switcher")
		return nil
	}

	// commander mode and another checks checking inside
	if err := clientSwitcher.SwitchToNotFirstMaster(ctx.Ctx()); err != nil {
		return fmt.Errorf("Cannot switch clients to not first control-plane node: %w", err)
	}

	return nil
}

func (c *MasterNodeGroupController) switchClientToFirstMaster(ctx *context.Context) error {
	clientSwitcher := ctx.ClientSwitcher()
	if govalue.IsNil(clientSwitcher) {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "Skipping switch of client to first master. Got empty client switcher")
		return nil
	}

	// commander mode and another checks checking inside
	if err := clientSwitcher.SwitchToFirstMaster(ctx.Ctx()); err != nil {
		return fmt.Errorf("Cannot switch clients to first control-plane node: %w", err)
	}

	return nil
}

func (c *MasterNodeGroupController) runWithReplicas(ctx *context.Context, replicas int) error {
	dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("run with replicas %v", c.desiredReplicas))

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

	kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return fmt.Errorf("Could not get kube client: %w", err)
	}

	for c.desiredReplicas > count {
		candidateName := fmt.Sprintf("%s-%s-%v", metaConfig.ClusterPrefix, c.name, index)

		if _, ok := c.state.State[candidateName]; !ok {
			cloudConfig := c.cloudConfig
			if c.immutable {
				cloudConfig, err = immutableMasterPayload(ctx, candidateName)
				if err != nil {
					return err
				}
			}

			output, err := operations.BootstrapAdditionalMasterNode(
				ctx.Ctx(),
				kubeClient,
				metaConfig,
				index,
				cloudConfig,
				ctx.InfrastructureContext(metaConfig),
				c.globalOptions,
			)
			if err != nil {
				return err
			}

			masterIPForSSHList = append(masterIPForSSHList, session.Host{Host: output.MasterIPForSSH, Name: candidateName})
			nodeInternalIPList = append(nodeInternalIPList, output.NodeInternalIP)

			count++
			c.state.State[candidateName] = output.InfrastructureState
			nodesToWait = append(nodesToWait, candidateName)

			// One at a time: etcd admits a single learner, so the next machine must not
			// start joining until control-plane-manager has this one voting.
			if c.immutable {
				if err := waitForImmutableMasterControlPlane(ctx, candidateName); err != nil {
					return err
				}
			}
		}
		index++
	}

	err = entity.WaitForNodesListBecomeReady(ctx.Ctx(), kubeClient, nodesToWait, controlplane.NewManagerReadinessChecker(ctx))
	if err != nil {
		return err
	}

	// An immutable node answers no sshd, and its payload comes from the cluster rather
	// than from the group's bashible secret: registering it as an SSH host and waiting
	// for that secret to list it would both stall on something that never happens.
	if len(masterIPForSSHList) > 0 && !c.immutable {
		if err := c.addNewNodesToSSH(ctx, masterIPForSSHList); err != nil {
			return err
		}

		// we hide deckhouse logs because we always have config
		nodeCloudConfig, err := entity.GetCloudConfig(ctx.Ctx(), ctx, c.name, global.HideDeckhouseLogs, nodeInternalIPList...)
		if err != nil {
			return err
		}
		c.cloudConfig = nodeCloudConfig

		c.addNewNodesToCache(ctx, masterIPForSSHList)
	}

	return nil
}

func (c *MasterNodeGroupController) beforeUpdateNodes(ctx *context.Context) error {
	noScaleToMultiMaster := c.convergeState.Phase != phases.ScaleToMultiMasterPhase

	dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Master ng. beforeUpdateNodes: has phase %s; noScaleToMultiMaster %v", c.convergeState.Phase, noScaleToMultiMaster))

	if noScaleToMultiMaster {
		return nil
	}

	return c.switchClientToNotFirstMaster(ctx)
}

func (c *MasterNodeGroupController) addNewNodesToSSH(ctx *context.Context, masterIPForSSHList []session.Host) error {
	if ctx.CommanderMode() {
		return nil
	}
	sshProvider, err := ctx.SSHProviderInitializer.GetSSHProvider(ctx.Ctx())
	if err != nil {
		return err
	}

	sshCl, err := sshProvider.Client(ctx.Ctx())
	if err != nil {
		return err
	}

	if govalue.IsNil(sshCl) {
		return fmt.Errorf("NodeInterface is not ssh")
	}

	sshCl.Session().AddAvailableHosts(masterIPForSSHList...)

	return nil
}

// sshProviderForHooks builds the provider the control-plane hooks reach nodes with.
// An immutable master is never registered as an SSH host, so there is nothing to
// build: the lookup fails on a master-hosts cache that was never written.
func (c *MasterNodeGroupController) sshProviderForHooks(ctx *context.Context) (libcon.SSHProvider, error) {
	if c.immutable {
		return nil, nil
	}

	return ctx.SSHProviderInitializer.GetSSHProvider(ctx.Ctx())
}

func (c *MasterNodeGroupController) addNewNodesToCache(ctx *context.Context, masterIPForSSHList []session.Host) {
	// An immutable node answers no sshd. A cached address makes every later converge
	// see an SSH-reachable cluster and wait for bashible on a machine without it.
	if c.immutable {
		return
	}

	dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Updating master hosts cache with %d new masters", len(masterIPForSSHList)))

	// Get current master hosts from cache
	stateCache := ctx.StateCache()
	currentHosts, err := state.GetMasterHostsIPs(ctx.Ctx(), stateCache)
	if err != nil {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Could not load current master hosts from cache (this is OK for first master): %v", err))
		currentHosts = []session.Host{}
	}

	hostsMap := make(map[string]string)
	for _, host := range currentHosts {
		hostsMap[host.Name] = host.Host
	}

	for _, newHost := range masterIPForSSHList {
		hostsMap[newHost.Name] = newHost.Host
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Adding new master to cache: %s -> %s", newHost.Name, newHost.Host))
	}

	dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Saving updated master hosts to cache: %v", hostsMap))

	state.SaveMasterHostsToCache(ctx.Ctx(), stateCache, hostsMap)

	dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Successfully updated master hosts cache with %d new masters. hostsMap: %v", len(masterIPForSSHList), hostsMap))
}

func (c *MasterNodeGroupController) updateNode(ctx *context.Context, nodeName string, nodeIndex int) error {
	metaConfig, err := ctx.MetaConfig()
	if err != nil {
		return err
	}

	// NOTE: In the commander mode nodes state should exist in the local state cache, no need to pass state explicitly.
	var nodeState []byte
	if !ctx.CommanderMode() {
		nodeState = c.state.State[nodeName]
	}

	hook, err := c.newHookForUpdatePipeline(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("build control plane hook for node %s: %w", nodeName, err)
	}

	// The payload only reaches the machine when the VM is recreated: the cloud-init
	// secret is immutable and its name carries the plan's destructive hash. Building
	// it here means a recreated master joins with a live token and live apiservers.
	cloudConfig := c.cloudConfig
	if c.immutable {
		cloudConfig, err = immutableMasterPayload(ctx, nodeName)
		if err != nil {
			return err
		}
	}

	var nodeGroupSettingsFromConfig []byte

	nodeRunner, err := ctx.InfrastructureContext(metaConfig).GetConvergeNodeRunner(ctx.Ctx(), metaConfig, infrastructure.NodeRunnerOptions{
		NodeName:        nodeName,
		NodeGroupName:   c.name,
		NodeGroupStep:   c.layoutStep,
		NodeIndex:       nodeIndex,
		NodeState:       nodeState,
		NodeCloudConfig: cloudConfig,
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

	outputs, err := infrastructure.ApplyPipeline(ctx.Ctx(), nodeRunner, nodeName, c.globalOptions, infrastructure.GetMasterNodeResult)
	if err != nil {
		if errors.Is(err, controlplane.ErrSingleMasterClusterInfrastructurePlanHasDestructiveChanges) {
			confirmation := input.NewConfirmation().WithMessage("A single-master cluster has disruptive changes in the infrastructure plan. Trying to migrate to a multi-master cluster and back to a single-master cluster. Do you want to continue?")
			if !ctx.ChangesSettings().AutoApprove && !confirmation.Ask() {
				dhlog.FromContext(ctx.Ctx()).InfoContext(ctx.Ctx(), "Aborted")
				return nil
			}

			dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "Destructive change single master. Scale to multimaster and converge")

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

		dhlog.FromContext(ctx.Ctx()).ErrorContext(ctx.Ctx(), fmt.Sprintf("Infrastructure utility exited with an error:\n%s", err.Error()))

		return err
	}

	if tomb.IsInterrupted() {
		return global.ErrConvergeInterrupted
	}

	kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return fmt.Errorf("Could not get kube client: %w", err)
	}

	err = infrastructurestate.SaveMasterNodeInfrastructureState(ctx.Ctx(), kubeClient, nodeName, outputs.InfrastructureState, []byte(outputs.KubeDataDevicePath))
	if err != nil {
		return err
	}

	c.state.State[nodeName] = outputs.InfrastructureState

	// Update master hosts IP cache after successful master node creation/update
	if outputs.MasterIPForSSH != "" {
		c.addNewNodesToCache(ctx, []session.Host{{Host: outputs.MasterIPForSSH, Name: nodeName}})
	} else {
		dhlog.FromContext(ctx.Ctx()).WarnContext(ctx.Ctx(), fmt.Sprintf("No SSH IP received for master node %s, cache not updated", nodeName))
	}

	return entity.WaitForSingleNodeBecomeReady(ctx.Ctx(), kubeClient, nodeName)
}

// newHookForUpdatePipeline reports its failures instead of returning a nil hook:
// the runner substitutes a DummyHook for a nil one, and a master VM would then be
// recreated without removing its control plane role, its Node object or its etcd
// membership.
func (c *MasterNodeGroupController) newHookForUpdatePipeline(ctx *context.Context, convergedNode string) (infrastructure.InfraActionHook, error) {
	err := c.populateNodeToHost(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect master addresses: %w", err)
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

	sshProvider, err := c.sshProviderForHooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("get ssh provider: %w", err)
	}

	return controlplane.NewHookForUpdatePipeline(
		ctx,
		sshProvider,
		nodesToCheck,
		ctx.CommanderMode(),
		c.skipChecks,
		c.immutable,
	).
		WithSourceCommandName("converge").
		WithNodeToConverge(convergedNode).
		WithConfirm(confirm).
		WithClientSwitcher(ctx.ClientSwitcher()), nil
}

func (c *MasterNodeGroupController) deleteNodes(ctx *context.Context, nodesToDeleteInfo []nodeToDeleteInfo) error {
	if c.desiredReplicas < 1 {
		return fmt.Errorf(`Cannot delete ALL master nodes. If you want to remove the cluster, use the 'dhctl destroy' command`)
	}

	needToQuorum := c.totalReplicas()/2 + 1

	noQuorum := c.desiredReplicas < needToQuorum
	msg := fmt.Sprintf("Desired master replica count (%d) can break the cluster. The minimum number of replicas required is (%d). Do you want to continue?", c.desiredReplicas, needToQuorum)
	confirm := input.NewConfirmation().WithMessage(msg)
	if noQuorum && !confirm.Ask() {
		return fmt.Errorf("Skipping deletion of master nodes")
	}

	title := fmt.Sprintf("Delete Nodes from NodeGroup %s (replicas: %v)", global.MasterNodeGroupName, c.desiredReplicas)
	return dhlog.RunProcess(ctx.Ctx(), dhlog.FromContext(ctx.Ctx()), title, func(gocontext.Context) error {
		// Collect names of nodes to be deleted for cache cleanup
		nodesToDelete := make([]string, 0, len(nodesToDeleteInfo))
		for _, nodeInfo := range nodesToDeleteInfo {
			nodesToDelete = append(nodesToDelete, nodeInfo.name)
		}

		sshProvider, err := c.sshProviderForHooks(ctx)
		if err != nil {
			return err
		}

		err = c.deleteRedundantNodes(
			ctx,
			c.state.Settings,
			nodesToDeleteInfo,
			func(nodeName string) infrastructure.InfraActionHook {
				return controlplane.NewHookForDestroyPipeline(
					ctx,
					sshProvider,
					nodeName,
					ctx.CommanderMode(),
					c.immutable,
				)
			},
			func(nodeName string) {
				standaloneProvider, ok := sshProvider.(libcon.StandaloneClientProvider)
				if !ok {
					return
				}

				standaloneProvider.StopStandaloneClientFor(
					ctx.Ctx(),
					controlplane.SSHCheckerClientKey(nodeName),
				)
			},
		)

		// If deletion was successful, update master hosts cache
		if err == nil && len(nodesToDelete) > 0 {
			dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Updating master hosts cache after deleting %d masters: %v", len(nodesToDelete), nodesToDelete))

			// Get current master hosts from cache
			stateCache := ctx.StateCache()
			currentHosts, cacheErr := state.GetMasterHostsIPs(ctx.Ctx(), stateCache)
			if cacheErr != nil {
				dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Could not load current master hosts from cache: %v", cacheErr))
				return err
			}

			hostsMap := make(map[string]string)
			for _, host := range currentHosts {
				hostsMap[host.Name] = host.Host
			}

			for _, deletedNode := range nodesToDelete {
				if _, exists := hostsMap[deletedNode]; exists {
					delete(hostsMap, deletedNode)
					dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Removed deleted master from cache: %s", deletedNode))
				}
			}

			dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Saving updated master hosts to cache after deletion: %v", hostsMap))

			state.SaveMasterHostsToCache(ctx.Ctx(), stateCache, hostsMap)

			dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), fmt.Sprintf("Successfully updated master hosts cache after deleting %d masters. hostsMap: %v", len(nodesToDelete), hostsMap))
		}

		return err
	})
}

func (c *MasterNodeGroupController) totalReplicas() int {
	return len(c.state.State)
}
