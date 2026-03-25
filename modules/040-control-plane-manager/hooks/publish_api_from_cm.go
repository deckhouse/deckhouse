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
	Enabled                     *bool        `json:"enabled,omitempty"`
	IngressClass                *string      `json:"ingressClass,omitempty"`
	WhitelistSourceRanges       []string     `json:"whitelistSourceRanges,omitempty"`
	HTTPS                       *HTTPSConfig `json:"https,omitempty"`
	AddKubeconfigGeneratorEntry *bool        `json:"addKubeconfigGeneratorEntry,omitempty"`
}

type HTTPSConfig struct {
	Mode   string       `json:"mode,omitempty"`
	Global *GlobalHTTPS `json:"global,omitempty"`
}

type GlobalHTTPS struct {
	KubeconfigGeneratorMasterCA string `json:"kubeconfigGeneratorMasterCA,omitempty"`
}

const (
	publishAPIIngressConfigPath = "controlPlaneManager.apiserver.publishAPI.ingress."
)

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

	input.Logger.Info("Setting PublishAPI values from 'd8-publishapi-config-migration' configmap.")
	fmt.Println(publishAPIConfig)

	var enabled *bool = publishAPIConfig.Enabled

	var addKubeconfigGeneratorEntryValue *bool = publishAPIConfig.AddKubeconfigGeneratorEntry

	setValueIfNotNil(input, "enabled", enabled)
	setValueIfNotNil(input, "ingressClass", publishAPIConfig.IngressClass)
	setValueIfNotNil(input, "whitelistSourceRanges", publishAPIConfig.WhitelistSourceRanges)
	setValueIfNotNil(input, "addKubeconfigGeneratorEntry", addKubeconfigGeneratorEntryValue)
	setValueIfNotNil(input, "https.mode", publishAPIConfig.HTTPS.Mode)
	setValueIfNotNil(input, "https.global.kubeconfigGeneratorMasterCA", publishAPIConfig.HTTPS.Global.KubeconfigGeneratorMasterCA)

	return nil
}

func setValueIfNotNil(input *go_hook.HookInput, key string, value any) {
	fmt.Printf("Trying to set publishAPI ingress settings key: %s, value %v\n", key, value)
	if value != nil {
		switch v := value.(type) {
		case []interface{}:
			if len(v) > 0 {
				input.Values.Set(publishAPIIngressConfigPath+key, value)
			}
		case []string:
			if len(v) > 0 || key == "https.global.kubeconfigGeneratorMasterCA" {
				input.Values.Set(publishAPIIngressConfigPath+key, value)
			}
		case *bool:
			if v != nil {
				input.Values.Set(publishAPIIngressConfigPath+key, *v)
			}
		case bool:
			input.Values.Set(publishAPIIngressConfigPath+key, v)
		default:
			input.Values.Set(publishAPIIngressConfigPath+key, value)
		}
	}
}
