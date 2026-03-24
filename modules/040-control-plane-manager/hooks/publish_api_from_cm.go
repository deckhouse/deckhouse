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

package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cm_publishapi_config_migration",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-publishapi-config-migration"},
			},
			FilterFunc: filterPublishAPIConfigMap,
		},
	},
}, handlePublishAPIConfig)

type Config struct {
	Enabled                     bool        `json:"enabled,omitempty"`
	IngressClass                string      `json:"ingressClass,omitempty"`
	WhitelistSourceRanges       []string    `json:"whitelistSourceRanges,omitempty"`
	HTTPS                       HTTPSConfig `json:"https,omitempty"`
	AddKubeconfigGeneratorEntry bool        `json:"addKubeconfigGeneratorEntry,omitempty"`
}

type HTTPSConfig struct {
	Mode   string       `json:"mode,omitempty"`
	Global *GlobalHTTPS `json:"global,omitempty"`
}

type GlobalHTTPS struct {
	KubeconfigGeneratorMasterCA string `json:"kubeconfigGeneratorMasterCA,omitempty"`
}

func filterPublishAPIConfigMap(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1.ConfigMap

	err := sdk.FromUnstructured(unstructured, &cm)
	if err != nil {
		return nil, err
	}

	var dataStruct Config
	if data, ok := cm.Data["config"]; ok {
		err = json.Unmarshal([]byte(data), &dataStruct)
		if err != nil {
			return nil, fmt.Errorf("invalid PublishAPI config format - json expected: %s", err)
		}
	}

	return dataStruct, nil
}

func handlePublishAPIConfig(_ context.Context, input *go_hook.HookInput) error {
	if input.ConfigValues.Get("controlPlaneManager.apiserver.publishAPI.ingress").Exists() {
		input.Logger.Info("Publish API ingress settings are set in moduleconfig control-plane-manager, skipping")
		return nil
	}
	input.Logger.Info("Unmarshalling")

	publishAPIConfigSnaps, err := sdkobjectpatch.UnmarshalToStruct[Config](input.Snapshots, "cm_publishapi_config_migration")
	if err != nil {
		return fmt.Errorf("failed to unmarshal cm_publishapi_config_migration snapshot: %w", err)
	}
	publishAPIConfig := publishAPIConfigSnaps[0]

	configMap := map[string]interface{}{
		"enabled":                     publishAPIConfig.Enabled,
		"ingressClass":                publishAPIConfig.IngressClass,
		"whitelistSourceRanges":       publishAPIConfig.WhitelistSourceRanges,
		"addKubeconfigGeneratorEntry": publishAPIConfig.AddKubeconfigGeneratorEntry,
		"https": map[string]interface{}{
			"mode":   publishAPIConfig.HTTPS.Mode,
			"global": publishAPIConfig.HTTPS.Global,
		},
	}

	// Clean nil values
	strippedConfig := stripNilValues(configMap)
	fmt.Println(strippedConfig)
	// strippedConfig["enabled"] =//  publishAPIConfig.Enabled

	// if publishAPIConfig.IngressClass != "" {
	// 	ingressConfig["ingressClass"] = publishAPIConfig.IngressClass
	// }

	// if publishAPIConfig.WhitelistSourceRanges != nil {
	// 	ingressConfig["whitelistSourceRanges"] = publishAPIConfig.WhitelistSourceRanges
	// } else {
	// }

	// httpsConfig := map[string]interface{}{
	// 	"mode": publishAPIConfig.HTTPS.Mode,
	// }

	// if publishAPIConfig.HTTPS.Global != nil {
	// 	httpsConfig["global"] = map[string]interface{}{
	// 		"kubeconfigGeneratorMasterCA": publishAPIConfig.HTTPS.Global.KubeconfigGeneratorMasterCA,
	// 	}
	// }

	// ingressConfig["https"] = httpsConfig
	// ingressConfig["addKubeconfigGeneratorEntry"] = publishAPIConfig.AddKubeconfigGeneratorEntry
	input.Logger.Info("Setting PublishAPI values from 'd8-publishapi-config-migration' configmap.")
	input.Values.Set("controlPlaneManager.apiserver.publishAPI.ingress", strippedConfig)

	return nil
}

func stripNilValues(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			if value != nil {
				result[key] = stripNilValues(value)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(v))
		for _, item := range v {
			if item != nil {
				result = append(result, stripNilValues(item))
			}
		}
		return result
	default:
		return data
	}
}
