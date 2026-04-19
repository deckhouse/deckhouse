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

package controller

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type NodeGroupControllerRunner interface {
	Run(ctx *context.Context) error
}

func NewNodeGroupControllerRunner(name string, state state.NodeGroupInfrastructureState, excludeNodes map[string]bool, skipChecks bool) NodeGroupControllerRunner {
	controller := NewNodeGroupController(name, state, excludeNodes)

	if name == global.MasterNodeGroupName {
		return NewMasterNodeGroupController(controller, skipChecks)
	}

	return NewCloudPermanentNodeGroupController(controller)
}
