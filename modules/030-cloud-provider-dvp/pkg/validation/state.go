// Copyright 2026 Flant JSC
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

package validation

import (
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

// BuildStateFromProtocolInput decodes dhctl input and enriches it with DVP migration status.
func BuildStateFromProtocolInput(input proto.PrepareInput, vars *proto.CloudProviderVars) (*cpval.State, error) {
	state, err := cpval.BuildStateFromProtocolInput(ModuleName, input, vars)
	if err != nil {
		return nil, err
	}

	state.MigrationStatus = MigrationStatusFromState(state)

	return state, nil
}
