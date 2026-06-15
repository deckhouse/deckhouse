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

import "fmt"

// ValidateModuleConfig checks ModuleConfig presence and module-specific invariants.
func ValidateModuleConfig(state *State) Result {
	if state == nil {
		return ResultForNilState()
	}

	result := Result{}

	if state.ModuleConfig == nil {
		if len(state.LegacyProviderClusterConfig) == 0 {
			result.AddError("ModuleConfig", "module_config_required", "ModuleConfig is required")
		}

		return result
	}

	moduleConfig := state.ModuleConfig
	if moduleConfig.Name != "" && moduleConfig.Name != state.ModuleName {
		result.AddError(
			"ModuleConfig.metadata.name",
			"invalid_module_config_name",
			fmt.Sprintf("must be %q", state.ModuleName),
		)
	}

	return result
}
