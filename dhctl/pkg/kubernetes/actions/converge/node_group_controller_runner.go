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
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

type NodeGroupControllerRunner interface {
	Run() error
}

func NewNodeGroupControllerRunner(
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	name string,
	state state.NodeGroupTerraformState,
	stateCache state.Cache,
	terraformContext *terraform.TerraformContext,
	commanderMode bool,
	changeSettings *terraform.ChangeActionSettings,
	nodesMap map[string]bool,
	lockRunner *InLockRunner,
	convergeStateStore StateStore,
	convergeState *State) NodeGroupControllerRunner {
	controller := NewNodeGroupController(kubeCl, metaConfig, name, state, stateCache, terraformContext, commanderMode, changeSettings, nodesMap)

	if name == MasterNodeGroupName {
		return NewMasterNodeGroupController(controller, lockRunner, convergeStateStore, convergeState)
	}

	return NewCloudPermanentNodeGroupController(controller)
}
