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
	"os"
	"path/filepath"
	"time"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh"
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
	TmpDir  string
	IsDebug bool
	Logger  log.Logger
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
	if s.ctx.CommanderMode() {
		s.logger.LogDebugLn("Switch to node user skipped. In commander mode")
		return nil
	}

	s.logger.LogDebugLn("Start switching to node user")

	convergeState, err := s.ctx.ConvergeState()
	if err != nil {
		return err
	}

	if convergeState.NodeUserCredentials == nil {
		s.logger.LogDebugLn("Generate node user")
		nodeUser, nodeUserCredentials, err := GenerateNodeUser()
		if err != nil {
			return fmt.Errorf("failed to generate NodeUser: %w", err)
		}

		c, cancel := s.ctx.WithTimeout(10 * time.Second)
		defer cancel()
		err = entity.CreateNodeUser(c, s.ctx, nodeUser)
		if err != nil {
			return fmt.Errorf("failed to create or update NodeUser: %w", err)
		}
		sshCl := s.ctx.KubeClient().NodeInterfaceAsSSHClient()
		if sshCl == nil {
			return fmt.Errorf("Node interface is not ssh")
		}
		currentHost := sshCl.Session().Host()

		err = entity.WaitForNodeUserPresentOnNode(ctx, s.ctx.KubeClient(), currentHost, global.ConvergeNodeUserName)
		if err != nil {
			return fmt.Errorf("could not ensure %s is presented on %s: %w", global.ConvergeNodeUserName, currentHost, err)
		}

		convergeState.NodeUserCredentials = nodeUserCredentials

		err = s.ctx.SetConvergeState(convergeState)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}
	}

	return s.replaceKubeClient(ctx, convergeState, nodesState)
}

func (s *KubeClientSwitcher) tmpDirForConverger() string {
	return filepath.Join(s.params.TmpDir, "converger")
}

func (s *KubeClientSwitcher) replaceKubeClient(ctx context.Context, convergeState *State, state map[string][]byte) (err error) {
	s.logger.LogDebugLn("Starting replacing kube client")

	tmpDir := s.tmpDirForConverger()

	err = os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create cache directory for NodeUser: %w", err)
	}

	s.logger.LogDebugLn("Tempdir created for kubeclient")

	privateKeyPath := filepath.Join(tmpDir, "id_rsa_converger")

	privateKey := session.AgentPrivateKey{
		Key:        privateKeyPath,
		Passphrase: convergeState.NodeUserCredentials.Password,
	}

	err = os.WriteFile(privateKeyPath, []byte(convergeState.NodeUserCredentials.PrivateKey), 0o600)
	if err != nil {
		return fmt.Errorf("failed to write private key for NodeUser: %w", err)
	}

	s.logger.LogDebugLn("Private key written")

	kubeCl := s.ctx.KubeClient()

	sshCl := kubeCl.NodeInterfaceAsSSHClient()
	if sshCl == nil {
		return fmt.Errorf("Node interface is not ssh")
	}

	settings := sshCl.Session()

	for nodeName, stateBytes := range state {
		metaConfig, err := s.ctx.MetaConfig()
		if err != nil {
			return fmt.Errorf("failed to get meta config for node %s: %w", nodeName, err)
		}
		statePath := filepath.Join(tmpDir, fmt.Sprintf("%s.tfstate", nodeName))

		s.logger.LogDebugLn("for extracting statePath: %s", statePath)

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

		executor, err := provider.OutputExecutor(s.ctx.Ctx(), s.logger)

		// do not cleanup provider after getting output executor!

		ipAddress, err := infrastructure.GetMasterIPAddressForSSH(s.ctx.Ctx(), statePath, executor)
		if err != nil {
			s.logger.LogWarnF("failed to get master IP address: %v\n", err)
			continue
		}

		settings.AddAvailableHosts(session.Host{Host: ipAddress, Name: nodeName})

		s.logger.LogDebugF("Extracted ip address %s and node name: %s", ipAddress, nodeName)
	}

	if s.lockRunner != nil {
		s.lockRunner.Stop()
	}

	s.logger.LogDebugLn("Stopping kube proxies for replacing kube client")

	kubeCl.KubeProxy.StopAll()

	if sshclient.IsModernMode() {
		s.logger.LogDebugF("Old SSH Client: %-v\n", sshCl)
		sshCl.Stop()
	}

	s.logger.LogDebugLn("Create new ssh client for replacing kube client")

	sess := session.NewSession(session.Input{
		User:           convergeState.NodeUserCredentials.Name,
		Port:           settings.Port,
		BastionHost:    settings.BastionHost,
		BastionPort:    settings.BastionPort,
		BastionUser:    settings.BastionUser,
		ExtraArgs:      settings.ExtraArgs,
		AvailableHosts: settings.AvailableHosts(),
		BecomePass:     convergeState.NodeUserCredentials.Password,
	})

	var pkeys []session.AgentPrivateKey

	if sshclient.IsLegacyMode() {
		pkeys = append(pkeys, privateKey)
	} else {
		pkeys = append(sshCl.PrivateKeys(), privateKey)
	}
	newSSHClient := sshclient.NewClient(ctx, sess, pkeys)

	err = newSSHClient.Start()
	if err != nil {
		return fmt.Errorf("failed to start SSH client: %w", err)
	}

	s.logger.LogDebugLn("ssh client started for replacing kube client")

	// adding keys to agent is actual only in legacy mode
	if sshclient.IsLegacyMode() {
		err = newSSHClient.(*clissh.Client).Agent.AddKeys(newSSHClient.PrivateKeys())
		if err != nil {
			return fmt.Errorf("failed to add keys to ssh agent: %w", err)
		}

		s.logger.LogDebugLn("private keys added for replacing kube client")
	}

	newKubeClient, err := kubernetes.ConnectToKubernetesAPI(s.ctx.Ctx(), ssh.NewNodeInterfaceWrapper(newSSHClient))
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes API: %w", err)
	}

	s.logger.LogDebugLn("connected to kube API for replacing kube client")

	s.ctx.setKubeClient(newKubeClient)

	if s.lockRunner != nil {
		s.logger.LogDebugLn("starting reset lock after replacing kube client")
		err := s.lockRunner.ResetLock(s.ctx.Ctx())
		if err != nil {
			return fmt.Errorf("failed to reset lock: %w", err)
		}
		s.logger.LogDebugLn("lock was reset after replacing kube client")
	}

	return nil
}

func (s *KubeClientSwitcher) CleanupNodeUser() error {
	if s.ctx.CommanderMode() {
		s.logger.LogDebugLn("Cleanup node user skipped. In commander mode")
		return nil
	}

	err := s.ctx.deleteConvergeState()
	if err != nil {
		return err
	}

	c, cancel := s.ctx.WithTimeout(10 * time.Second)
	defer cancel()
	return entity.DeleteNodeUser(c, s.ctx, global.ConvergeNodeUserName)
}
