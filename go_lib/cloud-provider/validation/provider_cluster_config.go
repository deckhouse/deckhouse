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
	"strings"
)

const providerClusterConfigurationPath = "ProviderClusterConfiguration"

// ValidateProviderClusterConfigKubeconfig checks validity of kubeconfig in legacy ProviderClusterConfiguration.
// pathToKubeconfig is a dot-separated path inside LegacyProviderClusterConfig, for example "provider.kubeconfigDataBase64".
// When the path is absent, validation is skipped. When the path is present, the value must be a non-empty
// base64-encoded kubeconfig.
func ValidateProviderClusterConfigKubeconfig(state *State, pathToKubeconfig string) Result {
	result := Result{}
	if state == nil || len(state.LegacyProviderClusterConfig) == 0 || pathToKubeconfig == "" {
		return result
	}

	value, ok := lookupMapStringPath(state.LegacyProviderClusterConfig, pathToKubeconfig)
	if !ok {
		return result
	}

	errorPath := providerClusterConfigurationPath + "." + pathToKubeconfig
	kubeconfigB64 := strings.TrimSpace(value)
	if kubeconfigB64 == "" {
		result.AddError(errorPath, "pcc_kubeconfig_required", pathToKubeconfig+" must be set")
		return result
	}

	if err := validateKubeconfigBase64(kubeconfigB64); err != nil {
		result.AddError(errorPath, "invalid_pcc_kubeconfig", "must contain base64-encoded kubeconfig")
	}

	return result
}

func lookupMapStringPath(data map[string]any, path string) (string, bool) {
	if data == nil || path == "" {
		return "", false
	}

	current := any(data)
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return "", false
		}

		value, ok := object[part]
		if !ok {
			return "", false
		}

		current = value
	}

	value, ok := current.(string)
	return value, ok
}
