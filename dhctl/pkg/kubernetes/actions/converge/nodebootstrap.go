// Copyright 2021 Flant JSC
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
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func NodeName(cfg *config.MetaConfig, nodeGroupName string, index int) string {
	return fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, nodeGroupName, index)
}

func BootstrapAdditionalNode(kubeCl *client.KubernetesClient, cfg *config.MetaConfig, index int, step, nodeGroupName, cloudConfig string, isConverge bool, terraformContext *terraform.TerraformContext) error {
	nodeName := NodeName(cfg, nodeGroupName, index)

	if isConverge {
		nodeExists, err := IsNodeExistsInCluster(kubeCl, nodeName)
		if err != nil {
			return err
		} else if nodeExists {
			return fmt.Errorf("node with name %s exists in cluster", nodeName)
		}
	}

	nodeGroupSettings := cfg.FindTerraNodeGroup(nodeGroupName)

	// TODO pass cache as argument or better refact func
	runner := terraformContext.GetBootstrapNodeRunner(cfg, cache.Global(), terraform.BootstrapNodeRunnerOptions{
		AutoApprove:     true,
		NodeName:        nodeName,
		NodeGroupStep:   step,
		NodeGroupName:   nodeGroupName,
		NodeIndex:       index,
		NodeCloudConfig: cloudConfig,
		AdditionalStateSaverDestinations: []terraform.SaverDestination{
			NewNodeStateSaver(kubeCl, nodeName, nodeGroupName, nodeGroupSettings),
		},
	})

	outputs, err := terraform.ApplyPipeline(runner, nodeName, terraform.OnlyState)
	if err != nil {
		return err
	}

	if tomb.IsInterrupted() {
		return ErrConvergeInterrupted
	}

	err = SaveNodeTerraformState(kubeCl, nodeName, nodeGroupName, outputs.TerraformState, nodeGroupSettings)
	if err != nil {
		return err
	}

	return nil
}

func BootstrapAdditionalNodeForParallelRun(kubeCl *client.KubernetesClient, cfg *config.MetaConfig, index int, step, nodeGroupName, cloudConfig string, isConverge bool, terraformContext *terraform.TerraformContext, buff *bytes.Buffer, saveLogToBuffer bool) error {
	nodeName := NodeName(cfg, nodeGroupName, index)

	nodeGroupSettings := cfg.FindTerraNodeGroup(nodeGroupName)

	// TODO pass cache as argument or better refact func
	runner := terraformContext.GetBootstrapNodeRunner(cfg, cache.Global(), terraform.BootstrapNodeRunnerOptions{
		AutoApprove:     true,
		NodeName:        nodeName,
		NodeGroupStep:   step,
		NodeGroupName:   nodeGroupName,
		NodeIndex:       index,
		NodeCloudConfig: cloudConfig,
		LogToBuffer:     saveLogToBuffer,

		AdditionalStateSaverDestinations: []terraform.SaverDestination{
			NewNodeStateSaver(kubeCl, nodeName, nodeGroupName, nodeGroupSettings),
		},
	})

	outputs, err := terraform.ApplyPipeline(runner, nodeName, terraform.OnlyState)
	if err != nil {
		return err
	}

	if tomb.IsInterrupted() {
		return ErrConvergeInterrupted
	}

	err = SaveNodeTerraformState(kubeCl, nodeName, nodeGroupName, outputs.TerraformState, nodeGroupSettings)
	if err != nil {
		return err
	}

	if saveLogToBuffer {
		logs := runner.GetLog()
		buff.WriteString(strings.Join(logs, "\n"))
	}

	return nil
}

func ParallelBootstrapAdditionalNodes(kubeCl *client.KubernetesClient, cfg *config.MetaConfig, nodesIndexToCreate []int, step, nodeGroupName, cloudConfig string, isConverge bool, terraformContext *terraform.TerraformContext) ([]string, error) {

	var (
		nodesToWait []string
		wg          sync.WaitGroup
		mu          sync.Mutex
	)

	type checkResult struct {
		name    string
		buffLog *bytes.Buffer
		err     error
	}

	for _, indexCandidate := range nodesIndexToCreate {
		candidateName := fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, nodeGroupName, indexCandidate)
		nodeExists, err := IsNodeExistsInCluster(kubeCl, candidateName)
		if err != nil {
			return nil, err
		} else if nodeExists {
			return nil, fmt.Errorf("node with name %s exists in cluster", candidateName)
		}
	}

	if len(nodesIndexToCreate) > 1 {
		log.WarnF("Many pipelines will run in parallel, terraform output for nodes %s-%v will be displayed after main execution.\n\n", nodeGroupName, nodesIndexToCreate[1:])
	}

	resultsChan := make(chan checkResult, len(nodesIndexToCreate))
	for i, indexCandidate := range nodesIndexToCreate {
		saveLogToBuffer := true
		candidateName := fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, nodeGroupName, indexCandidate)
		var buffLog bytes.Buffer
		wg.Add(1)
		go func(i, indexCandidate int, candidateName string) {
			defer wg.Done()
			if i == 0 {
				saveLogToBuffer = false
			}
			err := BootstrapAdditionalNodeForParallelRun(kubeCl, cfg, indexCandidate, step, nodeGroupName, cloudConfig, true, terraformContext, &buffLog, saveLogToBuffer)

			resultsChan <- checkResult{
				name:    candidateName,
				buffLog: &buffLog,
				err:     err,
			}
			mu.Lock()
			nodesToWait = append(nodesToWait, candidateName)
			mu.Unlock()
		}(i, indexCandidate, candidateName)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for candidate := range resultsChan {
		if candidate.err != nil {
			return nodesToWait, candidate.err
		}
		if candidate.buffLog.Len() == 0 {
			continue
		}
		currentLogger := log.GetProcessLogger()
		currentLogger.LogProcessStart(fmt.Sprintf("Output log [%s]", candidate.name))
		scanner := bufio.NewScanner(candidate.buffLog)
		for scanner.Scan() {
			log.InfoLn(scanner.Text())
		}
		currentLogger.LogProcessEnd()
	}
	return nodesToWait, nil
}

func BootstrapAdditionalMasterNode(kubeCl *client.KubernetesClient, cfg *config.MetaConfig, index int, cloudConfig string, isConverge bool, terraformContext *terraform.TerraformContext) (*terraform.PipelineOutputs, error) {
	nodeName := NodeName(cfg, MasterNodeGroupName, index)

	if isConverge {
		nodeExists, existsErr := IsNodeExistsInCluster(kubeCl, nodeName)
		if existsErr != nil {
			return nil, existsErr
		} else if nodeExists {
			return nil, fmt.Errorf("node with name %s exists in cluster", nodeName)
		}
	}

	// TODO pass cache as argument or better refact func
	runner := terraformContext.GetBootstrapNodeRunner(cfg, cache.Global(), terraform.BootstrapNodeRunnerOptions{
		AutoApprove:     true,
		NodeName:        nodeName,
		NodeGroupStep:   "master-node",
		NodeGroupName:   MasterNodeGroupName,
		NodeIndex:       index,
		NodeCloudConfig: cloudConfig,
		AdditionalStateSaverDestinations: []terraform.SaverDestination{
			NewNodeStateSaver(kubeCl, nodeName, MasterNodeGroupName, nil),
		},
	})

	outputs, err := terraform.ApplyPipeline(runner, nodeName, terraform.GetMasterNodeResult)
	if err != nil {
		return nil, err
	}

	if tomb.IsInterrupted() {
		return nil, ErrConvergeInterrupted
	}

	err = SaveMasterNodeTerraformState(kubeCl, nodeName, outputs.TerraformState, []byte(outputs.KubeDataDevicePath))
	if err != nil {
		return outputs, err
	}

	return outputs, err
}
