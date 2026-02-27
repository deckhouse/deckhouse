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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/hooks/cluster_configuration"
	v1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/hooks/internal/v1"
)

var _ = cluster_configuration.RegisterHook(func(input *go_hook.HookInput, metaCfg *config.MetaConfig, providerDiscoveryData *unstructured.Unstructured, _ bool) error {
	p := make(map[string]json.RawMessage)
	if metaCfg != nil {
		p = metaCfg.ProviderClusterConfig
	}

	var providerClusterConfiguration v1.DvpProviderClusterConfiguration
	err := convertJSONRawMessageToStruct(p, &providerClusterConfiguration)
	if err != nil {
		return err
	}

	var moduleConfiguration v1.DvpModuleConfiguration
	err = json.Unmarshal([]byte(input.Values.Get("cloudProviderDvp").String()), &moduleConfiguration)
	if err != nil {
		return err
	}

	err = overrideValues(&providerClusterConfiguration, &moduleConfiguration)
	if err != nil {
		return err
	}
	input.Values.Set("cloudProviderDvp.internal.providerClusterConfiguration", providerClusterConfiguration)

	var discoveryData cloudDataV1.DVPCloudProviderDiscoveryData
	if providerDiscoveryData != nil {
		err := sdk.FromUnstructured(providerDiscoveryData, &discoveryData)
		if err != nil {
			return err
		}
	}

	providerDiscoveryDataValuesJSON, ok := input.Values.GetOk("cloudProviderDvp.internal.providerDiscoveryData")
	if ok && len(providerDiscoveryDataValuesJSON.String()) != 0 {
		var providerDiscoveryDataValues cloudDataV1.DVPCloudProviderDiscoveryData
		err = json.Unmarshal([]byte(providerDiscoveryDataValuesJSON.String()), &providerDiscoveryDataValues)
		if err != nil {
			return err
		}
		discoveryData = mergeDiscoveryData(discoveryData, providerDiscoveryDataValues)
	}

	if discoveryData.APIVersion == "" {
		discoveryData.APIVersion = "deckhouse.io/v1"
	}

	if discoveryData.Kind == "" {
		discoveryData.Kind = "DVPCloudDiscoveryData"
	}

	if len(discoveryData.Zones) == 0 {
		discoveryData.Zones = []string{"default"}
	}

	input.Values.Set("cloudProviderDvp.internal.providerDiscoveryData", discoveryData)

	return nil
}, cluster_configuration.NewConfig(infrastructureprovider.MetaConfigPreparatorProvider(infrastructureprovider.NewPreparatorProviderParamsWithoutLogger())))

func convertJSONRawMessageToStruct(in map[string]json.RawMessage, out interface{}) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func overrideValues(p *v1.DvpProviderClusterConfiguration, m *v1.DvpModuleConfiguration) error {
	if m.Provider != nil {
		if p.Provider == nil {
			p.Provider = &v1.DvpProvider{}
		}
		if m.Provider.KubeconfigDataBase64 != nil {
			p.Provider.KubeconfigDataBase64 = m.Provider.KubeconfigDataBase64
		}
		if m.Provider.Namespace != nil {
			p.Provider.Namespace = m.Provider.Namespace
		}
	}

	if m.Zones != nil {
		p.Zones = m.Zones
	}

	if p.Provider == nil {
		return errors.New("provider section is required")
	}
	if p.Provider.KubeconfigDataBase64 == nil || len(*p.Provider.KubeconfigDataBase64) == 0 {
		return errors.New("provider.kubeconfigDataBase64 cannot be empty")
	}
	if p.Provider.Namespace == nil || len(*p.Provider.Namespace) == 0 {
		return errors.New("provider.namespace cannot be empty")
	}

	cloudManaged := p.APIVersion != nil || p.Kind != nil
	if cloudManaged {
		if p.APIVersion == nil || len(*p.APIVersion) == 0 {
			return errors.New("apiVersion cannot be empty")
		}
		if p.Kind == nil || len(*p.Kind) == 0 {
			return errors.New("kind cannot be empty")
		}
		if p.Zones == nil || len(*p.Zones) == 0 {
			def := []string{"default"}
			p.Zones = &def
		}
	}

	return nil
}

func mergeDiscoveryData(newValue cloudDataV1.DVPCloudProviderDiscoveryData, currentValue cloudDataV1.DVPCloudProviderDiscoveryData) cloudDataV1.DVPCloudProviderDiscoveryData {
	result := currentValue
	if newValue.APIVersion != "" && currentValue.APIVersion == "" {
		result.APIVersion = newValue.APIVersion
	}
	if newValue.Kind != "" && currentValue.Kind == "" {
		result.Kind = newValue.Kind
	}
	if newValue.Layout != "" && currentValue.Layout == "" {
		result.Layout = newValue.Layout
	}
	if len(newValue.Zones) > 0 && len(currentValue.Zones) == 0 {
		result.Zones = newValue.Zones
	}
	if len(newValue.StorageClassList) > 0 && len(currentValue.StorageClassList) == 0 {
		result.StorageClassList = newValue.StorageClassList
	}
	return result
}
