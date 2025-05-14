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

package converge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infra/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/maputil"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type MasterNodeGroupController struct {
	*NodeGroupController

	nodeToHost         map[string]string
	lockRunner         *InLockRunner
	convergeStateStore StateStore
	convergeState      *State
}

func NewMasterNodeGroupController(controller *NodeGroupController, lockRunner *InLockRunner, convergeStateStore StateStore, convergeState *State) *MasterNodeGroupController {
	masterNodeGroupController := &MasterNodeGroupController{
		NodeGroupController: controller,
		lockRunner:          lockRunner,
		convergeStateStore:  convergeStateStore,
		convergeState:       convergeState,
	}
	masterNodeGroupController.layoutStep = "master-node"
	masterNodeGroupController.desiredReplicas = getReplicasByNodeGroupName(controller.config, controller.name)
	masterNodeGroupController.nodeGroup = masterNodeGroupController

	return masterNodeGroupController
}

func (c *MasterNodeGroupController) populateNodeToHost() error {
	if c.nodeToHost != nil {
		return nil
	}

	var userPassedHosts []session.Host
	sshCl := c.client.NodeInterfaceAsSSHClient()
	if sshCl != nil {
		userPassedHosts = append(make([]session.Host, 0), sshCl.Settings.AvailableHosts()...)
	}

	nodesNames := make([]string, 0, len(c.state.State))
	for nodeName := range c.state.State {
		nodesNames = append(nodesNames, nodeName)
	}

	nodeToHost, err := ssh.CheckSSHHosts(userPassedHosts, nodesNames, string(c.convergeState.Phase), func(msg string) bool {
		if c.commanderMode {
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

func (c *MasterNodeGroupController) Run() error {
	if c.changeSettings.AutoDismissDestructive {
		return c.runWithReplicas(c.config.MasterNodeGroupSpec.Replicas)
	}

	if c.convergeState.NodeUserCredentials == nil {
		nodeUser, nodeUserCredentials, err := generateNodeUser()
		if err != nil {
			return fmt.Errorf("failed to generate NodeUser: %w", err)
		}

		err = createNodeUser(context.TODO(), c.client, nodeUser)
		if err != nil {
			return fmt.Errorf("failed to create or update NodeUser: %w", err)
		}

		c.convergeState.NodeUserCredentials = nodeUserCredentials

		err = c.convergeStateStore.SetState(c.convergeState)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}
	}

	if !c.commanderMode {
		if err := c.replaceKubeClient(c.state.State); err != nil {
			return fmt.Errorf("failed to replace kube client: %w", err)
		}

		c.lockRunner = NewInLockLocalRunner(c.client, "local-converger")
		if err := c.lockRunner.Run(c.run); err != nil {
			return fmt.Errorf("failed to run lock runner: %w", err)
		}
	}

	return nil
}

func (c *MasterNodeGroupController) run() (err error) {
	if c.convergeState.Phase == PhaseScaleToMultiMaster {
		replicas := 3

		err = c.runWithReplicas(replicas)
		if err != nil {
			return fmt.Errorf("failed to converge with 3 replicas: %w", err)
		}

		c.convergeState.Phase = PhaseScaleToSingleMaster

		err := c.convergeStateStore.SetState(c.convergeState)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}
	}

	if c.convergeState.Phase == PhaseScaleToSingleMaster {
		replicas := 1

		err := c.runWithReplicas(replicas)
		if err != nil {
			return fmt.Errorf("failed to converge with 1 replica: %w", err)
		}

		c.convergeState.Phase = ""

		err = c.convergeStateStore.SetState(c.convergeState)
		if err != nil {
			return fmt.Errorf("failed to set converge state: %w", err)
		}

		return nil
	}

	return c.runWithReplicas(c.config.MasterNodeGroupSpec.Replicas)
}

func (c *MasterNodeGroupController) replaceKubeClient(state map[string][]byte) (err error) {
	tmpDir := filepath.Join(app.CacheDir, "converge")

	err = os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create cache directory for NodeUser: %w", err)
	}

	privateKeyPath := filepath.Join(tmpDir, "id_rsa_converger")

	privateKey := session.AgentPrivateKey{
		Key:        privateKeyPath,
		Passphrase: c.convergeState.NodeUserCredentials.Password,
	}

	err = os.WriteFile(privateKeyPath, []byte(c.convergeState.NodeUserCredentials.PrivateKey), 0o600)
	if err != nil {
		return fmt.Errorf("failed to write private key for NodeUser: %w", err)
	}

	sshCl := c.client.NodeInterfaceAsSSHClient()
	if sshCl == nil {
		panic("Node interface is not ssh")
	}

	settings := sshCl.Settings

	for nodeName, stateBytes := range state {
		statePath := filepath.Join(tmpDir, fmt.Sprintf("%s.tfstate", nodeName))

		err := os.WriteFile(statePath, stateBytes, 0o644)
		if err != nil {
			return fmt.Errorf("failed to write terraform state: %w", err)
		}

		ipAddress, err := terraform.GetMasterIPAddressForSSH(statePath)
		if err != nil {
			log.WarnF("failed to get master IP address: %s", err)

			continue
		}

		settings.AddAvailableHosts(session.Host{Host: ipAddress, Name: nodeName})
	}

	if c.lockRunner != nil {
		c.lockRunner.Stop()
	}

	c.client.KubeProxy.StopAll()

	newSSHClient := ssh.NewClient(session.NewSession(session.Input{
		User:           c.convergeState.NodeUserCredentials.Name,
		Port:           settings.Port,
		BastionHost:    settings.BastionHost,
		BastionPort:    settings.BastionPort,
		BastionUser:    settings.BastionUser,
		ExtraArgs:      settings.ExtraArgs,
		AvailableHosts: settings.AvailableHosts(),
		BecomePass:     c.convergeState.NodeUserCredentials.Password,
	}), []session.AgentPrivateKey{privateKey})
	// Avoid starting a new ssh agent
	newSSHClient.InitializeNewAgent = false

	_, err = newSSHClient.Start()
	if err != nil {
		return fmt.Errorf("failed to start SSH client: %w", err)
	}

	err = newSSHClient.Agent.AddKeys(newSSHClient.PrivateKeys)
	if err != nil {
		return fmt.Errorf("failed to add keys to ssh agent: %w", err)
	}

	kubeClient, err := kubernetes.ConnectToKubernetesAPI(ssh.NewNodeInterfaceWrapper(newSSHClient))
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes API: %w", err)
	}

	c.client = kubeClient
	return nil
}

func (c *MasterNodeGroupController) runWithReplicas(replicas int) error {
	c.desiredReplicas = replicas
	c.nodeToHost = nil

	return c.NodeGroupController.Run()
}

func (c *MasterNodeGroupController) addNodes() error {
	count := len(c.state.State)
	index := 0

	var (
		nodesToWait        []string
		masterIPForSSHList []session.Host
		nodeInternalIPList []string
	)

	for c.desiredReplicas > count {
		candidateName := fmt.Sprintf("%s-%s-%v", c.config.ClusterPrefix, c.name, index)

		if _, ok := c.state.State[candidateName]; !ok {
			output, err := BootstrapAdditionalMasterNode(c.client, c.config, index, c.cloudConfig, true, c.terraformContext)
			if err != nil {
				return err
			}

			masterIPForSSHList = append(masterIPForSSHList, session.Host{Host: output.MasterIPForSSH, Name: candidateName})
			nodeInternalIPList = append(nodeInternalIPList, output.NodeInternalIP)

			count++
			c.state.State[candidateName] = output.TerraformState
			nodesToWait = append(nodesToWait, candidateName)
		}
		index++
	}

	err := WaitForNodesListBecomeReady(c.client, nodesToWait, controlplane.NewManagerReadinessChecker(c.client))
	if err != nil {
		return err
	}

	if len(masterIPForSSHList) > 0 {
		if !c.commanderMode {
			sshCl := c.client.NodeInterfaceAsSSHClient()
			if sshCl == nil {
				panic("NodeInterface is not ssh")
			}

			sshCl.Settings.AddAvailableHosts(masterIPForSSHList...)
		}

		// we hide deckhouse logs because we always have config
		nodeCloudConfig, err := GetCloudConfig(c.client, c.name, HideDeckhouseLogs, log.GetDefaultLogger(), nodeInternalIPList...)
		if err != nil {
			return err
		}

		c.cloudConfig = nodeCloudConfig
	}

	return nil
}

func (c *MasterNodeGroupController) updateNode(nodeName string) error {
	// NOTE: In the commander mode nodes state should exist in the local state cache, no need to pass state explicitly.
	var nodeState []byte
	if !c.commanderMode {
		nodeState = c.state.State[nodeName]
	}

	nodeIndex, err := config.GetIndexFromNodeName(nodeName)
	if err != nil {
		log.ErrorF("can't extract index from terraform state secret (%v), skip %s\n", err, nodeName)
		return nil
	}

	hook := c.newHookForUpdatePipeline(nodeName)

	var nodeGroupSettingsFromConfig []byte

	nodeRunner := c.terraformContext.GetConvergeNodeRunner(c.config, terraform.NodeRunnerOptions{
		AutoDismissDestructive: c.changeSettings.AutoDismissDestructive,
		AutoApprove:            c.changeSettings.AutoApprove,
		NodeName:               nodeName,
		NodeGroupName:          c.name,
		NodeGroupStep:          c.layoutStep,
		NodeIndex:              nodeIndex,
		NodeState:              nodeState,
		NodeCloudConfig:        c.cloudConfig,
		CommanderMode:          c.commanderMode,
		StateCache:             c.stateCache,
		AdditionalStateSaverDestinations: []terraform.SaverDestination{
			NewNodeStateSaver(c.client, nodeName, MasterNodeGroupName, nodeGroupSettingsFromConfig),
		},
		Hook: hook,
	})

	outputs, err := terraform.ApplyPipeline(nodeRunner, nodeName, terraform.GetMasterNodeResult)
	if err != nil {
		if errors.Is(err, controlplane.ErrSingleMasterClusterTerraformPlanHasDestructiveChanges) {
			confirmation := input.NewConfirmation().WithMessage("A single-master cluster has disruptive changes in the Terraform plan. Trying to migrate to a multi-master cluster and back to a single-master cluster. Do you want to continue?")
			if !c.changeSettings.AutoApprove && !confirmation.Ask() {
				log.InfoLn("Aborted")
				return nil
			}

			c.convergeState.Phase = PhaseScaleToMultiMaster

			err := c.convergeStateStore.SetState(c.convergeState)
			if err != nil {
				return fmt.Errorf("failed to set converge state: %w", err)
			}

			err = c.run()
			if err != nil {
				return fmt.Errorf("failed to converge to multi-master: %w", err)
			}

			return nil
		}

		log.ErrorF("Terraform exited with an error:\n%s\n", err.Error())

		return err
	}

	if tomb.IsInterrupted() {
		return ErrConvergeInterrupted
	}

	err = SaveMasterNodeTerraformState(c.client, nodeName, outputs.TerraformState, []byte(outputs.KubeDataDevicePath))
	if err != nil {
		return err
	}

	c.state.State[nodeName] = outputs.TerraformState

	return WaitForSingleNodeBecomeReady(c.client, nodeName)
}

func (c *MasterNodeGroupController) newHookForUpdatePipeline(convergedNode string) terraform.InfraActionHook {
	err := c.populateNodeToHost()
	if err != nil {
		return nil
	}

	nodesToCheck := maputil.ExcludeKeys(c.nodeToHost, convergedNode)

	confirm := func(msg string) bool {
		return input.NewConfirmation().WithMessage(msg).Ask()
	}

	if c.changeSettings.AutoApprove {
		confirm = func(_ string) bool {
			return true
		}
	}

	return controlplane.NewHookForUpdatePipeline(c.client, nodesToCheck, c.config.UUID, c.commanderMode).
		WithSourceCommandName("converge").
		WithNodeToConverge(convergedNode).
		WithConfirm(confirm)
}

func (c *MasterNodeGroupController) deleteNodes(nodesToDeleteInfo []nodeToDeleteInfo) error {
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

	title := fmt.Sprintf("Delete Nodes from NodeGroup %s (replicas: %v)", MasterNodeGroupName, c.desiredReplicas)
	return log.Process("converge", title, func() error {
		return c.deleteRedundantNodes(c.state.Settings, nodesToDeleteInfo, func(nodeName string) terraform.InfraActionHook {
			return controlplane.NewHookForDestroyPipeline(c.client, nodeName, c.commanderMode)
		})
	})
}

func (c *MasterNodeGroupController) totalReplicas() int {
	return len(c.state.State)
}
