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

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
)

type legacyProviderClusterConfig struct {
	MasterNodeGroup map[string]any   `json:"masterNodeGroup,omitempty"`
	NodeGroups      []map[string]any `json:"nodeGroups,omitempty"`
}

// MigrationStatusFromState derives migration status from the decoded validation state.
func MigrationStatusFromState(state *cpval.State) cpapi.MigrationStatus {
	if state == nil || len(state.LegacyProviderClusterConfig) == 0 {
		return cpapi.MigrationStatus{}
	}

	complete := isNewResourcesComplete(state)
	return cpapi.MigrationStatus{
		LegacyPCCPresent:     true,
		NewResourcesComplete: complete,
		MigrationPending:     !complete,
	}
}

func isNewResourcesComplete(state *cpval.State) bool {
	if state.ModuleConfig == nil ||
		state.ModuleConfig.Spec.Version < 2 ||
		state.ModuleConfig.Spec.Enabled == nil ||
		!*state.ModuleConfig.Spec.Enabled ||
		!hasProviderSettings(state.ModuleConfig.Spec.RawSettings()) {
		return false
	}

	if _, found := findCredentialSecret(state.CredentialSecrets, cpapi.CredentialSecretName); !found {
		return false
	}

	legacy, err := cpval.DecodeJSONValue[legacyProviderClusterConfig](state.LegacyProviderClusterConfig)
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

	if len(legacy.MasterNodeGroup) > 0 {
		if !hasNamedResource(nodeGroups, "master") || !hasNamedResource(instanceClasses, "master-dvp") {
			return false
		}
	}

	for _, nodeGroup := range legacy.NodeGroups {
		name, _ := nodeGroup["name"].(string)
		if name == "" {
			return false
		}

		if !hasNamedResource(nodeGroups, name) || !hasNamedResource(instanceClasses, fmt.Sprintf("%s-dvp", name)) {
			return false
		}
	}

	return true
}

func hasNamedResource(resources map[string]struct{}, name string) bool {
	_, ok := resources[name]
	return ok
}

func hasProviderSettings(settings map[string]any) bool {
	provider, ok := settings["provider"].(map[string]any)
	if !ok {
		return false
	}

	parameters, ok := provider["parameters"].(map[string]any)

	return ok && len(parameters) > 0
}
