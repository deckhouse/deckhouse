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
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type CloudPermanentNodeGroupController struct {
	*NodeGroupController
}

func NewCloudPermanentNodeGroupController(controller *NodeGroupController) *CloudPermanentNodeGroupController {
	cloudPermanentNodeGroupController := &CloudPermanentNodeGroupController{NodeGroupController: controller}
	cloudPermanentNodeGroupController.layoutStep = "static-node"
	cloudPermanentNodeGroupController.desiredReplicas = getReplicasByNodeGroupName(controller.config, controller.name)
	cloudPermanentNodeGroupController.nodeGroup = cloudPermanentNodeGroupController

	return cloudPermanentNodeGroupController
}

func (c *CloudPermanentNodeGroupController) Run() error {
	return c.NodeGroupController.Run()
}

func captureOutput(f func()) string {
	old := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		fmt.Println("Error creating pipe:", err)
		return ""
	}

	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()

	return buf.String()
}

func (c *CloudPermanentNodeGroupController) addNodes() error {
	count := len(c.state.State)
	index := 0

	var (
		nodesToWait        []string
		nodesIndexToCreate []int
		wg                 sync.WaitGroup
	)
	for c.desiredReplicas > count {
		candidateName := fmt.Sprintf("%s-%s-%v", c.config.ClusterPrefix, c.name, index)
		if _, ok := c.state.State[candidateName]; !ok {
			nodesIndexToCreate = append(nodesIndexToCreate, index)
			count++
		}
		index++
	}
	// type checkResult struct {
	// 	name string
	// 	log  string
	// 	err  error
	// }
	// resultsСhan := make(chan checkResult, len(nodesIndexToCreate))

	for _, indexCandidate := range nodesIndexToCreate {
		candidateName := fmt.Sprintf("%s-%s-%v", c.config.ClusterPrefix, c.name, indexCandidate)
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.InfoF("add goroutine for: %s, with index %v \n", candidateName, indexCandidate)
			// previouslyLogger := log.GetDefaultLogger()
			if indexCandidate != nodesIndexToCreate[0] {
				log.InitLogger("silent")
			}

			// captureOutput(func() {
			// 	BootstrapAdditionalNode(c.client, c.config, indexCandidate, c.layoutStep, c.name, c.cloudConfig, true, c.terraformContext)
			// })

			BootstrapAdditionalNode(c.client, c.config, indexCandidate, c.layoutStep, c.name, c.cloudConfig, true, c.terraformContext)

			log.InitLogger("simple")
			nodesToWait = append(nodesToWait, candidateName)
		}()
	}

	// for line := range resultsСhan {
	// 	log.InfoF("\n%s proccess: ", line.name)
	// 	log.InfoF("%s", line.log)
	// }

	wg.Wait()
	return WaitForNodesListBecomeReady(c.client, nodesToWait, nil)
}

func (c *CloudPermanentNodeGroupController) updateNode(nodeName string) error {
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

	nodeGroupName := c.name
	var nodeGroupSettingsFromConfig []byte

	// Node group settings are only for the static node.
	nodeGroupSettingsFromConfig = c.config.FindTerraNodeGroup(c.name)

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
			NewNodeStateSaver(c.client, nodeName, nodeGroupName, nodeGroupSettingsFromConfig),
		},
		Hook: &terraform.DummyHook{},
	})

	outputs, err := terraform.ApplyPipeline(nodeRunner, nodeName, terraform.OnlyState)
	if err != nil {
		log.ErrorF("Terraform exited with an error:\n%s\n", err.Error())
		return err
	}

	if tomb.IsInterrupted() {
		return ErrConvergeInterrupted
	}

	err = SaveNodeTerraformState(c.client, nodeName, c.name, outputs.TerraformState, nodeGroupSettingsFromConfig)
	if err != nil {
		return err
	}

	return WaitForSingleNodeBecomeReady(c.client, nodeName)
}

func (c *CloudPermanentNodeGroupController) deleteNodes(nodesToDeleteInfo []nodeToDeleteInfo) error {
	title := fmt.Sprintf("Delete Nodes from NodeGroup %s (replicas: %v)", c.name, c.desiredReplicas)
	return log.Process("converge", title, func() error {
		return c.deleteRedundantNodes(c.state.Settings, nodesToDeleteInfo, func(nodeName string) terraform.InfraActionHook {
			return &terraform.DummyHook{}
		})
	})
}
