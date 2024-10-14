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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	state_terraform "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
)

func LoadNodesStateForCommanderMode(stateCache dhctlstate.Cache, metaConfig *config.MetaConfig, kubeCl *client.KubernetesClient) (map[string]dhctlstate.NodeGroupTerraformState, error) {
	stateLoader := state_terraform.NewFileTerraStateLoader(stateCache, metaConfig)
	_, nodesState, err := stateLoader.PopulateClusterState()
	if err != nil {
		return nil, fmt.Errorf("state loader from cache failed: %w", err)
	}

	// NOTE(dhctl-for-commander): This Settings initialization needed for compatibility.
	// NOTE(dhctl-for-commander): If nodesState from local cache does not contain previous node-group-settings, then use settings from the cluster.
	// NOTE(dhctl-for-commander): In future versions nodesState loading from target kubernetes cluster for commander mode will be removed.
	inClusterNodesState, err := state_terraform.GetNodesStateFromCluster(kubeCl)
	if err != nil {
		return nil, fmt.Errorf("state loader from kubernetes failed: %w", err)
	}
	for nodeName, state := range nodesState {
		if state.Settings == nil {
			newState := state
			newState.Settings = inClusterNodesState[nodeName].Settings
			nodesState[nodeName] = newState
		}
	}

	return nodesState, nil
}
