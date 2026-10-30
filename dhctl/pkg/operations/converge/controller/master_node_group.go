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
	"maps"
	"slices"

	"github.com/name212/govalue"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/utils"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/maputil"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

// errMasterHostsRefreshFailed marks a master state that changed while the SSH inventory
// and the readiness checks could not follow it. The next master must not be changed: its
// checks would reach the other masters at addresses that may be gone.
var errMasterHostsRefreshFailed = errors.New("master SSH hosts do not follow the infrastructure states")

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

	if ctx.SSHProviderInitializer == nil {
		return nil
	}

	nodesNames := make([]string, 0, len(c.state.State))
	for nodeName := range c.state.State {
		nodesNames = append(nodesNames, nodeName)
	}

	hostsForChecks, err := c.masterHostsForChecks(ctx, nodesNames)
	if err != nil {
		return err
	}

	nodeToHost, err := utils.CheckSSHHosts(hostsForChecks, nodesNames, string(c.convergeState.Phase), func(msg string) bool {
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

func (c *MasterNodeGroupController) masterHostsForChecks(ctx *context.Context, nodeNames []string) ([]session.Host, error) {
	// A resumed converge may already have every master state while its newly created SSH
	// session knows only the address from --ssh-host, with no node name attached. Refresh
	// first so readiness mapping is built from the authoritative, named state inventory.
	clientSwitcher := ctx.ClientSwitcher()
	if !govalue.IsNil(clientSwitcher) {
		hosts, err := clientSwitcher.RefreshMasterHostsFromStates(ctx.Ctx(), c.inventoryStates())
		if err != nil {
			return nil, fmt.Errorf("%w: refresh hosts before building readiness inventory: %w", errMasterHostsRefreshFailed, err)
		}

		return hostsMatchingNodes(hosts, nodeNames), nil
	}

	// Keep the old provider path for contexts without a switcher. Production converge
	// installs one, while focused callers and tests may still supply a provider directly.
	sshProvider, err := ctx.SSHProviderInitializer.GetSSHProvider(ctx.Ctx())
	if err != nil {
		return nil, err
	}

	sshCl, err := sshProvider.Client(ctx.Ctx())
	if err != nil {
		return nil, err
	}

	var availableHosts []session.Host
	if sshCl != nil {
		availableHosts = sshCl.Session().AvailableHosts()
	}

	// The live session deliberately keeps masters scheduled for deletion until the
	// client has moved away from them. They are failover candidates, not nodes whose
	// readiness has to be checked after the scale-down decision. Passing them to
	// CheckSSHHosts makes it report a false "too many hosts" warning.
	return hostsMatchingNodes(availableHosts, nodeNames), nil
}

func hostsMatchingNodes(hosts []session.Host, nodeNames []string) []session.Host {
	nodes := make(map[string]struct{}, len(nodeNames))
	for _, nodeName := range nodeNames {
		nodes[nodeName] = struct{}{}
	}

	matching := make([]session.Host, 0, len(nodeNames))
	for _, host := range hosts {
		if _, ok := nodes[host.Name]; ok {
			matching = append(matching, host)
		}
	}

	return matching
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
		nodeInternalIPList []string
	)

	kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return fmt.Errorf("Could not get kube client: %w", err)
	}

	for c.desiredReplicas > count {
		candidateName := fmt.Sprintf("%s-%s-%v", metaConfig.ClusterPrefix, c.name, index)

		if _, ok := c.state.State[candidateName]; !ok {
			output, err := operations.BootstrapAdditionalMasterNode(
				ctx.Ctx(),
				kubeClient,
				metaConfig,
				index,
				c.cloudConfig,
				ctx.InfrastructureContext(metaConfig),
				c.globalOptions,
			)
			if err != nil {
				return err
			}

			nodeInternalIPList = append(nodeInternalIPList, output.NodeInternalIP)

			count++
			c.state.State[candidateName] = output.InfrastructureState
			c.rememberMasterIPs(ctx, candidateName, output)
			nodesToWait = append(nodesToWait, candidateName)
		}
		index++
	}

	err = entity.WaitForNodesListBecomeReady(ctx.Ctx(), kubeClient, nodesToWait, controlplane.NewManagerReadinessChecker(ctx))
	if err != nil {
		return err
	}

	if len(nodesToWait) > 0 {
		if err := c.refreshMasterHostsFromStates(ctx); err != nil {
			return err
		}

		// The refresh can have switched the SSH client, and the kube client with it.
		kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
		if err != nil {
			return fmt.Errorf("Could not get kube client: %w", err)
		}

		// we hide deckhouse logs because we always have config
		nodeCloudConfig, err := entity.GetCloudConfig(ctx.Ctx(), kubeClient, c.name, global.HideDeckhouseLogs, nodeInternalIPList...)
		if err != nil {
			return err
		}
		c.cloudConfig = nodeCloudConfig
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
		dhlog.FromContext(ctx.Ctx()).ErrorContext(ctx.Ctx(), fmt.Sprintf("can't extract index from infrastructure state secret (%v), skipping %s", err, nodeName))
		return nil
	}

	hook, err := c.newHookForUpdatePipeline(ctx, nodeName, metaConfig)
	if err != nil {
		return fmt.Errorf("build control plane hook for node %s: %w", nodeName, err)
	}

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

		return c.afterFailedApply(ctx, nodeName, nodeRunner.GetState, err)
	}

	if tomb.IsInterrupted() {
		return global.ErrConvergeInterrupted
	}

	c.state.State[nodeName] = outputs.InfrastructureState
	c.rememberMasterIPs(ctx, nodeName, outputs)

	// The inventory follows the state before anything goes to the cluster. The apply can
	// have replaced the address the client is tunnelled through, and the save would fail
	// through the dead tunnel. Should the save or the wait fail anyway, the next master's
	// checks must still reach this one at its new address.
	refreshErr := c.refreshMasterHostsFromStates(ctx)
	if errors.Is(refreshErr, context.ErrSSHClientSwitchFailed) {
		// No working client is left to save through. The node state saver sent the states
		// to the cluster during the apply, but only as a best effort.
		dhlog.FromContext(ctx.Ctx()).WarnContext(ctx.Ctx(), fmt.Sprintf(
			"The infrastructure state of node %s may be missing from the cluster: it could not be saved after the apply.", nodeName))
		return refreshErr
	}

	// The refresh can have replaced the SSH client, and the kube client tunnelled through
	// the old one with it: the provider hands out a new one for a changed session.
	kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return errors.Join(refreshErr, fmt.Errorf("Could not get kube client: %w", err))
	}

	err = infrastructurestate.SaveMasterNodeInfrastructureState(ctx.Ctx(), kubeClient, nodeName, outputs.InfrastructureState, []byte(outputs.KubeDataDevicePath))
	if err := errors.Join(refreshErr, err); err != nil {
		return err
	}

	return entity.WaitForSingleNodeBecomeReady(ctx.Ctx(), kubeClient, nodeName)
}

// afterFailedApply keeps what a failed apply left of the node and refreshes the SSH
// inventory from it. The apply can have recreated the VM before it stopped: if the update
// loop continues, readiness checks for the next master must reach this one at the new address.
// Whatever keeps the inventory from following the state is added to the apply error, and
// the loop stops on it. The runner writes the node state to its file before the apply,
// so a state that cannot be read, or is empty, leaves no proof the address stayed the same.
func (c *MasterNodeGroupController) afterFailedApply(ctx *context.Context, nodeName string, getState func() ([]byte, error), applyErr error) error {
	partialState, err := getState()
	if err != nil {
		return errors.Join(applyErr, fmt.Errorf("%w: cannot read infrastructure state of node %s after the failed apply: %w", errMasterHostsRefreshFailed, nodeName, err))
	}

	if len(partialState) == 0 {
		return errors.Join(applyErr, fmt.Errorf("%w: infrastructure state of node %s is empty after the failed apply", errMasterHostsRefreshFailed, nodeName))
	}

	c.state.State[nodeName] = partialState

	// With the client lost, the provider hands out one built from the connection flags:
	// refreshing through it would switch the wrong client. An interrupted run refreshes
	// nothing either: the next one starts from the states.
	if errors.Is(applyErr, context.ErrSSHClientSwitchFailed) || tomb.IsInterrupted() {
		return applyErr
	}

	if err := c.refreshMasterHostsFromStates(ctx); err != nil {
		return errors.Join(applyErr, err)
	}

	return applyErr
}

// newHookForUpdatePipeline reports its failures instead of returning a nil hook:
// the runner substitutes a DummyHook for a nil one, and a master VM would then be
// recreated without removing its control plane role, its Node object or its etcd
// membership. The one exception is a run with SSH configured but no hosts to reach.
func (c *MasterNodeGroupController) newHookForUpdatePipeline(ctx *context.Context, convergedNode string, metaConfig *config.MetaConfig) (infrastructure.InfraActionHook, error) {
	// A kube-flag run with an SSH configuration and no hosts (converge-migration by hand, or
	// converge with DHCTL_CLI_NO_SWITCH_TO_NODE_USER) cannot check or switch masters. 1.77
	// has always run it with a DummyHook, because GetSSHProvider failed and the hook was
	// dropped. Building a hook there instead turns a destructive plan into a scale dance
	// that the run cannot finish without SSH.
	if ctx.SSHProviderInitializer != nil && !ctx.SSHProviderInitializer.CheckHosts(ctx.Ctx()) {
		dhlog.FromContext(ctx.Ctx()).WarnContext(ctx.Ctx(), fmt.Sprintf("No SSH hosts to check control-plane nodes with. Node %s is converged without control-plane checks.", convergedNode))
		return nil, nil
	}

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

	sshProvider, err := ctx.SSHProviderInitializer.GetSSHProvider(ctx.Ctx())
	if err != nil {
		return nil, fmt.Errorf("get ssh provider: %w", err)
	}

	return controlplane.NewHookForUpdatePipeline(ctx, sshProvider, ctx.SSHProviderInitializer.GetSettings(), nodesToCheck, metaConfig.UUID, ctx.CommanderMode(), c.skipChecks).
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
		// The masters to delete leave the inventory before their destroys start. The destroy
		// hook takes a master's control-plane role away first, and a failover must not land
		// on it, whether its destroy then succeeds or fails. SwitchWhenDecreaseMastersIfNeed
		// has moved the client to a surviving master by now. Excluded masters are not
		// destroyed and stay. A copy: deleteRedundantNodes ranges over the same array.
		c.nodesToDelete = slices.DeleteFunc(slices.Clone(c.nodesToDelete), func(node nodeToDeleteInfo) bool {
			_, excluded := c.excludedNodes[node.name]
			return !excluded
		})
		if err := c.refreshMasterHostsFromStates(ctx); err != nil {
			return err
		}

		return c.deleteRedundantNodes(ctx, c.state.Settings, nodesToDeleteInfo, func(nodeName string) infrastructure.InfraActionHook {
			return controlplane.NewHookForDestroyPipeline(ctx, nodeName, ctx.CommanderMode())
		})
	})
}

// rememberMasterIPs hands the switcher the addresses the apply has already read from the
// new state, so the refresh after it runs no "output" for this master.
func (c *MasterNodeGroupController) rememberMasterIPs(ctx *context.Context, nodeName string, outputs *infrastructure.PipelineOutputs) {
	clientSwitcher := ctx.ClientSwitcher()
	if govalue.IsNil(clientSwitcher) || outputs == nil {
		return
	}

	clientSwitcher.RememberMasterIPs(nodeName, outputs.InfrastructureState, outputs.MasterIPForSSH, outputs.NodeInternalIP)
}

func (c *MasterNodeGroupController) refreshMasterHostsFromStates(ctx *context.Context) error {
	clientSwitcher := ctx.ClientSwitcher()
	if govalue.IsNil(clientSwitcher) {
		dhlog.FromContext(ctx.Ctx()).DebugContext(ctx.Ctx(), "Skipping refresh of master SSH hosts. Got empty client switcher")
		return nil
	}

	hosts, err := clientSwitcher.RefreshMasterHostsFromStates(ctx.Ctx(), c.inventoryStates())
	if err != nil {
		return fmt.Errorf("%w: %w", errMasterHostsRefreshFailed, err)
	}

	c.syncNodeToHost(hosts)

	return nil
}

// inventoryStates are the states of every master that still serves. The masters to delete
// are out of c.state.State from the start of the run, yet they stay failover hosts and
// stay in the saved hosts until deleteNodes starts destroying them: the deletion can still
// be dismissed. The client connected to one of them keeps its connection until
// SwitchWhenDecreaseMastersIfNeed moves it.
func (c *MasterNodeGroupController) inventoryStates() map[string][]byte {
	if len(c.nodesToDelete) == 0 {
		return c.state.State
	}

	// A name in both is a master added again in this run, by the scale to three masters
	// that updateNode can start: its state in c.state.State is the live one.
	states := maps.Clone(c.state.State)
	for _, node := range c.nodesToDelete {
		if _, ok := states[node.name]; !ok {
			states[node.name] = node.state
		}
	}

	return states
}

// syncNodeToHost brings the addresses the readiness checks reach the other masters by in
// line with the refreshed inventory. Only the masters already listed are touched. A master
// missing from the inventory has no address in its state right now: dropping it would let
// the checks skip a master that is down, and refilling the map from scratch would ask the
// user to confirm hosts again. The readiness checker opens its connection to the address
// in this map on every check, so updating the map is enough.
func (c *MasterNodeGroupController) syncNodeToHost(hosts []session.Host) {
	for _, host := range hosts {
		if old, ok := c.nodeToHost[host.Name]; ok && old != host.Host {
			c.nodeToHost[host.Name] = host.Host
		}
	}
}

func (c *MasterNodeGroupController) totalReplicas() int {
	return len(c.state.State)
}
