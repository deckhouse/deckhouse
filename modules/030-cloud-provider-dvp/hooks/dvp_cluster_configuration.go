/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/go_lib/hooks/cluster_configuration"
)

type dvpModuleConfiguration struct {
	Provider *dvpProviderModuleConfiguration `json:"provider,omitempty"`
}

type dvpProviderModuleConfiguration struct {
	KubeconfigDataBase64 *string `json:"kubeconfigDataBase64,omitempty"`
	Namespace            *string `json:"namespace,omitempty"`
}

var _ = cluster_configuration.RegisterHook(func(input *go_hook.HookInput, metaCfg *config.MetaConfig, providerDiscoveryData *unstructured.Unstructured, secretFound bool) error {
	if !secretFound {
		return fmt.Errorf("kube-system/d8-provider-cluster-configuration secret not found")
	}

	providerClusterConfiguration, err := rawMessageMapToInterfaceMap(metaCfg.ProviderClusterConfig)
	if err != nil {
		return fmt.Errorf("convert ProviderClusterConfig: %w", err)
	}

	var moduleConfiguration dvpModuleConfiguration
	if s := input.Values.Get("cloudProviderDvp").String(); len(s) != 0 {
		if err := json.Unmarshal([]byte(s), &moduleConfiguration); err != nil {
			return fmt.Errorf("unmarshal cloudProviderDvp values: %w", err)
		}
	}

	if err := overrideValues(providerClusterConfiguration, &moduleConfiguration); err != nil {
		return err
	}

	input.Values.Set("cloudProviderDvp.internal.providerClusterConfiguration", providerClusterConfiguration)

	var discoveryData map[string]any
	if providerDiscoveryData != nil {
		discoveryData = providerDiscoveryData.Object
	} else {
		discoveryData = map[string]any{}
	}

	if v, ok := input.Values.GetOk("cloudProviderDvp.internal.providerDiscoveryData"); ok && len(v.String()) != 0 {
		var valuesDiscoveryData map[string]any
		if err := json.Unmarshal([]byte(v.String()), &valuesDiscoveryData); err != nil {
			return fmt.Errorf("unmarshal cloudProviderDvp.internal.providerDiscoveryData: %w", err)
		}
		discoveryData = mergeMapsPreferNonEmpty(discoveryData, valuesDiscoveryData)
	}

	input.Values.Set("cloudProviderDvp.internal.providerDiscoveryData", discoveryData)

	return nil
}, cluster_configuration.NewConfig(infrastructureprovider.MetaConfigPreparatorProvider(infrastructureprovider.NewPreparatorProviderParamsWithoutLogger())))

func rawMessageMapToInterfaceMap(in map[string]json.RawMessage) (map[string]any, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any)
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func overrideValues(p map[string]any, m *dvpModuleConfiguration) error {
	getOrCreateMap := func(parent map[string]any, key string) map[string]any {
		if v, ok := parent[key]; ok {
			if mm, ok := v.(map[string]any); ok {
				return mm
			}
		}
		mm := map[string]any{}
		parent[key] = mm
		return mm
	}

	if m != nil && m.Provider != nil {
		provider := getOrCreateMap(p, "provider")

		if m.Provider.KubeconfigDataBase64 != nil {
			provider["kubeconfigDataBase64"] = *m.Provider.KubeconfigDataBase64
		}
		if m.Provider.Namespace != nil {
			provider["namespace"] = *m.Provider.Namespace
		}
	}

	provider, _ := p["provider"].(map[string]any)
	if provider == nil {
		return errors.New("provider section is required")
	}
	if s, _ := provider["kubeconfigDataBase64"].(string); len(s) == 0 {
		return errors.New("provider.kubeconfigDataBase64 cannot be empty")
	}
	if s, _ := provider["namespace"].(string); len(s) == 0 {
		return errors.New("provider.namespace cannot be empty")
	}

	return nil
}

func mergeMapsPreferNonEmpty(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for k, v := range src {
		if v == nil {
			continue
		}

		if srcMap, ok := v.(map[string]any); ok {
			if dstMap, ok := dst[k].(map[string]any); ok {
				dst[k] = mergeMapsPreferNonEmpty(dstMap, srcMap)
				continue
			}
			if isEmptyValue(dst[k]) {
				dst[k] = mergeMapsPreferNonEmpty(map[string]any{}, srcMap)
			}
			continue
		}

		if srcArr, ok := v.([]any); ok {
			if isEmptyArray(dst[k]) {
				dst[k] = srcArr
			}
			continue
		}

		if isEmptyValue(dst[k]) {
			dst[k] = v
		}
	}
	return dst
}

func isEmptyArray(v any) bool {
	if v == nil {
		return true
	}
	if arr, ok := v.([]any); ok {
		return len(arr) == 0
	}
	return false
}

func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	default:
		return false
	}
}
