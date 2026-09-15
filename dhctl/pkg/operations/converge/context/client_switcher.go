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
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"

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
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
)

type KubeClientSwitcher struct {
	ctx        *Context
	lockRunner *lock.InLockRunner
	params     KubeClientSwitcherParams

	// slogger is used for this switcher's own logging output.
	slogger *slog.Logger
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

// CleanupConvergeUser removes the account this converge baked into the masters it built.
// A node left uncleaned is not a converge failure: the rollout is over by the time this
// runs and the account expires on its own, while failing here would report a finished
// converge as broken. The error is for the caller to keep the state with, not to abort on.
func (s *KubeClientSwitcher) CleanupConvergeUser(ctx context.Context) error {
	const action = "Remove the converge user from the nodes this converge built"

	// Gated by hand rather than by isSkipOrLogStart: that also skips a disabled switch, and
	// DHCTL_CLI_NO_SWITCH_TO_NODE_USER stops dhctl logging in as the account, not a new
	// master from booting with it.
	if s.inCommander(action) {
		return nil
	}

	if s.sshless(action) {
		return nil
	}

	s.debugStartOperation(action)

	err := dhlog.RunProcess(ctx, s.slogger, action, func(ctx context.Context) error {
		sshProvider, err := s.ctx.SSHProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return err
		}

		return s.removeConvergeUser(ctx, sshProvider)
	})
	if err == nil {
		return nil
	}

	s.warn(
		"Could not remove %s from every node this converge built:\n%v\nThose nodes still accept it "+
			"until the expiry date baked into the account, at most two days after this converge created "+
			"them. The next converge removes it from the ones still listed.",
		global.ConvergeUserName, err,
	)

	return err
}

// removeConvergeUser deletes the account node by node. One machine refusing must not save
// the account on the others, so the failures are collected and reported together.
func (s *KubeClientSwitcher) removeConvergeUser(ctx context.Context, sshProvider libcon.SSHProvider) error {
	convergeState, err := s.ctx.ConvergeState()
	if err != nil {
		return fmt.Errorf("get converge state: %w", err)
	}

	// An immutable control plane is never given the account, and neither is a converge
	// that rebuilt no master.
	if len(convergeState.ConvergeUserNodes) == 0 {
		s.debug("No node was built with %s, nothing to remove", global.ConvergeUserName)
		return nil
	}

	standalone, ok := sshProvider.(libcon.StandaloneClientProvider)
	if !ok {
		return fmt.Errorf("remove %s: the ssh provider opens no per-node connection", global.ConvergeUserName)
	}

	sshCl, err := sshProvider.Client(ctx)
	if err != nil {
		return fmt.Errorf("get ssh client: %w", err)
	}

	creds, err := s.credentialsFor(convergeState.ConvergeUserNodes)
	if err != nil {
		return err
	}

	addresses, err := s.masterAddresses(sshCl.Session())
	if err != nil {
		return err
	}

	var failures *multierror.Error

	for _, node := range cleanupOrder(convergeState.ConvergeUserNodes, addresses, sshCl.Session().Host()) {
		address := addresses[node]

		// Neither the session nor the hosts cache knows where this node is, so nothing of
		// ours can be reached on it. Left in the list it would fail every later cleanup,
		// and with it the removal of the NodeUser and the secret holding its key.
		if address == "" {
			s.warn(
				"No ssh address known for %s, so %s cannot be removed there; it stops accepting logins on its own at the expiry date it was created with",
				node, global.ConvergeUserName,
			)
		} else if err := removeConvergeUserOn(ctx, standalone, sshCl, creds, session.Host{Host: address, Name: node}); err != nil {
			failures = multierror.Append(failures, err)
			continue
		}

		// Written back node by node: a converge that stops here must not send the next
		// one to log in to this node as the account it has just removed.
		convergeState.ConvergeUserNodes = slices.DeleteFunc(convergeState.ConvergeUserNodes, func(name string) bool {
			return name == node
		})

		if err := s.ctx.SetConvergeState(convergeState); err != nil {
			failures = multierror.Append(failures, fmt.Errorf("save converge state without %s: %w", node, err))
		}
	}

	return failures.ErrorOrNil()
}

// cleanupOrder leaves the node the live client is connected to for last: the kube tunnel
// rides that session, the ssh backend reconnects on its own, and a reconnect made after
// the account is gone cannot authenticate.
func cleanupOrder(nodes []string, addresses map[string]string, connected string) []string {
	ordered := make([]string, 0, len(nodes))

	var last []string

	for _, node := range nodes {
		if addresses[node] == connected {
			last = append(last, node)
			continue
		}

		ordered = append(ordered, node)
	}

	return append(ordered, last...)
}

// masterAddresses is every master dhctl has an address for. A master this converge built is
// deliberately kept out of the session — that carries one generation of users — so its
// address comes from the hosts cache, written once the new masters are up.
func (s *KubeClientSwitcher) masterAddresses(sess *session.Session) (map[string]string, error) {
	cached, err := dstate.GetMasterHostsIPs(s.ctx.Ctx(), s.ctx.StateCache())
	if err != nil {
		return nil, fmt.Errorf("read master addresses from the cache: %w", err)
	}

	merged := dstate.MergeMasterHosts(sess.AvailableHosts(), cached)

	addresses := make(map[string]string, len(merged))
	for _, host := range merged {
		addresses[host.Name] = host.Host
	}

	return addresses, nil
}

// removeConvergeUserOn deletes the account while logged in as it. -f is what makes that
// work: userdel refuses a user that owns a running process, and the ssh session running
// the command is one. The account also carries an expiry date, in case this never runs.
func removeConvergeUserOn(ctx context.Context, provider libcon.StandaloneClientProvider, source libcon.SSHClient, creds sshCredentials, host session.Host) error {
	key := "converge-user-cleanup/" + host.Name
	sess := switchSession(source.Session(), creds, []session.Host{host})

	client, err := provider.StandaloneClientFor(ctx, key, sess, source.PrivateKeys())
	if err != nil {
		return fmt.Errorf("connect to %s as %s: %w", host.Name, creds.User, err)
	}

	// Nothing else stops a keyed client, and this one logs in with an account that is
	// about to be gone.
	defer provider.StopStandaloneClientFor(ctx, key)

	cmd := client.Command("userdel", "-f", "-r", global.ConvergeUserName)
	cmd.Sudo(ctx)

	if err := cmd.Run(ctx); err != nil {
		return fmt.Errorf("remove %s on %s: %w; stderr: %s", global.ConvergeUserName, host.Name, err, string(cmd.StderrBytes()))
	}

	return nil
}

func (s *KubeClientSwitcher) SwitchToFirstMaster(ctx context.Context) error {
	const action = "Switch clients to first control-plane node"

	if skip, err := s.isSkipOrLogStart(action, true); err != nil {
		return err
	} else if skip {
		return nil
	}

	return dhlog.RunProcess(ctx, s.slogger, action, func(ctx context.Context) error {
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
			state: selectMasterStates(firstMasterState, nil, keepAllMasters),
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
		firstMasterState, anotherMastersStates, err := s.extractStatesFromCluster(ctx)
		if err != nil {
			return err
		}

		statesMap := selectMasterStates(nil, anotherMastersStates, keepAllMasters)

		if len(statesMap) == 0 {
			if firstMasterState == nil {
				return fmt.Errorf("Cannot switch to another control-plane node: no states found")
			}

			s.warn("States for other control-plane nodes not found. Trying to continue with the first one")
			statesMap = selectMasterStates(firstMasterState, nil, keepAllMasters)
		}

		return s.replaceKubeClient(ctx, replaceKubeClientParams{
			state: statesMap,
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

	return s.switchAwayFromHosts(ctx, action, map[string]struct{}{nodeName: {}})
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

	return s.switchAwayFromHosts(ctx, action, deletedHostsNames)
}

// switchAwayFromHosts moves the clients to any control-plane node that is not being deleted.
func (s *KubeClientSwitcher) switchAwayFromHosts(ctx context.Context, action string, deleted map[string]struct{}) error {
	return dhlog.RunProcess(ctx, s.slogger, action, func(ctx context.Context) error {
		firstMaster, anotherMasters, err := s.extractStatesFromCluster(ctx)
		if err != nil {
			return err
		}

		statesMap := selectMasterStates(firstMaster, anotherMasters, func(name string) bool {
			_, deleting := deleted[name]
			return !deleting
		})

		return s.replaceKubeClient(ctx, replaceKubeClientParams{
			state: statesMap,
		})
	})
}

type replaceKubeClientParams struct {
	state map[string][]byte
	// creds names the user every host in state answers to. Nil picks it by node generation.
	creds *sshCredentials
}

func (s *KubeClientSwitcher) replaceKubeClient(ctx context.Context, params replaceKubeClientParams) error {
	if len(params.state) == 0 {
		return fmt.Errorf("Empty node states for replacing client")
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

	availableHosts := make([]session.Host, 0, len(params.state))

	ipExtractor, err := newSSHIPExtractor(s)
	if err != nil {
		return err
	}

	for nodeName, stateBytes := range params.state {
		ip, err := ipExtractor.getIPForSSH(s.ctx.Ctx(), &sshIPExtractorParams{
			nodeName: nodeName,
			state:    stateBytes,
			settings: settings,
		})
		if err != nil {
			return err
		}

		if ip != "" {
			availableHosts = append(availableHosts, session.Host{Host: ip, Name: nodeName})
		}
	}

	if len(availableHosts) == 0 {
		return fmt.Errorf("Cannot switch clients: no available hosts found in node states")
	}

	// Picked while the kube client still stands: the converge state it reads lives in
	// the cluster, reachable only through the session this call is about to replace.
	creds := params.creds

	if creds == nil {
		hosts, err := s.hostsOfOneGeneration(availableHosts)
		if err != nil {
			return err
		}

		if len(hosts) < len(availableHosts) {
			s.debug("Switching to %d of %d hosts: one session carries one generation", len(hosts), len(availableHosts))
		}

		availableHosts = hosts

		picked, err := s.credentialsFor(hostNames(availableHosts))
		if err != nil {
			return err
		}

		picked.Keys = sshCl.PrivateKeys()
		creds = &picked
	}

	if s.lockRunner != nil {
		s.lockRunner.Stop()
	}

	s.debug("Stopping kube proxies for replacing kube client")

	// todo during migrate to lib-connection
	// please use .*Switch.* function in ssh provider
	// also because we will use kube provider
	// setting kube client not needed

	s.debug("Creating new ssh client for replacing kube client")

	newSSHClient, err := s.switchClientTo(ctx, sshProvider, settings, *creds, availableHosts)
	if err != nil {
		return fmt.Errorf("failed to start SSH client: %w", err)
	}

	s.debug("SSH client started for replacing kube client")

	if err := newSSHClient.RefreshPrivateKeys(ctx); err != nil {
		return fmt.Errorf("Failed to refresh ssh agent private keys: %w", err)
	}

	s.debug("Private keys refreshed for replacing kube client")

	if s.lockRunner != nil {
		s.debugStartOperation("reset lock after replacing kube client")

		err := s.lockRunner.ResetLock(s.ctx.Ctx())
		if err != nil {
			return fmt.Errorf("Failed to reset lock: %w", err)
		}

		s.debug("lock was reset after replacing kube client")
	}

	return nil
}

// switchClientTo moves the ssh client to hosts under creds, retrying once as the user
// dhctl started with when the converge user does not answer. Any failure counts: a master
// built before that user existed and an unreachable one are still indistinguishable.
func (s *KubeClientSwitcher) switchClientTo(ctx context.Context, sshProvider libcon.SSHProvider, settings *session.Session, creds sshCredentials, hosts []session.Host) (libcon.SSHClient, error) {
	client, err := switchAndCheck(ctx, sshProvider, switchSession(settings, creds, hosts), creds.Keys)
	if err == nil {
		return client, nil
	}

	if creds.User != global.ConvergeUserName {
		return nil, err
	}

	s.warn("Cannot connect as %s: %v. The node looks built without the converge user, retrying as the user dhctl started with", creds.User, err)

	operator, credsErr := operatorCredentials(s.ctx)
	if credsErr != nil {
		return nil, fmt.Errorf("connect as %s (%v), then read the user dhctl started with: %w", creds.User, err, credsErr)
	}

	operator.Keys = creds.Keys

	client, retryErr := switchAndCheck(ctx, sshProvider, switchSession(settings, operator, hosts), operator.Keys)
	if retryErr != nil {
		return nil, fmt.Errorf("connect as %s (%v), then as %s: %w", creds.User, err, operator.User, retryErr)
	}

	return client, nil
}

// switchAndCheck switches the client and runs one command on it. That command is the only
// failure signal the legacy backend gives: clissh starts a local agent and nothing else,
// so a user the node does not know surfaces on the first command instead of on the switch.
func switchAndCheck(ctx context.Context, sshProvider libcon.SSHProvider, sess *session.Session, keys []session.AgentPrivateKey) (libcon.SSHClient, error) {
	client, err := sshProvider.SwitchClient(ctx, sess, keys)
	if err != nil {
		return nil, err
	}

	cmd := client.Command("true")
	if err := cmd.Run(ctx); err != nil {
		return nil, fmt.Errorf("run a command as %s: %w; stderr: %s", sess.User, err, string(cmd.StderrBytes()))
	}

	return client, nil
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
		state: state,
		creds: &sshCredentials{
			User:       convergeState.NodeUserCredentials.Name,
			Keys:       []session.AgentPrivateKey{privateKey},
			BecomePass: convergeState.NodeUserCredentials.Password,
		},
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

// sshless skips what only SSH can do. The NodeUser this switcher creates is delivered
// by bashible, and an sshless converge has neither bashible nor a way to log in, so the
// wait for it would never end.
func (s *KubeClientSwitcher) sshless(action string) bool {
	if s.ctx.SSHless() {
		s.warn("%s skipped. Converge runs over the Kubernetes API, with no SSH access to nodes", action)
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

	if s.sshless(action) {
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
	settings *session.Session
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

func (e *sshIPExtractor) getIPForSSH(ctx context.Context, params *sshIPExtractorParams) (string, error) {
	executor, err := e.getExecutor(ctx, params)
	if err != nil {
		return "", err
	}

	// do not cleanup provider after getting output executor!

	statePath, err := e.prepareState(params)
	if err != nil {
		return "", err
	}

	nodeName := params.nodeName

	addresses, err := infrastructure.GetMasterIPAddressForSSH(ctx, statePath, executor)
	if err != nil {
		e.switcher.warn(
			"Cannot extract IPs for node '%s':\n%v\nSkipping adding node to ssh client",
			nodeName,
			err,
		)
		return "", nil
	}

	sshIP := addresses.SSH
	internal := addresses.Internal

	if sshIP == "" && internal == "" {
		e.switcher.warn("IPs for node '%s' not found. Skipping adding node to ssh client", nodeName)
		return "", nil
	}

	bastion := params.settings.BastionHost

	if bastion != "" {
		e.switcher.debug(
			"Using node internal IP '%s' for node %s because bastion host '%s' was passed",
			internal,
			nodeName,
			bastion,
		)

		return internal, nil
	}

	e.switcher.debug("Using direct ssh IP '%s' for node %s", sshIP, nodeName)

	return sshIP, nil
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

	err := os.WriteFile(statePath, params.state, 0o644)
	if err != nil {
		return "", fmt.Errorf("Failed to write infrastructure state for %s in %s: %w", nodeName, statePath, err)
	}

	return statePath, nil
}

func (s *KubeClientSwitcher) GetGlobalOptions() *options.GlobalOptions {
	return s.params.GlobalOptions
}
