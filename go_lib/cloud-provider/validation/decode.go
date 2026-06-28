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
	"encoding/json"
	"fmt"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

// DecodeCredentialSecret decodes a credential Secret from a Kubernetes object map.
func DecodeCredentialSecret(rawSecret map[string]any) (cpapi.CredentialSecret, error) {
	secret, err := DecodeJSONValue[cpapi.CredentialSecret](rawSecret)
	if err != nil {
		return cpapi.CredentialSecret{}, fmt.Errorf("decode credential secret: %w", err)
	}

	return secret, nil
}

// DecodeCredentialSecrets decodes credential Secrets from CloudProviderVars.
func DecodeCredentialSecrets(rawSecrets map[string]map[string]any) ([]cpapi.CredentialSecret, error) {
	if len(rawSecrets) == 0 {
		return []cpapi.CredentialSecret{}, nil
	}

	credSecrets := make([]cpapi.CredentialSecret, 0, len(rawSecrets))

	for name, rawSecret := range rawSecrets {
		secret, err := DecodeCredentialSecret(rawSecret)
		if err != nil {
			return credSecrets, fmt.Errorf("decode secret %q: %w", name, err)
		}

		credSecrets = append(credSecrets, secret)
	}

	return credSecrets, nil
}

// DecodeNodeGroup decodes NodeGroup resource from CloudProviderVars.
func DecodeNodeGroup(rawNodeGroup map[string]any) (*cpapi.NodeGroup, error) {
	nodeGroup, err := DecodeJSONValue[cpapi.NodeGroup](rawNodeGroup)
	if err != nil {
		return nil, fmt.Errorf("decode node group: %w", err)
	}

	return &nodeGroup, nil
}

// DecodeNodeGroups decodes NodeGroup resources from CloudProviderVars.
func DecodeNodeGroups(rawNodeGroups map[string]map[string]any) ([]cpapi.NodeGroup, error) {
	if len(rawNodeGroups) == 0 {
		return []cpapi.NodeGroup{}, nil
	}

	nodeGroups := make([]cpapi.NodeGroup, 0, len(rawNodeGroups))

	for name, rawNodeGroup := range rawNodeGroups {
		nodeGroup, err := DecodeNodeGroup(rawNodeGroup)
		if err != nil {
			return nodeGroups, fmt.Errorf("decode node group %q: %w", name, err)
		}

		nodeGroups = append(nodeGroups, *nodeGroup)
	}

	return nodeGroups, nil
}

// DecodeInstanceClass decodes InstanceClass resource from CloudProviderVars.
func DecodeInstanceClass(rawInstanceClass map[string]any) (*cpapi.InstanceClass, error) {
	instanceClass, err := DecodeJSONValue[cpapi.InstanceClass](rawInstanceClass)
	if err != nil {
		return nil, fmt.Errorf("decode instance class: %w", err)
	}

	return &instanceClass, nil
}

// DecodeInstanceClasses decodes InstanceClass resources from CloudProviderVars.
func DecodeInstanceClasses(rawInstanceClasses map[string]map[string]any) ([]cpapi.InstanceClass, error) {
	if len(rawInstanceClasses) == 0 {
		return []cpapi.InstanceClass{}, nil
	}

	instanceClasses := make([]cpapi.InstanceClass, 0, len(rawInstanceClasses))

	for name, rawInstanceClass := range rawInstanceClasses {
		instanceClass, err := DecodeInstanceClass(rawInstanceClass)
		if err != nil {
			return instanceClasses, fmt.Errorf("decode instance class %q: %w", name, err)
		}

		instanceClasses = append(instanceClasses, *instanceClass)
	}

	return instanceClasses, nil
}

// DecodeModuleConfig decodes a ModuleConfig resource object from admission or cluster state.
func DecodeModuleConfig(rawModuleConfig map[string]any) (*cpapi.ModuleConfig, error) {
	return DecodeModuleConfigForModule("", rawModuleConfig)
}

// DecodeModuleConfigForModule decodes ModuleConfig from a full CR object or a dhctl settings map.
func DecodeModuleConfigForModule(moduleName string, rawModuleConfig map[string]any) (*cpapi.ModuleConfig, error) {
	if len(rawModuleConfig) == 0 {
		return nil, nil
	}

	if _, hasSpec := rawModuleConfig["spec"]; hasSpec {
		moduleConfig, err := DecodeJSONValue[cpapi.ModuleConfig](rawModuleConfig)
		if err != nil {
			return nil, fmt.Errorf("decode ModuleConfig: %w", err)
		}

		if moduleConfig.Name == "" && moduleName != "" {
			moduleConfig.Name = moduleName
		}

		return &moduleConfig, nil
	}

	settings, err := DecodeJSONValue[cpapi.ModuleConfigSpecSettings](rawModuleConfig)
	if err != nil {
		return nil, fmt.Errorf("decode module settings: %w", err)
	}

	enabled := true
	return &cpapi.ModuleConfig{
		ObjectMeta: cpapi.ObjectMeta{Name: moduleName},
		Spec: cpapi.ModuleConfigSpec{
			Enabled:  &enabled,
			Version:  2,
			Settings: settings,
		},
	}, nil
}

// DecodeJSONValue round-trips an arbitrary value through JSON into type T.
func DecodeJSONValue[T any](value any) (T, error) {
	var out T
	raw, err := json.Marshal(value)
	if err != nil {
		return out, fmt.Errorf("marshal value: %w", err)
	}

	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("unmarshal value: %w", err)
	}

	return out, nil
}
