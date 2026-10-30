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

package context

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	statecache "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
)

type KubeClientSwitcher struct {
	ctx        *Context
	lockRunner *lock.InLockRunner
	params     KubeClientSwitcherParams

	// slogger is used for this switcher's own logging output.
	slogger *slog.Logger

	// keptUnknownHost is the host the user passed that no master state lists and that
	// switching disabled keeps: the refresh after every master warns about it only once.
	keptUnknownHost string

	// masterIPs caches addresses extracted from each master state. A refresh receives
	// all master states, but usually only one has changed, so unchanged states should
	// not invoke Terraform/OpenTofu output again.
	masterIPs map[string]masterIPsEntry
}

type masterIPsEntry struct {
	stateHash [sha256.Size]byte
	ips       infrastructure.OutputMasterIPs
}

type KubeClientSwitcherParams struct {
	TmpDir        string
	GlobalOptions *options.GlobalOptions
	IsDebug       bool
	DisableSwitch bool
}

func NewKubeClientSwitcher(ctx *Context, lockRunner *lock.InLockRunner, params KubeClientSwitcherParams) *KubeClientSwitcher {
	return &KubeClientSwitcher{
		ctx:        ctx,
		lockRunner: lockRunner,
		slogger:    dhlog.FromContext(ctx.Ctx()),
		params:     params,
	}
}

func (s *KubeClientSwitcher) SwitchToNodeUser(ctx context.Context, nodesState map[string][]byte) error {
	const action = "Switch clients to node user"

	if skip, err := s.isSkipOrLogStart(action, false); err != nil {
		return err
	} else if skip {
		return nil
	}

	return dhlog.RunProcess(ctx, s.slogger, action, func(ctx context.Context) error {
		convergeState, err := s.createNodeUser(ctx)
		if err != nil {
			return err
		}

		return s.replaceKubeClientForSwithToNodeUser(ctx, convergeState, nodesState)
	})
}

func (s *KubeClientSwitcher) CleanupNodeUser() error {
	const action = "Cleanup"

	if skip, err := s.isSkipOrLogStart(action, false); err != nil {
		return err
	} else if skip {
		return nil
	}

	// todo(ctx): does it's real need to use s.ctx.Ctx() instead of param context?
	return dhlog.RunProcess(s.ctx.Ctx(), s.slogger, action, func(ctx context.Context) error {
		err := s.ctx.deleteConvergeState()
		if err != nil {
			return err
		}

		c, cancel := s.ctx.WithTimeout(10 * time.Second)
		defer cancel()
		return entity.DeleteNodeUser(c, s.ctx, global.ConvergeNodeUserName)
	})
}

func (s *KubeClientSwitcher) SwitchToFirstMaster(ctx context.Context) error {
	const action = "Switch clients to first control-plane node"

	if skip, err := s.isSkipOrLogStart(action, true); err != nil {
		return err
	} else if skip {
		return nil
	}

	return dhlog.RunProcess(ctx, s.slogger, action, func(ctx context.Context) error {
		convergeState, err := s.ctx.ConvergeState()
		if err != nil {
			return fmt.Errorf("Cannot get converge state: %w", err)
		}

		firstMasterState, anotherMastersStates, err := s.extractStatesFromCluster(ctx)
		if err != nil {
			return err
		}

		if firstMasterState == nil {
			mastersNames := make([]string, 0, len(anotherMastersStates))
			for _, s := range anotherMastersStates {
				mastersNames = append(mastersNames, s.Name)
			}

			return fmt.Errorf(
				"Cannot find first control-plane node state or it is empty. Available states for [%s]",
				strings.Join(mastersNames, ", "),
			)
		}

		return s.replaceKubeClient(ctx, replaceKubeClientParams{
			convergeState: convergeState,
			state: map[string][]byte{
				firstMasterState.Name: firstMasterState.State,
			},
			appendPKey: nil,
		})
	})
}

func (s *KubeClientSwitcher) SwitchToNotFirstMaster(ctx context.Context) error {
	const action = "Switch clients to not first control-plane nodes"

	if skip, err := s.isSkipOrLogStart(action, true); err != nil {
		return err
	} else if skip {
		return nil
	}

	return dhlog.RunProcess(ctx, s.slogger, action, func(ctx context.Context) error {
		convergeState, err := s.ctx.ConvergeState()
		if err != nil {
			return fmt.Errorf("Cannot get converge state: %w", err)
		}

		firstMasterState, anotherMastersStates, err := s.extractStatesFromCluster(ctx)
		if err != nil {
			return err
		}

		statesMap := make(map[string][]byte)

		for _, s := range anotherMastersStates {
			statesMap[s.Name] = s.State
		}

		if len(statesMap) == 0 {
			if firstMasterState == nil {
				return fmt.Errorf("Cannot switch to another control-plane node: no states found")
			}

			s.warn("States for other control-plane nodes not found. Trying to continue with the first one")
			statesMap[firstMasterState.Name] = firstMasterState.State
		}

		return s.replaceKubeClient(ctx, replaceKubeClientParams{
			convergeState: convergeState,
			state:         statesMap,
			appendPKey:    nil,
		})
	})
}

func (s *KubeClientSwitcher) SwitchClientsToAnotherNodeIfNeed(ctx context.Context, nodeName, ip string) error {
	const action = "Switch clients on destructive change of control-plane nodes"

	if skip, err := s.isSkipOrLogStart(action, true); err != nil {
		return err
	} else if skip {
		return nil
	}

	sshClient, err := s.extractSSHClient(ctx)
	if err != nil {
		return err
	}

	s.debug("SwitchClientsToAnotherNodeIfNeed sshClient: %v", sshClient)
	currentHost := session.CurrentHost(sshClient.Session())
	if currentHost.Host == "" {
		return fmt.Errorf("Got an empty current host")
	}

	if nodeName != currentHost.Name {
		s.debug("Skipping %s: current host is not the deleted host '%s'", action, nodeName)
		return nil
	}

	return dhlog.RunProcess(ctx, s.slogger, action, func(ctx context.Context) error {
		convergeState, err := s.ctx.ConvergeState()
		if err != nil {
			return fmt.Errorf("Cannot get converge state: %w", err)
		}

		firstMaster, anotherMasters, err := s.extractStatesFromCluster(ctx)
		if err != nil {
			return err
		}

		statesMap := make(map[string][]byte)
		for _, s := range append([]*NodeState{firstMaster}, anotherMasters...) {
			if nodeName != s.Name {
				statesMap[s.Name] = s.State
			}
		}

		return s.replaceKubeClient(ctx, replaceKubeClientParams{
			convergeState: convergeState,
			state:         statesMap,
			appendPKey:    nil,
		})
	})
}

func (s *KubeClientSwitcher) SwitchWhenDecreaseMastersIfNeed(ctx context.Context, ngName string, nodesToDeleteInfo []*NodeState) error {
	const action = "Switch clients when decrease control-plane nodes"

	logSkip := func(f string, args ...any) {
		s.debug(fmt.Sprintf("Skipping %s: ", action)+f, args...)
	}

	if ngName != global.MasterNodeGroupName {
		logSkip("target node group '%s' is not master", ngName)
		return nil
	}

	if len(nodesToDeleteInfo) == 0 {
		logSkip("no nodes to delete")
		return nil
	}

	if skip, err := s.isSkipOrLogStart(action, true); err != nil {
		return err
	} else if skip {
		return nil
	}

	sshClient, err := s.extractSSHClient(ctx)
	if err != nil {
		return err
	}

	s.debug("SwitchWhenDecreaseMastersIfNeed sshClient: %v", sshClient)
	currentHost := session.CurrentHost(sshClient.Session())
	if currentHost.Host == "" {
		return fmt.Errorf("Got an empty current host")
	}

	needReconnect := false
	deletedHostsNames := make(map[string]struct{})

	for _, dhost := range nodesToDeleteInfo {
		dName := dhost.Name
		if currentHost.Name == dName {
			needReconnect = true
		}
		deletedHostsNames[dName] = struct{}{}
	}

	if !needReconnect {
		logSkip("use not deleted host as current")
		return nil
	}

	return dhlog.RunProcess(ctx, s.slogger, action, func(ctx context.Context) error {
		convergeState, err := s.ctx.ConvergeState()
		if err != nil {
			return fmt.Errorf("Cannot get converge state: %w", err)
		}

		firstMaster, anotherMasters, err := s.extractStatesFromCluster(ctx)
		if err != nil {
			return err
		}

		statesMap := make(map[string][]byte)
		for _, s := range append([]*NodeState{firstMaster}, anotherMasters...) {
			if _, ok := deletedHostsNames[s.Name]; !ok {
				statesMap[s.Name] = s.State
			}
		}

		return s.replaceKubeClient(ctx, replaceKubeClientParams{
			convergeState: convergeState,
			state:         statesMap,
			appendPKey:    nil,
		})
	})
}

type replaceKubeClientParams struct {
	convergeState *State
	state         map[string][]byte
	appendPKey    *session.AgentPrivateKey
}

// ErrSSHClientSwitchFailed marks a failure after the converge lock stopped being renewed
// and the SSH client was torn down. Such a run holds neither a working connection nor a
// renewed lock, so it must stop instead of changing the next node.
var ErrSSHClientSwitchFailed = errors.New("switching the SSH client failed")

// errIPExtraction marks a failed "output" of a single node state. Picking a host to
// connect to can do without that node; any other failure is not about the node.
var errIPExtraction = errors.New("cannot extract IPs")

// RefreshMasterHostsFromStates makes the Terraform states of the masters the SSH inventory
// and returns it. In Commander, which has no SSH client, only the persisted hosts cache is
// refreshed. The client keeps its connection while the machine it is connected to is still
// a master, and moves to the remaining ones only once that machine is gone. All addresses
// are extracted before anything changes: a failed extraction leaves the session and the
// saved master hosts as they were. Addresses already read stay in masterIPs, keyed by the
// state they were read from.
func (s *KubeClientSwitcher) RefreshMasterHostsFromStates(ctx context.Context, nodesState map[string][]byte) ([]session.Host, error) {
	if len(nodesState) == 0 {
		return nil, fmt.Errorf("Empty node states for refreshing control-plane SSH hosts")
	}

	// Commander has no SSH client; this cache is compatibility bookkeeping only and its
	// refresh must not turn an infrastructure operation into a failure.
	if s.ctx.CommanderMode() {
		hosts, err := s.masterHostsFromStates(ctx, nodesState, s.bastionHostFromConfig(), newLazySSHIPExtractor(s), true)
		if err != nil {
			s.warn("Cannot refresh control-plane hosts cache: %v", err)
			return nil, nil
		}

		// An empty list would replace every saved host, as the SSH path refuses to.
		if len(hosts) == 0 {
			s.warn("Cannot refresh control-plane hosts cache: the states have no control-plane addresses")
			return nil, nil
		}

		s.saveMasterHosts(ctx, hosts)
		return hosts, nil
	}

	// A run over the Kubernetes API alone, such as converge-migration in the auto-converger
	// pod, has no SSH inventory to rebuild. Asking for a provider there fails the converge.
	if !s.ctx.SSHProviderInitializer.CheckHosts(ctx) {
		s.debug("Refresh control-plane SSH hosts skipped: converge has no SSH hosts")
		return nil, nil
	}

	sshProvider, err := s.ctx.SSHProviderInitializer.GetSSHProvider(ctx)
	if err != nil {
		return nil, err
	}

	sshCl, err := sshProvider.Client(ctx)
	if err != nil {
		return nil, err
	}

	return s.refreshSSHHosts(ctx, sshProvider, sshCl, nodesState, newLazySSHIPExtractor(s))
}

func (s *KubeClientSwitcher) refreshSSHHosts(ctx context.Context, sshProvider libcon.SSHProvider, sshCl libcon.SSHClient, nodesState map[string][]byte, ipExtractor sshHostExtractor) ([]session.Host, error) {
	sess := sshCl.Session()

	hosts, err := s.masterHostsFromStates(ctx, nodesState, sess.BastionHost, ipExtractor, true)
	if err != nil {
		return nil, err
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("Cannot refresh control-plane SSH hosts: no available hosts found in node states")
	}

	currentHost := sess.Host()

	switch {
	case setHostsKeepingCurrent(sess, hosts):
		s.debug("Control-plane SSH hosts refreshed, the client stays on %s", currentHost)
	case s.params.DisableSwitch:
		// DHCTL_CLI_NO_SWITCH_TO_NODE_USER keeps the client as it is, on the host the user
		// passed. That can be a name or an address no state lists, of a machine that is
		// still there: it stays first, and the masters from the states are for reconnects.
		const msg = "SSH host %s is not found among control-plane addresses in the states, and switching clients is disabled: keeping it"
		if s.keptUnknownHost != currentHost {
			s.keptUnknownHost = currentHost
			s.warn(msg, currentHost)
		} else {
			s.debug(msg, currentHost)
		}

		if current := session.CurrentHost(sess); current.Host != "" {
			setHostsKeepingCurrent(sess, append([]session.Host{current}, hosts...))
		} else {
			sess.SetAvailableHosts(hosts)
		}
	default:
		s.debug("SSH host %s is not a control-plane node anymore, switching the client to the remaining ones", currentHost)
		if err := s.refreshSSHClient(ctx, sshProvider, sshCl, hosts); err != nil {
			return nil, err
		}
	}

	s.saveMasterHosts(ctx, hosts)
	return hosts, nil
}

func (s *KubeClientSwitcher) replaceKubeClient(ctx context.Context, params replaceKubeClientParams) error {
	if len(params.state) == 0 {
		return fmt.Errorf("Empty node states for replacing client")
	}

	if params.convergeState == nil {
		return fmt.Errorf("Internal error: empty converge state for replacing client")
	}

	sshProvider, err := s.ctx.SSHProviderInitializer.GetSSHProvider(ctx)
	if err != nil {
		return err
	}

	sshCl, err := sshProvider.Client(ctx)
	if err != nil {
		return err
	}

	settings := sshCl.Session()

	availableHosts, err := s.masterHostsFromStates(ctx, params.state, settings.BastionHost, newLazySSHIPExtractor(s), false)
	if err != nil {
		return err
	}

	if len(availableHosts) == 0 {
		return fmt.Errorf("Cannot switch clients: no available hosts found in node states")
	}

	sess := session.NewSession(session.Input{
		User:            params.convergeState.NodeUserCredentials.Name,
		Port:            settings.Port,
		BastionHost:     settings.BastionHost,
		BastionPort:     settings.BastionPort,
		BastionUser:     settings.BastionUser,
		BastionPassword: settings.BastionPassword,
		ExtraArgs:       settings.ExtraArgs,
		AvailableHosts:  availableHosts,
		BecomePass:      params.convergeState.NodeUserCredentials.Password,
	})

	pkeys := privateKeysForSwitch(sshCl.PrivateKeys(), params.appendPKey)

	return s.switchSSHClient(ctx, sshProvider, sess, pkeys)
}

type sshHostExtractor interface {
	masterIPs(ctx context.Context, params *sshIPExtractorParams) (infrastructure.OutputMasterIPs, error)
}

// lazySSHIPExtractor avoids preparing a temporary directory and an output executor when
// every address is already cached from an earlier state read or the apply outputs.
type lazySSHIPExtractor struct {
	switcher *KubeClientSwitcher
	delegate *sshIPExtractor
}

func newLazySSHIPExtractor(s *KubeClientSwitcher) *lazySSHIPExtractor {
	return &lazySSHIPExtractor{switcher: s}
}

func (e *lazySSHIPExtractor) masterIPs(ctx context.Context, params *sshIPExtractorParams) (infrastructure.OutputMasterIPs, error) {
	if e.delegate == nil {
		extractor, err := newSSHIPExtractor(e.switcher)
		if err != nil {
			return infrastructure.OutputMasterIPs{}, err
		}

		e.delegate = extractor
	}

	return e.delegate.masterIPs(ctx, params)
}

// masterHostsFromStates extracts the SSH address of every master from its state. A state
// without addresses is skipped: its machine is not there yet. With strict, a failed
// "output" fails the whole call, so the caller never acts on a partial inventory; without
// it the node is skipped, which is enough to pick a host to connect to.
func (s *KubeClientSwitcher) masterHostsFromStates(ctx context.Context, nodesState map[string][]byte, bastionHost string, ipExtractor sshHostExtractor, strict bool) ([]session.Host, error) {
	availableHosts := make([]session.Host, 0, len(nodesState))

	for nodeName, stateBytes := range nodesState {
		ips, err := s.masterIPsFromState(ctx, nodeName, stateBytes, ipExtractor)
		if err != nil {
			if strict || !errors.Is(err, errIPExtraction) {
				return nil, err
			}

			s.warn("%v\nSkipping adding node to ssh client", err)
			continue
		}

		if ips.SSH == "" && ips.Internal == "" {
			s.warn("IPs for node '%s' not found. Skipping adding node to ssh client", nodeName)
			continue
		}

		// Through a bastion only the internal address is routable.
		ip := ips.SSH
		if bastionHost != "" {
			ip = ips.Internal
		}

		s.debug("Using IP '%s' for node %s (bastion host: '%s')", ip, nodeName, bastionHost)

		if ip != "" {
			availableHosts = append(availableHosts, session.Host{Host: ip, Name: nodeName})
		}
	}

	sort.Sort(session.SortByName(availableHosts))
	return availableHosts, nil
}

// masterIPsFromState reads both addresses of a master from its state, once per state.
// The pair is kept rather than the address picked from it, for the pick depends on the
// bastion of the caller. A failure is not kept: the next call reads the state again.
func (s *KubeClientSwitcher) masterIPsFromState(ctx context.Context, nodeName string, state []byte, ipExtractor sshHostExtractor) (infrastructure.OutputMasterIPs, error) {
	stateHash := sha256.Sum256(state)
	if entry, ok := s.masterIPs[nodeName]; ok && entry.stateHash == stateHash {
		return entry.ips, nil
	}

	ips, err := ipExtractor.masterIPs(ctx, &sshIPExtractorParams{nodeName: nodeName, state: state})
	if err != nil {
		return infrastructure.OutputMasterIPs{}, err
	}

	s.rememberMasterIPs(nodeName, stateHash, ips)
	return ips, nil
}

// RememberMasterIPs keeps the addresses an apply has just read from the new state of a
// master, so the refresh that follows the apply does not read them from it again.
func (s *KubeClientSwitcher) RememberMasterIPs(nodeName string, state []byte, sshIP, internalIP string) {
	s.rememberMasterIPs(nodeName, sha256.Sum256(state), infrastructure.OutputMasterIPs{SSH: sshIP, Internal: internalIP})
}

func (s *KubeClientSwitcher) rememberMasterIPs(nodeName string, stateHash [sha256.Size]byte, ips infrastructure.OutputMasterIPs) {
	if s.masterIPs == nil {
		s.masterIPs = make(map[string]masterIPsEntry)
	}

	s.masterIPs[nodeName] = masterIPsEntry{stateHash: stateHash, ips: ips}
}

// refreshSSHClient moves the client to the given hosts, for the one it is connected to is
// gone. Everything else about the session stays: the same user reaches the same kind of
// machine through the same bastion.
func (s *KubeClientSwitcher) refreshSSHClient(ctx context.Context, sshProvider libcon.SSHProvider, sshCl libcon.SSHClient, hosts []session.Host) error {
	sess := sshCl.Session().Copy()
	sess.SetAvailableHosts(hosts)

	return s.switchSSHClient(ctx, sshProvider, sess, sshCl.PrivateKeys())
}

func (s *KubeClientSwitcher) switchSSHClient(ctx context.Context, sshProvider libcon.SSHProvider, sess *session.Session, privateKeys []session.AgentPrivateKey) error {
	// The lease is renewed through the kube client being replaced. Deleting it for that
	// time would let someone else take it, so only its renewal stops.
	if s.lockRunner != nil {
		s.lockRunner.PauseRenew()
	}

	s.debug("Creating new ssh client for replacing kube client")

	// Nothing renews or deletes the paused lease when the replacement cannot complete.
	// Explain why the next converge may have to wait for it to expire.
	lockLeft := func() {
		if s.lockRunner != nil {
			s.warn("The converge lock is no longer renewed and may stay in the cluster until its lease expires: the next converge may wait for it.")
		}
	}

	newSSHClient, err := sshProvider.SwitchClient(ctx, sess, privateKeys)
	if err != nil {
		lockLeft()
		return fmt.Errorf("%w: failed to start SSH client: %w", ErrSSHClientSwitchFailed, err)
	}

	s.debug("SSH client started for replacing kube client")

	if err := newSSHClient.RefreshPrivateKeys(ctx); err != nil {
		lockLeft()
		return fmt.Errorf("%w: failed to refresh ssh agent private keys: %w", ErrSSHClientSwitchFailed, err)
	}

	s.debug("Private keys refreshed for replacing kube client")

	if s.lockRunner != nil {
		s.debugStartOperation("reset lock after replacing kube client")

		if err := s.lockRunner.ResetLock(s.ctx.Ctx()); err != nil {
			lockLeft()
			return fmt.Errorf("%w: failed to reset lock: %w", ErrSSHClientSwitchFailed, err)
		}

		s.debug("lock was reset after replacing kube client")
	}

	return nil
}

func privateKeysForSwitch(current []session.AgentPrivateKey, additional *session.AgentPrivateKey) []session.AgentPrivateKey {
	privateKeys := append([]session.AgentPrivateKey(nil), current...)
	if additional != nil {
		privateKeys = append(privateKeys, *additional)
	}

	return privateKeys
}

// setHostsKeepingCurrent replaces the hosts of a live session and keeps the one it is
// connected to. SetAvailableHosts selects the first host, so the current one goes first:
// otherwise the session would name one machine while its connection is open to another.
// It reports false, leaving the session as it is, when the current host is not among hosts.
func setHostsKeepingCurrent(sess *session.Session, hosts []session.Host) bool {
	currentHost := sess.Host()
	i := slices.IndexFunc(hosts, func(h session.Host) bool { return h.Host == currentHost })
	if currentHost == "" || i < 0 {
		return false
	}

	ordered := append([]session.Host{hosts[i]}, slices.Delete(slices.Clone(hosts), i, i+1)...)
	sess.SetAvailableHosts(ordered)

	return true
}

func (s *KubeClientSwitcher) bastionHostFromConfig() string {
	if s.ctx.SSHProviderInitializer == nil {
		return ""
	}

	connectionConfig := s.ctx.SSHProviderInitializer.GetConfig()
	if connectionConfig == nil || connectionConfig.Config == nil {
		return ""
	}

	return connectionConfig.Config.BastionHost
}

func (s *KubeClientSwitcher) saveMasterHosts(ctx context.Context, hosts []session.Host) {
	if s.ctx.StateCache() == nil {
		return
	}

	hostsByName := make(map[string]string, len(hosts))
	for _, host := range hosts {
		hostsByName[host.Name] = host.Host
	}

	statecache.SaveMasterHostsToCache(ctx, s.ctx.StateCache(), hostsByName)
}

func (s *KubeClientSwitcher) tmpDirForConverger() (string, error) {
	tmpDir := filepath.Join(s.params.TmpDir, "converger")
	err := os.MkdirAll(tmpDir, 0o755)
	if err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("Failed to create tmp directory for converge: %w", err)
	}

	s.debug("Temp dir %s created for switching kube client", tmpDir)
	return tmpDir, nil
}

func (s *KubeClientSwitcher) createNodeUser(ctx context.Context) (*State, error) {
	convergeState, err := s.ctx.ConvergeState()
	if err != nil {
		return nil, err
	}

	if convergeState.NodeUserCredentials != nil {
		exists, err := entity.NodeUserExists(s.ctx.Ctx(), s.ctx, convergeState.NodeUserCredentials.Name)
		if err != nil {
			return nil, err
		}

		if exists {
			return convergeState, nil
		}

		s.warn(
			"NodeUser %q is missing while converge state exists; recreating NodeUser",
			convergeState.NodeUserCredentials.Name,
		)

		convergeState.NodeUserCredentials = nil

		if err := s.ctx.SetConvergeState(convergeState); err != nil {
			return nil, fmt.Errorf("Failed to reset stale node user credentials: %w", err)
		}
	}

	s.debugStartOperation("create node user")
	s.debug("Generate node user")

	nodeUser, nodeUserCredentials, err := v1.GenerateNodeUser(v1.ConvergerNodeUser())
	if err != nil {
		return nil, fmt.Errorf("Failed to generate NodeUser: %w", err)
	}

	err = entity.CreateOrUpdateNodeUser(s.ctx.Ctx(), s.ctx, nodeUser, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create or update NodeUser: %w", err)
	}

	// check ssh client
	_, err = s.extractSSHClient(ctx)
	if err != nil {
		return nil, err
	}

	err = entity.NewConvergerNodeUserExistsWaiter(s.ctx).WaitPresentOnNodes(ctx, nodeUserCredentials)
	if err != nil {
		return nil, fmt.Errorf("Could not ensure converger user is present on control plane hosts: %w", err)
	}

	convergeState.NodeUserCredentials = nodeUserCredentials

	err = s.ctx.SetConvergeState(convergeState)
	if err != nil {
		return nil, fmt.Errorf("Failed to set converge state: %w", err)
	}

	return convergeState, nil
}

func (s *KubeClientSwitcher) replaceKubeClientForSwithToNodeUser(ctx context.Context, convergeState *State, state map[string][]byte) error {
	s.debugStartOperation("call replaceKubeClientForSwithToNodeUser")

	tmpDir, err := s.tmpDirForConverger()
	if err != nil {
		return err
	}

	privateKeyPath := filepath.Join(tmpDir, "id_rsa_converger")

	privateKey := session.AgentPrivateKey{
		Key:        privateKeyPath,
		Passphrase: convergeState.NodeUserCredentials.Password,
	}

	err = os.WriteFile(privateKeyPath, []byte(convergeState.NodeUserCredentials.PrivateKey), 0o600)
	if err != nil {
		return fmt.Errorf("Failed to write private key for NodeUser: %w", err)
	}

	return s.replaceKubeClient(ctx, replaceKubeClientParams{
		convergeState: convergeState,
		state:         state,
		appendPKey:    &privateKey,
	})
}

type NodeState struct {
	Name  string
	State []byte
}

func (s *KubeClientSwitcher) extractStatesFromCluster(ctx context.Context) (*NodeState, []*NodeState, error) {
	const firstMasterSuffix = "-0"

	states, err := infrastructurestate.GetMasterNodesStateFromCluster(ctx, s.ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Cannot extract control-plane node states: %w", err)
	}

	var firstMasterState *NodeState
	anoterNodesStates := make([]*NodeState, 0, 2)

	for nodeName, state := range states {
		st := &NodeState{
			Name:  nodeName,
			State: state,
		}

		if strings.HasSuffix(nodeName, firstMasterSuffix) {
			s.debug("Found first master state %s", nodeName)
			firstMasterState = st
			continue
		}

		s.debug("Found another master state %s", nodeName)
		anoterNodesStates = append(anoterNodesStates, st)
	}

	if len(anoterNodesStates) > 0 {
		sort.Slice(anoterNodesStates, func(i, j int) bool {
			return anoterNodesStates[i].Name < anoterNodesStates[j].Name
		})
	}

	return firstMasterState, anoterNodesStates, nil
}

func (s *KubeClientSwitcher) inCommander(action string) bool {
	if s.ctx.CommanderMode() {
		s.debug("%s skipped. In commander mode", action)
		return true
	}

	return false
}

func (s *KubeClientSwitcher) switchDisbled(action string) bool {
	if s.params.DisableSwitch {
		s.warn("%s skipped. Switch disabled", action)
		return true
	}

	return false
}

func (s *KubeClientSwitcher) isSkipOrLogStart(action string, strict bool) (bool, error) {
	if s.inCommander(action) {
		return true, nil
	}

	if s.switchDisbled(action) {
		if strict {
			return true, fmt.Errorf("Internal error: disabling switch to node user was requested, but it is needed for %s", action)
		}

		return true, nil
	}

	s.debugStartOperation(action)

	return false, nil
}

func (s *KubeClientSwitcher) extractSSHClient(ctx context.Context) (libcon.SSHClient, error) {
	sshProvider, err := s.ctx.SSHProviderInitializer.GetSSHProvider(ctx)
	if err != nil {
		return nil, err
	}

	sshCl, err := sshProvider.Client(ctx)
	if err != nil {
		return nil, err
	}

	return sshCl, nil
}

func (s *KubeClientSwitcher) debug(f string, args ...any) {
	s.slogger.DebugContext(s.ctx.Ctx(), strings.TrimRight(fmt.Sprintf(f, args...), "\n"))
}

func (s *KubeClientSwitcher) warn(f string, args ...any) {
	s.slogger.WarnContext(s.ctx.Ctx(), strings.TrimRight(fmt.Sprintf(f, args...), "\n"))
}

func (s *KubeClientSwitcher) debugStartOperation(action string) {
	s.debug("Starting %s", strings.ToLower(action))
}

type sshIPExtractorParams struct {
	nodeName string
	state    []byte
}

type sshIPExtractor struct {
	switcher *KubeClientSwitcher
	tmpDir   string
	suffix   string
}

func newSSHIPExtractor(s *KubeClientSwitcher) (*sshIPExtractor, error) {
	tmpDir, err := s.tmpDirForConverger()
	if err != nil {
		return nil, err
	}

	suff := rand.NewSource(time.Now().UnixNano()).Int63()

	return &sshIPExtractor{
		switcher: s,
		tmpDir:   tmpDir,
		suffix:   fmt.Sprintf("%d", suff),
	}, nil
}

func (e *sshIPExtractor) masterIPs(ctx context.Context, params *sshIPExtractorParams) (infrastructure.OutputMasterIPs, error) {
	executor, err := e.getExecutor(ctx, params)
	if err != nil {
		return infrastructure.OutputMasterIPs{}, err
	}

	// do not cleanup provider after getting output executor!

	statePath, err := e.prepareState(params)
	if err != nil {
		return infrastructure.OutputMasterIPs{}, err
	}
	defer func() {
		if err := os.Remove(statePath); err != nil {
			e.switcher.debug("Cannot remove temporary state %s: %v", statePath, err)
		}
	}()

	addresses, err := infrastructure.GetMasterIPAddressForSSH(ctx, statePath, executor)
	if err != nil {
		return infrastructure.OutputMasterIPs{}, fmt.Errorf("%w for node '%s': %w", errIPExtraction, params.nodeName, err)
	}

	return *addresses, nil
}

func (e *sshIPExtractor) getExecutor(ctx context.Context, params *sshIPExtractorParams) (infrastructure.OutputExecutor, error) {
	nodeName := params.nodeName

	metaConfig, err := e.switcher.ctx.MetaConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get meta config for node %s: %w", nodeName, err)
	}

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           e.tmpDir,
		GlobalOptions:    e.switcher.params.GlobalOptions,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		IsDebug:          e.switcher.params.IsDebug,
	})

	// yes working dir for output is not required
	provider, err := providerGetter(ctx, metaConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to create executor for node %s: %w", nodeName, err)
	}

	executor, err := provider.OutputExecutor(ctx)
	if err != nil {
		return nil, fmt.Errorf("Cannot get output executor for node %s: %w", nodeName, err)
	}

	return executor, nil
}

func (e *sshIPExtractor) prepareState(params *sshIPExtractorParams) (string, error) {
	nodeName := params.nodeName

	statePath := filepath.Join(e.tmpDir, fmt.Sprintf("%s-%s.tfstate", nodeName, e.suffix))

	e.switcher.debug("State path for extracting IP for node %s: %s", nodeName, statePath)

	// the state can hold secrets of the cloud provider
	err := os.WriteFile(statePath, params.state, 0o600)
	if err != nil {
		return "", fmt.Errorf("Failed to write infrastructure state for %s in %s: %w", nodeName, statePath, err)
	}

	return statePath, nil
}

func (s *KubeClientSwitcher) GetGlobalOptions() *options.GlobalOptions {
	return s.params.GlobalOptions
}
