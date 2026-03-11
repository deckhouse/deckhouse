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
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
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
	const action = "Switch to node user"

	if s.switchDisbled(action) {
		return nil
	}

	if s.inCommander(action) {
		return nil
	}

	s.debugStartOperation(action)

	convergeState, err := s.ctx.ConvergeState()
	if err != nil {
		return err
	}

	if convergeState.NodeUserCredentials == nil {
		s.debug("Generate node user")
		nodeUser, nodeUserCredentials, err := v1.GenerateNodeUser(v1.ConvergerNodeUser())
		if err != nil {
			return fmt.Errorf("Failed to generate NodeUser: %w", err)
		}

		c, cancel := s.ctx.WithTimeout(10 * time.Second)
		defer cancel()
		err = entity.CreateOrUpdateNodeUser(c, s.ctx, nodeUser, nil)
		if err != nil {
			return fmt.Errorf("Failed to create or update NodeUser: %w", err)
		}

		kubeCl, err := s.ctx.KubeClientCtx(ctx)
		if err != nil {
			return fmt.Errorf("Cannot get kube client: %w", err)
		}

		sshCl := kubeCl.NodeInterfaceAsSSHClient()
		if sshCl == nil {
			return fmt.Errorf("Node interface is not ssh")
		}

		err = entity.NewConvergerNodeUserExistsWaiter(s.ctx).WaitPresentOnNodes(ctx, nodeUserCredentials)
		if err != nil {
			return fmt.Errorf("Could not ensure converger user is presented on control plane hosts: %w", err)
		}

		convergeState.NodeUserCredentials = nodeUserCredentials

		err = s.ctx.SetConvergeState(convergeState)
		if err != nil {
			return fmt.Errorf("Failed to set converge state: %w", err)
		}
	}

	return s.replaceKubeClientForSwithToNodeUser(ctx, convergeState, nodesState)
}

func (s *KubeClientSwitcher) CleanupNodeUser() error {
	const action = "Cleanup node user"

	if s.switchDisbled(action) {
		return nil
	}

	if s.inCommander(action) {
		return nil
	}

	s.debugStartOperation(action)

	err := s.ctx.deleteConvergeState()
	if err != nil {
		return err
	}

	c, cancel := s.ctx.WithTimeout(10 * time.Second)
	defer cancel()
	return entity.DeleteNodeUser(c, s.ctx, global.ConvergeNodeUserName)
}

const firstMasterSuffix = "-0"

func (s *KubeClientSwitcher) SwitchToFirstMaster(ctx context.Context) error {
	const action = "Switch to first master"

	if s.inCommander(action) {
		return nil
	}

	if s.switchDisbled(action) {
		return fmt.Errorf("Internal error. Disable switch to node user passed, but it needs for %s", action)
	}

	s.debugStartOperation(action)

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
			mastersNames = append(mastersNames, s.nodeName)
		}

		return fmt.Errorf(
			"Cannot find first control-plane node state or it is empty. Has states for [%s]",
			strings.Join(mastersNames, ", "),
		)
	}

	return s.replaceKubeClient(ctx, replaceKubeClientParams{
		convergeState: convergeState,
		state: map[string][]byte{
			firstMasterState.nodeName: firstMasterState.state,
		},
		appendPKey: nil,
	})
}

func (s *KubeClientSwitcher) SwitchToNotFirstMaster(ctx context.Context) error {
	const action = "Switch to not first master"

	if s.inCommander(action) {
		return nil
	}

	if s.switchDisbled(action) {
		return fmt.Errorf("Internal error. Disable switch to node user passed, but it needs for %s", action)
	}

	s.debugStartOperation(action)

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
		statesMap[s.nodeName] = s.state
	}

	if len(statesMap) == 0 {
		if firstMasterState == nil {
			return fmt.Errorf("Cannot switch to another control-plane, no any states found")
		}

		s.logger.LogWarnF("Another control-plane nodes states not found. Try to continue with first")
		statesMap[firstMasterState.nodeName] = firstMasterState.state
	}

	return s.replaceKubeClient(ctx, replaceKubeClientParams{
		convergeState: convergeState,
		state:         statesMap,
		appendPKey:    nil,
	})
}

type replaceKubeClientParams struct {
	convergeState *State
	state         map[string][]byte
	appendPKey    *session.AgentPrivateKey
}

func (s *KubeClientSwitcher) replaceKubeClient(ctx context.Context, params replaceKubeClientParams) error {
	kubeCl, err := s.ctx.KubeClientCtx(ctx)
	if err != nil {
		return fmt.Errorf("Cannot get kube client: %w", err)
	}

	sshCl := kubeCl.NodeInterfaceAsSSHClient()
	if sshCl == nil {
		return fmt.Errorf("Node interface is not ssh")
	}

	tmpDir, err := s.tmpDirForConverger()
	if err != nil {
		return err
	}

	settings := sshCl.Session()

	suff := rand.NewSource(time.Now().UnixNano()).Int63()

	for nodeName, stateBytes := range params.state {
		metaConfig, err := s.ctx.MetaConfig()
		if err != nil {
			return fmt.Errorf("failed to get meta config for node %s: %w", nodeName, err)
		}

		statePath := filepath.Join(tmpDir, fmt.Sprintf("%s-%d.tfstate", nodeName, suff))

		s.debug("for extracting statePath: %s", statePath)

		err = os.WriteFile(statePath, stateBytes, 0o644)
		if err != nil {
			return fmt.Errorf("failed to write infrastructure state: %w", err)
		}

		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           tmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           s.logger,
			IsDebug:          s.params.IsDebug,
		})

		// yes working dir for output is not required
		provider, err := providerGetter(s.ctx.Ctx(), metaConfig)
		if err != nil {
			return fmt.Errorf("failed to create executor for node %s: %w", nodeName, err)
		}

		executor, _ := provider.OutputExecutor(s.ctx.Ctx(), s.logger)

		// do not cleanup provider after getting output executor!

		ipAddress, err := infrastructure.GetMasterIPAddressForSSH(s.ctx.Ctx(), statePath, executor)
		if err != nil {
			s.logger.LogWarnF("Failed to get master IP address: %v\n", err)
			continue
		}

		settings.AddAvailableHosts(session.Host{Host: ipAddress, Name: nodeName})

		s.debug("Extracted ip address %s and node name: %s", ipAddress, nodeName)
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
		s.debug("Old SSH Client: %-v\n", sshCl)
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
		AvailableHosts: settings.AvailableHosts(),
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

	if err := newSSHClient.RefreshPrivateKeys(); err != nil {
		return fmt.Errorf("Failed to refresh ssh agent private keys: %w", err)
	}

	s.debug("Private keys refreshed for replacing kube client")

	err = newSSHClient.Start()
	if err != nil {
		return fmt.Errorf("failed to start SSH client: %w", err)
	}

	s.debug("SSH client started for replacing kube client")

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

	s.debug("Temp dir %s created for kubeclient", tmpDir)
	return tmpDir, nil
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

type nodeState struct {
	nodeName string
	state    []byte
}

func (s *KubeClientSwitcher) extractStatesFromCluster(ctx context.Context) (*nodeState, []*nodeState, error) {
	kubeCl, err := s.ctx.KubeClientCtx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Cannot get kube client: %w", err)
	}

	states, err := infrastructurestate.GetMasterNodesStateFromCluster(ctx, kubeCl)
	if err != nil {
		return nil, nil, fmt.Errorf("Cannot extract control-plane node states: %w", err)
	}

	var firstMasterState *nodeState
	anoterNodesStates := make([]*nodeState, 0, 2)

	for nodeName, state := range states {
		st := &nodeState{
			nodeName: nodeName,
			state:    state,
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
			return anoterNodesStates[i].nodeName < anoterNodesStates[j].nodeName
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
		s.logger.LogWarnF("%s skipped. Switch disabled\n", action)
		return true
	}

	return false
}

func (s *KubeClientSwitcher) debug(f string, args ...any) {
	// todo remove new line after migrate to lib-dhctl
	s.logger.LogDebugF(f+"\n", args...)
}

func (s *KubeClientSwitcher) debugStartOperation(action string) {
	s.debug("Starting %s", strings.ToLower(action))
}
