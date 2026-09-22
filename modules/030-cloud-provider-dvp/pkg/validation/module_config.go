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
	"fmt"

	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

const (
	CodeReservedAdditionalVMLabelKey = "reserved_additional_vm_label_key"
)

var reservedAdditionalVMLabelKeys = []string{
	"deckhouse.io/managed-by",
	"dvp.deckhouse.io/cluster-uuid",
	"dvp.deckhouse.io/hostname",
}

// ValidateAdditionalVMLabels rejects labels owned by Deckhouse or cloud-provider-dvp.
func ValidateAdditionalVMLabels(state *State) cpvalapi.Result {
	result := cpvalapi.Result{}
	if state == nil || state.ModuleConfig == nil || state.ModuleConfig.Spec.Settings == nil {
		return result
	}

	labels := state.ModuleConfig.Spec.Settings.Nodes.Parameters.AdditionalVMLabels
	for _, key := range reservedAdditionalVMLabelKeys {
		if _, ok := labels[key]; !ok {
			continue
		}

		result.AddError(
			fmt.Sprintf("ModuleConfig.spec.settings.nodes.parameters.additionalVMLabels[%q]", key),
			CodeReservedAdditionalVMLabelKey,
			key,
			fmt.Sprintf("label key %q is reserved and cannot be set in additionalVMLabels", key),
		)
	}

	return result
}
