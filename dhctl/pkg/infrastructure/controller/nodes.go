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

package controller

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type NodeGroupInfrastructureController struct {
	metaConfig            *config.MetaConfig
	stateCache            state.Cache
	infrastructureContext *infrastructure.Context
	nodeGroupName         string
}

func NewNodesController(ctx context.Context, clusterMetaConfig *config.MetaConfig, stateCache state.Cache, nodeGroupName string, settings []byte, infrastructureContext *infrastructure.Context) (*NodeGroupInfrastructureController, error) {
	ngMetaConfig, err := getNgMetaConfig(ctx, clusterMetaConfig, settings)
	if err != nil {
		return nil, err
	}

	return &NodeGroupInfrastructureController{
		metaConfig:            ngMetaConfig,
		stateCache:            stateCache,
		infrastructureContext: infrastructureContext,
		nodeGroupName:         nodeGroupName,
	}, nil
}

func getNgMetaConfig(ctx context.Context, clusterMetaConfig *config.MetaConfig, settings []byte) (*config.MetaConfig, error) {
	// we use dummy preparator because metaConfig was prepared early
	cfg, err := clusterMetaConfig.DeepCopy().Prepare(ctx, config.DummyPreparatorProvider())
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

func (r *NodeGroupInfrastructureController) DestroyNode(ctx context.Context, name string, nodeState []byte, autoApprove bool) error {
	stateName := fmt.Sprintf("%s.tfstate", name)
	if err := saveInCacheIfNotExists(r.stateCache, stateName, nodeState); err != nil {
		return err
	}

	nodeIndex, err := config.GetIndexFromNodeName(name)
	if err != nil {
		log.ErrorF("can't extract index from infrastructure state secret (%v), skip %s\n", err, name)
		return nil
	}

	nodeRunner, err := r.infrastructureContext.GetDestroyNodeRunner(ctx, r.metaConfig, r.stateCache, infrastructure.DestroyNodeRunnerOptions{
		AutoApproveSettings: infrastructure.AutoApproveSettings{
			AutoApprove: autoApprove,
		},
		NodeName:      name,
		NodeGroupName: r.nodeGroupName,
		NodeGroupStep: infrastructure.GetStepByNodeGroupName(r.nodeGroupName),
		NodeIndex:     nodeIndex,
	})
	if err != nil {
		return err
	}

	if err := infrastructure.DestroyPipeline(ctx, nodeRunner, name); err != nil {
		return fmt.Errorf("destroing of node %s failed: %v", name, err)
	}

	return nil
}
