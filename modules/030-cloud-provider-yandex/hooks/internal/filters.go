/*
Copyright 2026 Flant JSC

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

package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

type PCCSecretFilterResult struct {
	ProviderClusterConfig     map[string]json.RawMessage `json:"providerClusterConfig,omitempty"`
	ProviderDiscoveryDataJSON json.RawMessage            `json:"providerDiscoveryDataJSON,omitempty"`
}

type CandiDiscoveryDataFilterResult struct {
	DiscoveryDataJSON json.RawMessage `json:"discoveryDataJSON,omitempty"`
}

type ModuleConfigFilterResult struct {
	Version    int64           `json:"version"`
	Enabled    bool            `json:"enabled"`
	SettingsV1 json.RawMessage `json:"settingsV1,omitempty"`
	SettingsV2 json.RawMessage `json:"settingsV2,omitempty"`
}

type NamedResourceFilterResult struct {
	Name string `json:"name"`
}

type CredentialsSecretFilterResult struct {
	Name string `json:"name"`
}

func FilterPCCSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// The fake k8s dynamic client ignores field selectors, so we guard by name here.
	if obj.GetName() != PCCSecretName {
		return nil, nil
	}

	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("cannot convert PCC secret from unstructured: %v", err)
	}

	result := &PCCSecretFilterResult{}

	if discoveryDataJSON, ok := secret.Data[PCCDiscoveryDataFilename]; ok && len(discoveryDataJSON) > 0 {
		if _, err := config.ValidateDiscoveryData(&discoveryDataJSON, nil, nil); err != nil {
			return nil, fmt.Errorf("validate cloud-provider-discovery-data.json: %v", err)
		}
		result.ProviderDiscoveryDataJSON = json.RawMessage(discoveryDataJSON)
	}

	if clusterConfigYAML, ok := secret.Data[PCCClusterConfigFilename]; ok && len(clusterConfigYAML) > 0 {
		m, err := config.ParseConfigFromData(
			context.Background(),
			string(clusterConfigYAML),
			infrastructureprovider.MetaConfigValidatorProvider(),
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("validate cloud-provider-cluster-configuration.yaml: %v", err)
		}
		result.ProviderClusterConfig = m.ProviderClusterConfig
	}

	return result, nil
}

func FilterModuleConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &deckhousev1alpha1.ModuleConfig{}
	if err := sdk.FromUnstructured(obj, mc); err != nil {
		return nil, fmt.Errorf("convert ModuleConfig from unstructured: %w", err)
	}

	result := ModuleConfigFilterResult{
		Version: int64(mc.Spec.Version),
		Enabled: mc.Spec.Enabled != nil && *mc.Spec.Enabled,
	}

	if mc.Spec.Settings != nil {
		settings := mc.Spec.Settings.GetMap()
		settingsJSON, err := json.Marshal(settings)
		if err != nil {
			return nil, fmt.Errorf("marshal ModuleConfig settings: %w", err)
		}
		switch mc.Spec.Version {
		case 1:
			result.SettingsV1 = json.RawMessage(settingsJSON)
		case 2:
			result.SettingsV2 = json.RawMessage(settingsJSON)
		}
	}

	return result, nil
}

func FilterCredentialSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, err
	}

	if secret.Type != cpapi.CredentialsSecretType {
		return nil, nil
	}

	return NamedResourceFilterResult{Name: secret.Name}, nil
}

func FilterNamedResource(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return NamedResourceFilterResult{Name: obj.GetName()}, nil
}

func FilterCandiDiscoverySecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// The fake k8s dynamic client ignores field selectors, so we guard by name here.
	if obj.GetName() != CandiDiscoverySecretName {
		return nil, nil
	}

	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("cannot convert candi discovery secret from unstructured: %v", err)
	}

	discoveryDataJSON, ok := secret.Data["cloud-provider-discovery-data.json"]
	if !ok || len(discoveryDataJSON) == 0 {
		return CandiDiscoveryDataFilterResult{}, nil
	}

	if _, err := config.ValidateDiscoveryData(&discoveryDataJSON, nil, nil); err != nil {
		return nil, fmt.Errorf("validate candi cloud-provider-discovery-data.json: %v", err)
	}

	return CandiDiscoveryDataFilterResult{
		DiscoveryDataJSON: json.RawMessage(discoveryDataJSON),
	}, nil
}
