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

package infrastructure

import (
	"encoding/json"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

type NodeGroupTerraformController struct {
	metaConfig       *config.MetaConfig
	stateCache       state.Cache
	terraformContext *terraform.TerraformContext
	nodeGroupName    string
}

func NewNodesController(clusterMetaConfig *config.MetaConfig, stateCache state.Cache, nodeGroupName string, settings []byte, terraformContext *terraform.TerraformContext) (*NodeGroupTerraformController, error) {
	ngMetaConfig, err := getNgMetaConfig(clusterMetaConfig, settings)
	if err != nil {
		return nil, err
	}

	return &NodeGroupTerraformController{
		metaConfig:       ngMetaConfig,
		stateCache:       stateCache,
		terraformContext: terraformContext,
		nodeGroupName:    nodeGroupName,
	}, nil
}

func getNgMetaConfig(clusterMetaConfig *config.MetaConfig, settings []byte) (*config.MetaConfig, error) {
	cfg, err := clusterMetaConfig.DeepCopy().Prepare()
	if err != nil {
		return nil, fmt.Errorf("unable to prepare copied config: %v", err)
	}
	if settings != nil {
		nodeGroupsSettings, err := json.Marshal([]json.RawMessage{settings})
		if err != nil {
			log.ErrorLn(err)
		} else {
			cfg.ProviderClusterConfig["nodeGroups"] = nodeGroupsSettings
		}
	}

	return cfg, nil
}

func (r *NodeGroupTerraformController) DestroyNode(name string, nodeState []byte, autoApprove bool) error {
	stateName := fmt.Sprintf("%s.tfstate", name)
	if err := saveInCacheIfNotExists(r.stateCache, stateName, nodeState); err != nil {
		return err
	}

	step := "static-node"
	if r.nodeGroupName == "master" {
		step = "master-node"
	}

	nodeIndex, err := config.GetIndexFromNodeName(name)
	if err != nil {
		log.ErrorF("can't extract index from terraform state secret (%v), skip %s\n", err, name)
		return nil
	}

	nodeRunner := r.terraformContext.GetDestroyNodeRunner(r.metaConfig, r.stateCache, terraform.DestroyNodeRunnerOptions{
		AutoApprove:   autoApprove,
		NodeName:      name,
		NodeGroupName: r.nodeGroupName,
		NodeGroupStep: step,
		NodeIndex:     nodeIndex,
	})

	if err := terraform.DestroyPipeline(nodeRunner, name); err != nil {
		return fmt.Errorf("destroing of node %s failed: %v", name, err)
	}

	return nil
}
