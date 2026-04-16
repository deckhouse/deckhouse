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
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/name212/govalue"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	kclient "github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
)

type KubeClientSwitcher struct {
	ctx        *Context
	lockRunner *lock.InLockRunner
	params     KubeClientSwitcherParams

	logger log.Logger
}

type KubeClientSwitcherParams struct {
	TmpDir        string
	IsDebug       bool
	Logger        log.Logger
	DisableSwitch bool
}

func NewKubeClientSwitcher(ctx *Context, lockRunner *lock.InLockRunner, params KubeClientSwitcherParams) *KubeClientSwitcher {
	logger := params.Logger
	if govalue.IsNil(logger) {
		logger = log.GetDefaultLogger()
	}

	return &KubeClientSwitcher{
		ctx:        ctx,
		lockRunner: lockRunner,
		logger:     logger,
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

	return s.logger.LogProcess("default", action, func() error {
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

	return s.logger.LogProcess("default", action, func() error {
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

	return s.logger.LogProcess("default", action, func() error {
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
				"Cannot find first control-plane node state or it is empty. Has states for [%s]",
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

	return s.logger.LogProcess("default", action, func() error {
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
				return fmt.Errorf("Cannot switch to another control-plane, no any states found")
			}

			s.warn("Another control-plane nodes states not found. Try to continue with first")
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
	const action = "Switch clients when destructive cahange control-plane nodes"

	if skip, err := s.isSkipOrLogStart(action, true); err != nil {
		return err
	} else if skip {
		return nil
	}

	_, sshClient, err := s.extractClients(ctx)
	if err != nil {
		return err
	}

	s.debug("SwitchClientsToAnotherNodeIfNeed sshClient: %v", sshClient)
	currentHost := sshClient.Session().CurrentHost()
	if currentHost.IsEmpty() {
		return fmt.Errorf("Got empty current host")
	}

	if nodeName != currentHost.Name {
		s.debug("Skip %s: current host is not deleted host '%s'", action, nodeName)
		return nil
	}

	return s.logger.LogProcess("default", action, func() error {
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

	if skip, err := s.isSkipOrLogStart(action, true); err != nil {
		return err
	} else if skip {
		return nil
	}

	logSkip := func(f string, args ...any) {
		s.debug(fmt.Sprintf("Skip %s: ", action)+f, args...)
	}

	if ngName != global.MasterNodeGroupName {
		logSkip("target node group '%s' is not master", ngName)
		return nil
	}

	if len(nodesToDeleteInfo) == 0 {
		logSkip("no nodes to delete")
		return nil
	}

	_, sshClient, err := s.extractClients(ctx)
	if err != nil {
		return err
	}

	s.debug("SwitchWhenDecreaseMastersIfNeed sshClient: %v", sshClient)
	currentHost := sshClient.Session().CurrentHost()
	if currentHost.IsEmpty() {
		return fmt.Errorf("Got empty current host")
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

	return s.logger.LogProcess("default", action, func() error {
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

func (s *KubeClientSwitcher) replaceKubeClient(ctx context.Context, params replaceKubeClientParams) error {
	if len(params.state) == 0 {
		return fmt.Errorf("Empty nodes states for replace client")
	}

	if params.convergeState == nil {
		return fmt.Errorf("Internal error. Empty converge state for replace client")
	}

	kubeCl, sshCl, err := s.extractClients(ctx)
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
		return fmt.Errorf("Cannot switch clients. Got empty available hosts from node states")
	}

	if s.lockRunner != nil {
		s.lockRunner.Stop()
	}

	s.debug("Stopping kube proxies for replacing kube client")

	// todo during migrate to lib-connection
	// please use .*Switch.* function in ssh provider
	// also because we will use kube provider
	// setting kube client not needed

	kubeCl.KubeProxy.StopAll()

	if sshclient.IsModernMode() {
		s.debug("Stop old SSH Client: %-v\n", sshCl)
		sshCl.Stop()
	}

	s.debug("Create new ssh client for replacing kube client")

	sess := session.NewSession(session.Input{
		User:           params.convergeState.NodeUserCredentials.Name,
		Port:           settings.Port,
		BastionHost:    settings.BastionHost,
		BastionPort:    settings.BastionPort,
		BastionUser:    settings.BastionUser,
		ExtraArgs:      settings.ExtraArgs,
		AvailableHosts: availableHosts,
		BecomePass:     params.convergeState.NodeUserCredentials.Password,
	})

	var pkeys []session.AgentPrivateKey

	appendPKey := params.appendPKey

	if appendPKey != nil {
		if sshclient.IsLegacyMode() {
			pkeys = append(pkeys, *appendPKey)
		} else {
			pkeys = append(sshCl.PrivateKeys(), *appendPKey)
		}
	} else {
		pkeys = sshCl.PrivateKeys()
	}

	newSSHClient := sshclient.NewClient(ctx, sess, pkeys)

	err = newSSHClient.Start()
	if err != nil {
		return fmt.Errorf("failed to start SSH client: %w", err)
	}

	s.debug("SSH client started for replacing kube client")

	if err := newSSHClient.RefreshPrivateKeys(); err != nil {
		return fmt.Errorf("Failed to refresh ssh agent private keys: %w", err)
	}

	s.debug("Private keys refreshed for replacing kube client")

	newKubeClient, err := kubernetes.ConnectToKubernetesAPI(s.ctx.Ctx(), ssh.NewNodeInterfaceWrapper(newSSHClient))
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes API: %w", err)
	}

	s.debug("connected to kube API for replacing kube client")

	s.ctx.setKubeClient(newKubeClient)

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

func (s *KubeClientSwitcher) tmpDirForConverger() (string, error) {
	tmpDir := filepath.Join(s.params.TmpDir, "converger")
	err := os.MkdirAll(tmpDir, 0o755)
	if err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("Failed to create tmp directory for converge: %w", err)
	}

	s.debug("Temp dir %s created for switch kube client", tmpDir)
	return tmpDir, nil
}

func (s *KubeClientSwitcher) createNodeUser(ctx context.Context) (*State, error) {
	convergeState, err := s.ctx.ConvergeState()
	if err != nil {
		return nil, err
	}

	if convergeState.NodeUserCredentials != nil {
		return convergeState, nil
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
	_, _, err = s.extractClients(ctx)
	if err != nil {
		return nil, err
	}

	err = entity.NewConvergerNodeUserExistsWaiter(s.ctx).WaitPresentOnNodes(ctx, nodeUserCredentials)
	if err != nil {
		return nil, fmt.Errorf("Could not ensure converger user is presented on control plane hosts: %w", err)
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

	kubeCl, _, err := s.extractClients(ctx)
	if err != nil {
		return nil, nil, err
	}

	states, err := infrastructurestate.GetMasterNodesStateFromCluster(ctx, kubeCl)
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
			return true, fmt.Errorf("Internal error. Disable switch to node user passed, but it needs for %s", action)
		}

		return true, nil
	}

	s.debugStartOperation(action)

	return false, nil
}

func (s *KubeClientSwitcher) extractClients(ctx context.Context) (*kclient.KubernetesClient, node.SSHClient, error) {
	kubeCl, err := s.ctx.KubeClientCtx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Cannot get kube client: %w", err)
	}

	sshCl := kubeCl.NodeInterfaceAsSSHClient()
	if govalue.IsNil(sshCl) {
		return nil, nil, fmt.Errorf("Node interface is not ssh")
	}

	return kubeCl, sshCl, nil
}

func (s *KubeClientSwitcher) debug(f string, args ...any) {
	// todo remove new line after migrate to lib-dhctl
	s.logger.LogDebugF(f+"\n", args...)
}

func (s *KubeClientSwitcher) warn(f string, args ...any) {
	// todo remove new line after migrate to lib-dhctl
	s.logger.LogWarnF(f+"\n", args...)
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
			"Cannot extract ips for node '%s':\n%v\nSkip adding node to ssh client",
			nodeName,
			err,
		)
		return "", nil
	}

	sshIP := addresses.SSH
	internal := addresses.Internal

	if sshIP == "" && internal == "" {
		e.switcher.warn("IPs for node '%s' not found. Skip adding node to ssh client", nodeName)
		return "", nil
	}

	bastion := params.settings.BastionHost

	if bastion != "" {
		e.switcher.debug(
			"Use node internal ip '%s' for node %s because bastion host '%s' was passed",
			internal,
			nodeName,
			bastion,
		)

		return internal, nil
	}

	e.switcher.debug("Use direct ssh ip '%s' for node %s", sshIP, nodeName)

	return sshIP, nil
}

func (e *sshIPExtractor) getExecutor(ctx context.Context, params *sshIPExtractorParams) (infrastructure.OutputExecutor, error) {
	nodeName := params.nodeName

	metaConfig, err := e.switcher.ctx.MetaConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get meta config for node %s: %w", nodeName, err)
	}

	logger := e.switcher.logger

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           e.tmpDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           logger,
		IsDebug:          e.switcher.params.IsDebug,
	})

	// yes working dir for output is not required
	provider, err := providerGetter(ctx, metaConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to create executor for node %s: %w", nodeName, err)
	}

	executor, err := provider.OutputExecutor(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("Cannot get output executor for node %s: %w", nodeName, err)
	}

	return executor, nil
}

func (e *sshIPExtractor) prepareState(params *sshIPExtractorParams) (string, error) {
	nodeName := params.nodeName

	statePath := filepath.Join(e.tmpDir, fmt.Sprintf("%s-%s.tfstate", nodeName, e.suffix))

	e.switcher.debug("State path for extracting ip for node %s: %s", nodeName, statePath)

	err := os.WriteFile(statePath, params.state, 0o644)
	if err != nil {
		return "", fmt.Errorf("Failed to write infrastructure state for %s in %s: %w", nodeName, statePath, err)
	}

	return statePath, nil
}
