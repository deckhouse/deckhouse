// Copyright 2026 Flant JSC
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

package bootstrap

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	dhctlkube "github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

// immutableBootstrap is what this path carries between phases. A node on it
// runs no sshd and no bashible: dhctl hands it a cloud-init payload and then
// only talks to its API, over a bastion tunnel when there is one.
type immutableBootstrap struct {
	masterNodeName string
	// masterIP is where that API answers; the BaseInfra phase reports it.
	masterIP string
	// hosts is node name to address, from --master-host. Empty in a cloud, where
	// the infrastructure reports the addresses instead.
	hosts map[string]string
	// customizations is what the operator wrote about each machine, by node name.
	customizations map[string]immutable.Customization
	// kubeconfigPath is where the collected admin credentials were written, empty
	// until the node has handed them over.
	kubeconfigPath string
	tunnelStop     func()
}

// stopImmutableTunnel closes the bastion tunnel the path opened, if it opened one.
func (c *bootstrapContext) stopImmutableTunnel() {
	if c.immutable == nil || c.immutable.tunnelStop == nil {
		return
	}
	c.immutable.tunnelStop()
}

// printCollectedKubeconfig says how to reach the cluster once the node has
// handed its credentials over. Silent until then, and on every other path.
func (b *ClusterBootstrapper) printCollectedKubeconfig(ctx context.Context, bctx *bootstrapContext) {
	if bctx.immutable == nil || bctx.immutable.kubeconfigPath == "" {
		return
	}
	b.printHowToReachTheCluster(ctx, bctx.immutable.kubeconfigPath, bctx)
}

// detectImmutableMaster decides how the very first node is created, so it runs
// before anything touches the infrastructure.
func (b *ClusterBootstrapper) detectImmutableMaster(ctx context.Context, bctx *bootstrapContext) error {
	if !immutable.IsImmutableMaster(ctx, bctx.metaConfig) {
		return nil
	}

	// The documents describe machines the installer talks to, not objects to
	// create, so they leave the resources before anything else reads them.
	documents, rest := splitNodeCustomizations(bctx.metaConfig.ResourcesYAML)
	bctx.metaConfig.ResourcesYAML = rest

	customizations, err := immutable.ParseCustomizations(ctx, documents)
	if err != nil {
		return err
	}

	hosts, err := immutable.ParseHosts(b.Options.Bootstrap.MasterHostsRaw)
	if err != nil {
		return err
	}

	// Refused here rather than by a preflight: preflights can be skipped, and
	// the bootstrap this guards runs every phase to the end before dying on a
	// master address a non-cloud cluster never reports.
	if err := immutable.ValidateInputs(ctx, bctx.metaConfig, hosts); err != nil {
		return err
	}
	if err := refuseSharedAddresses(hosts); err != nil {
		return err
	}
	if err := b.refuseSSHHostOnImmutableStatic(ctx, bctx.metaConfig); err != nil {
		return err
	}

	matched, err := customizationsByNode(bctx.metaConfig, hosts, customizations)
	if err != nil {
		return err
	}

	dhlog.FromContext(ctx).InfoContext(ctx, "Master NodeGroup asks for an immutable system: bootstrapping without SSH and bashible")
	bctx.immutable = &immutableBootstrap{
		masterNodeName: firstImmutableMasterName(bctx.metaConfig, b.Options.Bootstrap.MasterHostsRaw),
		hosts:          hosts,
		customizations: matched,
	}

	return nil
}

// firstImmutableMasterName is the node the cluster starts on: the prefix names
// it in a cloud, and the operator's first --master-host where there is no
// prefix. ParseHosts refused a nameless pair, ValidateInputs an empty list.
func firstImmutableMasterName(metaConfig *config.MetaConfig, raw []string) string {
	if metaConfig.ClusterType == config.CloudClusterType {
		return firstMasterNodeName(metaConfig)
	}
	name, _ := immutable.ParseHost(raw[0])
	return name
}

// remainingMasterNames are the machines that join the cluster the first master
// started, in a stable order: etcd takes members one at a time, and a bootstrap
// that picks a different order on a rerun is one nobody can follow.
func remainingMasterNames(bctx *bootstrapContext) []string {
	names := slices.Sorted(maps.Keys(bctx.immutable.hosts))
	return slices.DeleteFunc(names, func(name string) bool { return name == bctx.immutable.masterNodeName })
}

// refuseSharedAddresses refuses two node names pointing at one machine: the
// second payload would reach a machine already installed as the first node, and
// the cluster would then wait for a master nobody configured.
func refuseSharedAddresses(hosts map[string]string) error {
	// Sorted so the pair named in the message is the same on every run.
	named := make(map[string]string, len(hosts))
	for _, name := range slices.Sorted(maps.Keys(hosts)) {
		if taken, shared := named[hosts[name]]; shared {
			return fmt.Errorf("--master-host points both %s and %s at %s; every master needs a machine of its own",
				taken, name, hosts[name])
		}
		named[hosts[name]] = name
	}

	return nil
}

// staticBootstrapNeedsSSHHost reports whether the bootstrap has to be given an
// SSH host. An immutable machine runs no sshd and is named by --master-host
// instead, so the usual demand for one sends every later phase at a dead port.
func staticBootstrapNeedsSSHHost(ctx context.Context, metaConfig *config.MetaConfig) bool {
	if !metaConfig.IsStatic() {
		return false
	}
	return !immutable.IsImmutableMaster(ctx, metaConfig)
}

// refuseSSHHostOnImmutableStatic refuses an SSH host given alongside
// --master-host: with one set the static preflights reach for sshd, and they
// run after the first master has already been handed its configuration.
func (b *ClusterBootstrapper) refuseSSHHostOnImmutableStatic(ctx context.Context, metaConfig *config.MetaConfig) error {
	if metaConfig.ClusterType == config.CloudClusterType {
		return nil
	}
	if !b.SSHProviderInitializer.CheckHosts(ctx) {
		return nil
	}

	return errors.New(
		"an SSH host is configured, but the machines of an immutable cluster are named by --master-host and answer no sshd: " +
			"drop --ssh-host, or the SSHHost resource of --connection-config")
}

// customizationsByNode pairs the operator's documents with the machines they
// describe. In a cloud there are no machines to name: the master NodeGroup's
// instanceClass describes them, so a document there is in the wrong file.
func customizationsByNode(metaConfig *config.MetaConfig, hosts map[string]string, customizations []immutable.Customization) (map[string]immutable.Customization, error) {
	if metaConfig.ClusterType != config.CloudClusterType {
		return matchCustomizationsToHosts(hosts, customizations)
	}
	if len(customizations) > 0 {
		return nil, fmt.Errorf(
			"the configuration describes node %s, but in a cloud the machines are described by the instanceClass of the master NodeGroup: drop the document",
			customizations[0].NodeName)
	}

	return nil, nil
}

// matchCustomizationsToHosts pairs what the operator wrote about a machine with
// where that machine is. A document nobody named is a typo in the node name,
// and letting it pass would boot the machine with defaults it was written to
// replace.
func matchCustomizationsToHosts(hosts map[string]string, customizations []immutable.Customization) (map[string]immutable.Customization, error) {
	matched := make(map[string]immutable.Customization, len(customizations))

	for _, customization := range customizations {
		if _, named := hosts[customization.NodeName]; !named {
			return nil, fmt.Errorf(
				"the configuration describes node %s, but no --master-host names it: add --master-host %s=<address>, or drop the document",
				customization.NodeName, customization.NodeName)
		}
		// Keyed by node name, so a second document about one machine would replace
		// the first without a word, and the machine boots with the half nobody
		// meant. Same refusal as two --master-host values for one name.
		if _, described := matched[customization.NodeName]; described {
			return nil, fmt.Errorf("the configuration describes node %s twice: keep one document per machine", customization.NodeName)
		}
		matched[customization.NodeName] = customization
	}

	return matched, nil
}

// applyImmutablePreflights adds the checks that only apply to an immutable
// master and drops the ones that reach the master over SSH, which it does not
// answer.
func (b *ClusterBootstrapper) applyImmutablePreflights(runner *preflight.Preflight, bctx *bootstrapContext) {
	if bctx.immutable == nil {
		return
	}

	runner.AddSuite(suites.NewImmutableSuite(suites.ImmutableDeps{
		MetaConfig:    bctx.metaConfig,
		BootstrapOpts: &b.Options.Bootstrap,
		GlobalOpts:    &b.Options.Global,
		CommanderMode: b.CommanderMode,
	}))

	// The cloud API check tunnels through the master host; there is no sshd
	// there to tunnel with.
	runner.DisableCheck(checks.CloudAPICheckName.String())

	// These reach the node over SSH, and without an SSH host their helper falls
	// back to the machine dhctl runs on (system/helper/helper.go). An immutable
	// node answers neither, so a check against the installer is worse than none.
	runner.DisableChecks(
		checks.SudoAllowedCheckName.String(),
		checks.HostNetworkCIDRIntersectionCheckName.String(),
		checks.DeckhouseUserCheckName.String(),
		checks.StaticSystemRequirementsCheckName.String(),
		checks.PythonCheckName.String(),
		checks.PortsCheckName.String(),
		checks.LocalhostDomainCheckName.String(),
		checks.TimeDriftCheckName.String(),
	)
}

// buildImmutableMasterPayload renders the cloud-init the first master boots
// with, and "" on every other path. A progress line of its own: it decides what
// the machine will be, and it runs before the machine exists.
func (b *ClusterBootstrapper) buildImmutableMasterPayload(ctx context.Context, bctx *bootstrapContext, nodeName string) (string, []byte, error) {
	if bctx.immutable == nil {
		return "", nil, nil
	}

	var (
		cloudConfig string
		nodeConfig  []byte
	)

	err := dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Prepare immutable master payload", func(ctx context.Context) error {
		var err error
		cloudConfig, nodeConfig, err = immutable.BuildMasterPayload(ctx, immutable.MasterPayloadInput{
			NodeName:      nodeName,
			MetaConfig:    bctx.metaConfig,
			StateCache:    bctx.stateCache,
			CandiDir:      b.Options.Global.CandiDir,
			GlobalOptions: &b.Options.Global,
			Customization: immutableCustomization(bctx, nodeName),
		})
		return err
	})

	return cloudConfig, nodeConfig, err
}

// immutableCustomization is what the operator wrote about this machine, and nil
// when nothing was written: only a nil tells the render to keep every value it
// put there itself.
func immutableCustomization(bctx *bootstrapContext, nodeName string) *immutable.Customization {
	described, ok := bctx.immutable.customizations[nodeName]
	if !ok {
		return nil
	}
	return new(described)
}

// addressAfterInstall is where the machine answers once it has installed itself:
// the push address, unless its document moves it to a static one. Everything
// after the push — the handoff channel and the apiserver — goes there.
func addressAfterInstall(c *immutable.Customization, pushAddress string) string {
	if c == nil {
		return pushAddress
	}
	// The document's own nodeIP wins: the machine check has confirmed it is one of
	// the addresses this document gives it, and on a machine with several NICs it
	// is not necessarily the first one.
	if nodeIP := c.NodeIP(); nodeIP != "" {
		return nodeIP
	}
	for _, iface := range c.Interfaces() {
		if iface.DHCP || len(iface.Addresses) == 0 {
			continue
		}
		host, _, err := net.ParseCIDR(iface.Addresses[0])
		if err != nil {
			// ParseCustomizations refuses an address without a prefix length, so this
			// is unreachable from a document — and the push address is where the
			// machine answers now, which beats killing the installer mid-run.
			return pushAddress
		}
		return host.String()
	}
	return pushAddress
}

// bootstrapImmutableFirstMaster hands the machine the cluster starts on its
// payload. This is what the cloud path gets for free: there terraform carries
// the same document into the machine it creates.
func (b *ClusterBootstrapper) bootstrapImmutableFirstMaster(ctx context.Context, bctx *bootstrapContext, port int) error {
	nodeName := bctx.immutable.masterNodeName

	// The name is read out of the same --master-host list as the addresses, so
	// this only fires if the two stop being read the same way — and an empty
	// address means pushing at whatever answers on the machine dhctl runs on.
	address, named := bctx.immutable.hosts[nodeName]
	if !named {
		return fmt.Errorf("no --master-host names the first master %s", nodeName)
	}

	// The cloud path prints this from the infrastructure output; on bare metal
	// nothing else says which of the machines in the room dhctl is configuring.
	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("First master: %s at %s (no SSH access)", nodeName, address))

	// The machine is configured at one address and, when its document gives it a
	// static one, answers at another from the moment it has installed itself.
	installedAddress := addressAfterInstall(immutableCustomization(bctx, nodeName), address)
	if installedAddress != address {
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(
			"%s takes the static address %s its document assigns it; everything after the push goes there, not to %s",
			nodeName, installedAddress, address))
	}

	// Every phase replays on a rerun and the push is not idempotent: a machine
	// that already has its configuration answers the next one as an installed
	// node, which is terminal. Only this record tells that from a wrong address.
	pushed, err := payloadAlreadyPushed(ctx, bctx.stateCache, nodeName, address)
	if err != nil {
		return err
	}
	if pushed {
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(
			"An earlier attempt already handed %s its configuration; continuing with the node at %s", nodeName, installedAddress,
		))
		bctx.immutable.masterIP = installedAddress
		return nil
	}

	payload, nodeConfig, err := b.buildImmutableMasterPayload(ctx, bctx, nodeName)
	if err != nil {
		return err
	}
	document, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("decode the payload of %s: %w", nodeName, err)
	}

	if err := b.pushImmutablePayload(ctx, nodeName, address, port, document, nodeConfig); err != nil {
		return err
	}
	if err := savePushedPayload(ctx, bctx.stateCache, nodeName, address); err != nil {
		return err
	}

	bctx.immutable.masterIP = installedAddress
	return nil
}

// bootstrapImmutableAdditionalMasters gives the rest of the control plane its
// payloads, one machine at a time: each carries the current bootstrap token and
// the apiservers that answer now, so the payload is rendered per node and only
// when its turn comes.
func (b *ClusterBootstrapper) bootstrapImmutableAdditionalMasters(ctx context.Context, bctx *bootstrapContext, kubeCl *client.KubernetesClient, port int) error {
	for _, nodeName := range remainingMasterNames(bctx) {
		if err := b.handImmutableJoinPayload(ctx, bctx, kubeCl, nodeName, port); err != nil {
			return err
		}

		// etcd takes members one at a time, so the next machine is only handed
		// anything once this one is a working control-plane member.
		if err := waitForImmutableMasterControlPlane(ctx, kubeCl, nodeName); err != nil {
			return err
		}
	}

	return nil
}

// handImmutableJoinPayload renders what this machine joins the running cluster
// with and hands it over, unless an earlier attempt already did: a machine that
// has its configuration answers the next one as an installed node, terminally.
func (b *ClusterBootstrapper) handImmutableJoinPayload(ctx context.Context, bctx *bootstrapContext, kubeCl *client.KubernetesClient, nodeName string, port int) error {
	// The name came out of the hosts map, so the address is there.
	address := bctx.immutable.hosts[nodeName]

	pushed, err := payloadAlreadyPushed(ctx, bctx.stateCache, nodeName, address)
	if err != nil {
		return err
	}
	if pushed {
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(
			"An earlier attempt already handed %s its configuration; continuing with the node at %s", nodeName, address,
		))
		return nil
	}

	payload, nodeConfig, err := buildImmutableJoinPayload(ctx, kubeCl, bctx.metaConfig, nodeName, immutableCustomization(bctx, nodeName))
	if err != nil {
		return err
	}
	document, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("decode the payload of %s: %w", nodeName, err)
	}

	if err := b.pushImmutablePayload(ctx, nodeName, address, port, document, nodeConfig); err != nil {
		return err
	}

	return savePushedPayload(ctx, bctx.stateCache, nodeName, address)
}

// pushedPayloadCacheKey names the record of the machine that already took this
// node's payload. Per node: every master is pushed to, and one shared key would
// let the last record answer for all of them. Named like the other keys of this
// path (pkg/immutable/constants.go).
func pushedPayloadCacheKey(nodeName string) string {
	return "immutable-control-plane-pushed-payload-" + nodeName
}

// pushRecord is what the state cache holds: which machine took which node's
// payload. The address is part of it so a rerun with a corrected --master-host
// pushes again instead of waiting on a machine that never got anything.
func pushRecord(nodeName, address string) string {
	return nodeName + "=" + address
}

// savePushedPayload records the machine that took the payload. Returned rather
// than warned: without the record every rerun walks into the terminal refusal
// of an installed node, and nothing on disk says why.
func savePushedPayload(ctx context.Context, stateCache state.Cache, nodeName, address string) error {
	if err := stateCache.Save(ctx, pushedPayloadCacheKey(nodeName), []byte(pushRecord(nodeName, address))); err != nil {
		return fmt.Errorf("record the configuration handed to %s in the state cache: %w", nodeName, err)
	}
	return nil
}

// payloadAlreadyPushed reports that this very machine took this node's payload
// in an earlier attempt.
func payloadAlreadyPushed(ctx context.Context, stateCache state.Cache, nodeName, address string) (bool, error) {
	key := pushedPayloadCacheKey(nodeName)

	inCache, err := stateCache.InCache(ctx, key)
	if err != nil {
		return false, fmt.Errorf("look up %s in the state cache: %w", key, err)
	}
	if !inCache {
		return false, nil
	}

	recorded, err := stateCache.Load(ctx, key)
	if err != nil {
		return false, fmt.Errorf("load %s from the state cache: %w", key, err)
	}
	return string(recorded) == pushRecord(nodeName, address), nil
}

// pushImmutablePayload waits for the machine to open its maintenance port and
// hands it document, the cloud-init. The wait is generous: the machine may still
// be POSTing. nodeConfig is the document inside it, checked against the hardware.
func (b *ClusterBootstrapper) pushImmutablePayload(ctx context.Context, nodeName, address string, port int, document, nodeConfig []byte) error {
	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), fmt.Sprintf("Hand %s its configuration", nodeName), func(ctx context.Context) error {
		// A channel per attempt: this wait starts while the machine is still
		// powering on, so an early dial hangs to gossh's deadline, which ends the
		// tunnel's accept loop for good and leaves a bound port nobody serves.
		openChannel := func() (string, func(), error) {
			return b.openImmutableChannelTo(ctx, address, port, "maintenance")
		}
		loop := libretry.NewLoop(fmt.Sprintf("Waiting for %s to ask for a configuration", nodeName), waitMaintenancePort.attempts, waitMaintenancePort.interval).
			BreakIf(pushGaveUp)

		return retryWithFreshChannel(ctx, loop, openChannel, func(endpoint string) error {
			// The check is inside the loop, because the loop is the wait: a machine
			// still powering on answers nothing to check against, and the attempt
			// that reaches it is the last moment before it takes the document.
			if err := checkMachineAgainstDocument(ctx, endpoint, nodeConfig); err != nil {
				return err
			}
			return immutable.PushNodeConfig(ctx, endpoint, document)
		})
	})
}

// checkMachineAgainstDocument refuses a NodeConfig the machine cannot satisfy,
// while nothing is installed yet. A machine that cannot be asked — an older image,
// a port not open — is a check nobody can run, not a bootstrap to fail.
func checkMachineAgainstDocument(ctx context.Context, address string, nodeConfig []byte) error {
	inventory, err := immutable.FetchInventory(ctx, address)
	if err != nil {
		// Only when the machine answered. A connection that goes nowhere is a
		// machine still powering on, which is the normal case here, and this
		// attempt's own push failure reports it a line later.
		if errors.Is(err, immutable.ErrInventoryUnusable) {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
				"%v; the document is not checked against the machine", err))
		}
		return nil
	}
	if inventory == nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
			"the machine at %s serves no inventory (an older image); the document is not checked against it", address))
		return nil
	}

	if err := immutable.CheckDocumentAgainstInventory(ctx, nodeConfig, inventory); err != nil {
		return fmt.Errorf("%w: %w", errDocumentUnfitForMachine, err)
	}
	return nil
}

// errDocumentUnfitForMachine marks the refusal so the wait ends on it: the
// machine will not grow another disk while the loop retries, and the operator
// has to read the refusal now, not ten minutes from now.
var errDocumentUnfitForMachine = errors.New("the configuration does not fit this machine")

// pushGaveUp reports the answers no waiting changes: the port is held by the
// agent of a node that is installed already and may not be handed a second
// configuration, or the document contradicts the hardware of the machine.
func pushGaveUp(err error) bool {
	return errors.Is(err, immutable.ErrMaintenanceTokenRequired) || errors.Is(err, errDocumentUnfitForMachine)
}

// connectToImmutableMaster collects the cluster credentials from the node and
// hands the rest of the bootstrap a Kubernetes client. No bashible pipeline:
// dhctl never sees a cluster key until the node gives it one.
func (b *ClusterBootstrapper) connectToImmutableMaster(ctx context.Context, bctx *bootstrapContext) error {
	if bctx.immutable.masterIP == "" {
		return errors.New("the first master address is unknown: rerun the bootstrap so the BaseInfra phase reports it")
	}

	complete, err := b.reuseCollectedKubeconfig(ctx, bctx)
	if err != nil {
		return err
	}

	// The tunnel behind it stays open for the rest of the bootstrap.
	address, stop, err := b.openImmutableChannel(ctx, bctx, immutable.APIServerPort, "API server")
	if err != nil {
		return err
	}
	bctx.immutable.tunnelStop = stop
	server := "https://" + address

	if complete == nil {
		complete, err = b.collectImmutableCredentials(ctx, bctx)
		if err != nil {
			return err
		}
	}

	content, err := immutable.RetargetKubeconfig(ctx, complete, server, bctx.immutable.masterNodeName)
	if err != nil {
		return err
	}

	kubeconfigPath, err := b.writeImmutableKubeconfig(ctx, b.TmpDir, content)
	if err != nil {
		return err
	}

	kubeProvider, err := newKubeconfigKubeProvider(ctx, b, kubeconfigPath)
	if err != nil {
		return err
	}
	b.KubeProvider = kubeProvider

	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return fmt.Errorf("connect to the API server of the immutable master at %s: %w", server, err)
	}

	// The credentials are stored and proven to work, so the node may stop
	// offering them. Attempted on a rerun too: a node that was already told
	// answers that it was.
	b.confirmImmutableHandoff(ctx, bctx)

	// Read exactly once, here — no later Client() call rebuilds from it. In
	// dhctl-server the process outlives the bootstrap, so leaving it to the
	// shutdown hook means admin credentials on disk for hours.
	removeImmutableKubeconfig(ctx, kubeconfigPath)

	return waitForImmutableMasterNode(ctx, kubeCl, bctx.immutable.masterNodeName)
}

// waitForImmutableMasterControlPlane waits until control-plane-manager reports
// this master ready, etcd member included: the Node appears when kubelet starts,
// long before that, and the machine has its whole install ahead of it.
func waitForImmutableMasterControlPlane(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string) error {
	checker := controlplane.NewManagerReadinessChecker(dhctlkube.NewSimpleKubeClientGetter(kubeCl))

	return libretry.NewLoop(fmt.Sprintf("Waiting for the control plane of %s", nodeName), waitJoinedControlPlane.attempts, waitJoinedControlPlane.interval).
		RunContext(ctx, func() error {
			ready, err := checker.IsReady(ctx, nodeName)
			if err != nil {
				return fmt.Errorf("check the control plane of %s: %w", nodeName, err)
			}
			if !ready {
				return fmt.Errorf("the control plane of %s is not ready yet", nodeName)
			}
			return nil
		})
}

// reuseCollectedKubeconfig returns the credentials an earlier attempt collected,
// or nil when there are none. A rerun does not come back through the channel:
// once an attempt has collected them the channel is closed and what it served is
// on disk, which beats waiting on a listener that is gone.
func (b *ClusterBootstrapper) reuseCollectedKubeconfig(ctx context.Context, bctx *bootstrapContext) ([]byte, error) {
	complete, collectedPath, err := adminKubeconfigFromCache(ctx, bctx.stateCache)
	if err != nil || collectedPath == "" {
		return nil, err
	}

	bctx.immutable.kubeconfigPath = collectedPath
	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(
		"A previous attempt already collected the credentials; reusing the admin kubeconfig at %s", collectedPath,
	))

	// The collecting branch is the only other writer of --kubeconfig-out and a
	// rerun skips it, so a path named for the first time on this run is written
	// here or nowhere. saveAdminKubeconfig says where it wrote them.
	out := b.Options.Bootstrap.KubeconfigOut
	if out != "" && out != collectedPath {
		return complete, b.saveAdminKubeconfig(ctx, complete, bctx)
	}

	// The other prints fire on a phase error and at the end of a successful run,
	// so a rerun going on to sit in a long wait would say nothing at all — as one
	// live rerun did. Guarded by TestConnectLineIsPrintedOnTheReusePath.
	b.printHowToReachTheCluster(ctx, collectedPath, bctx)

	return complete, nil
}

// waitForImmutableMasterNode waits until kubelet has registered the node. The
// node also creates the bootstrap RBAC, the control-plane label and taint and
// the d8-pki Secret on its own; dhctl creates none of them.
func waitForImmutableMasterNode(ctx context.Context, kubeCl kubernetes.Interface, nodeName string) error {
	return libretry.NewLoop(fmt.Sprintf("Waiting for the master node %s to register", nodeName), waitNodeRegistered.attempts, waitNodeRegistered.interval).
		RunContext(ctx, func() error {
			_, err := kubeCl.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("get node %s: %w", nodeName, err)
			}
			return nil
		})
}
