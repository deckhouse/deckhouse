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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

// BuildStateFromProtocolInput decodes dhctl provider input into a validation State.
func BuildStateFromProtocolInput(
	moduleName string,
	input proto.PrepareInput,
	vars *proto.CloudProviderVars,
) (*State, error) {
	moduleConfig, err := DecodeModuleConfig(moduleName, input.ModuleConfig)
	if err != nil {
		return nil, err
	}

	credSecrets, err := DecodeCredentialSecrets(vars)
	if err != nil {
		return nil, err
	}

	nodeGroups, err := DecodeNodeGroups(vars)
	if err != nil {
		return nil, err
	}

	instanceClasses, err := DecodeInstanceClasses(vars)
	if err != nil {
		return nil, err
	}

	return &State{
		ModuleConfig:                moduleConfig,
		CredentialSecrets:           credSecrets,
		NodeGroups:                  nodeGroups,
		InstanceClasses:             instanceClasses,
		LegacyProviderClusterConfig: input.ProviderClusterConfig,
	}, nil
}

// DecodeCredentialSecrets decodes credential Secrets from CloudProviderVars.
func DecodeCredentialSecrets(vars *proto.CloudProviderVars) ([]cpapi.CredentialSecret, error) {
	credSecrets := make([]cpapi.CredentialSecret, 0)

	if vars == nil {
		return credSecrets, nil
	}

	for _, rawSecret := range vars.Secrets {
		secret, err := DecodeJSONValue[corev1.Secret](rawSecret)
		if err != nil {
			return credSecrets, fmt.Errorf("decode secret: %w", err)
		}

		credSecrets = append(credSecrets, cpapi.SecretToCredentialSecret(&secret))
	}

	return credSecrets, nil
}

// DecodeNodeGroups decodes NodeGroup resources from CloudProviderVars.
func DecodeNodeGroups(vars *proto.CloudProviderVars) ([]cpapi.NodeGroup, error) {
	nodeGroups := make([]cpapi.NodeGroup, 0)

	if vars == nil {
		return nodeGroups, nil
	}

	for _, rawNodeGroup := range vars.NodeGroups {
		nodeGroup, err := DecodeJSONValue[cpapi.NodeGroup](rawNodeGroup)
		if err != nil {
			return nodeGroups, fmt.Errorf("decode node group: %w", err)
		}

		nodeGroups = append(nodeGroups, nodeGroup)
	}

	return nodeGroups, nil
}

// DecodeInstanceClasses decodes InstanceClass resources from CloudProviderVars.
func DecodeInstanceClasses(vars *proto.CloudProviderVars) ([]cpapi.InstanceClass, error) {
	instanceClasses := make([]cpapi.InstanceClass, 0)

	if vars == nil {
		return instanceClasses, nil
	}

	for _, rawInstanceClass := range vars.InstanceClasses {
		instanceClass, err := DecodeJSONValue[cpapi.InstanceClass](rawInstanceClass)
		if err != nil {
			return instanceClasses, fmt.Errorf("decode instance class: %w", err)
		}

		instanceClasses = append(instanceClasses, instanceClass)
	}

	return instanceClasses, nil
}

// DecodeModuleConfig decodes ModuleConfig from either a full object or a settings map.
func DecodeModuleConfig(moduleName string, raw map[string]any) (*cpapi.ModuleConfig, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	if _, hasSpec := raw["spec"]; hasSpec {
		moduleConfig, err := DecodeJSONValue[cpapi.ModuleConfig](raw)
		if err != nil {
			return nil, fmt.Errorf("decode ModuleConfig object: %w", err)
		}

		if moduleConfig.Name == "" {
			moduleConfig.Name = moduleName
		}

		return &moduleConfig, nil
	}

	settings, err := DecodeJSONValue[cpapi.ModuleConfigSpecSettings](raw)
	if err != nil {
		return nil, fmt.Errorf("decode module settings: %w", err)
	}

	spec := cpapi.ModuleConfigSpec{
		Enabled:  ptr.To(true),
		Version:  2,
		Settings: settings,
	}
	spec.SetRawSettings(raw)

	return &cpapi.ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{Name: moduleName},
		Spec:       spec,
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
