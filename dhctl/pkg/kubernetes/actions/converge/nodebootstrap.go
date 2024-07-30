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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
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
