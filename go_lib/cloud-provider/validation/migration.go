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
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

type legacyProviderClusterConfig struct {
	MasterNodeGroup map[string]any   `json:"masterNodeGroup,omitempty"`
	NodeGroups      []map[string]any `json:"nodeGroups,omitempty"`
}

// MigrationStatusFromState derives migration status from the decoded validation state.
func MigrationStatusFromState(state *State) cpapi.MigrationStatus {
	if state == nil || len(state.LegacyProviderClusterConfig) == 0 {
		return cpapi.MigrationStatus{}
	}

	complete := IsNewResourcesComplete(state)
	return cpapi.MigrationStatus{
		LegacyPCCPresent:     true,
		NewResourcesComplete: complete,
		MigrationPending:     !complete,
	}
}

// IsNewResourcesComplete reports whether all new-model resources required by legacy PCC are present.
func IsNewResourcesComplete(state *State) bool {
	if state == nil {
		return false
	}

	if state.ModuleConfig == nil ||
		state.ModuleConfig.Spec.Version < 2 ||
		!isModuleConfigEnabled(state.ModuleConfig) ||
		!hasProviderSettings(state.ModuleConfig) {
		return false
	}

	if _, ok := findCredentialSecret(state, cpapi.CredentialSecretName); !ok {
		return false
	}

	legacy, err := DecodeJSONValue[legacyProviderClusterConfig](state.LegacyProviderClusterConfig)
	if err != nil {
		return false
	}

	nodeGroups := make(map[string]struct{}, len(state.NodeGroups))
	for _, nodeGroup := range state.NodeGroups {
		nodeGroups[nodeGroup.Name] = struct{}{}
	}

	instanceClasses := make(map[string]struct{}, len(state.InstanceClasses))
	for _, class := range state.InstanceClasses {
		instanceClasses[class.Name] = struct{}{}
	}

	if legacy.MasterNodeGroup != nil {
		if !hasNamedResource(nodeGroups, "master") || !hasNamedResource(instanceClasses, cpapi.BuildInstanceClassName("master")) {
			return false
		}
	}

	for _, nodeGroup := range legacy.NodeGroups {
		name, _ := nodeGroup["name"].(string)
		if name == "" {
			return false
		}

		if !hasNamedResource(nodeGroups, name) || !hasNamedResource(instanceClasses, cpapi.BuildInstanceClassName(name)) {
			return false
		}
	}

	return true
}

func isModuleConfigEnabled(moduleConfig *cpapi.ModuleConfig) bool {
	return moduleConfig.Spec.Enabled != nil && *moduleConfig.Spec.Enabled
}

func hasProviderSettings(moduleConfig *cpapi.ModuleConfig) bool {
	return moduleConfig.Spec.Settings.Provider != nil && len(moduleConfig.Spec.Settings.Provider.Parameters) > 0
}

func hasNamedResource(resources map[string]struct{}, name string) bool {
	_, ok := resources[name]
	return ok
}
