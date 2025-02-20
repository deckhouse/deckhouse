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
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	CheckCloudProviderConfigRaw = "checkCloudProviderConfigRaw"
	CheckStaticClusterConfigRaw = "checkStaticClusterConfigRaw"
	CheckClusterConfigRaw       = "checkClusterConfigRaw"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/requirements/check-config",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "provider_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"d8-provider-cluster-configuration",
			}},
			FilterFunc: applyProviderClusterConfigurationSecretFilter,
		},
	},
}, CheckCloudProviderConfig)

func applyProviderClusterConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret = &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret from unstructured: %v", err)
	}

	return secret, nil
}

func CheckCloudProviderConfig(input *go_hook.HookInput) error {
	snap := input.Snapshots["provider_cluster_configuration"]
	if len(snap) > 0 {
		secret := snap[0].(*v1.Secret)
		if YAML, ok := secret.Data["cloud-provider-cluster-configuration.yaml"]; ok && len(YAML) > 0 {
			err := config.CheckParseConfigFromData(string(YAML))
			if err != nil {
				requirements.SaveValue(CheckCloudProviderConfigRaw, true)
				input.MetricsCollector.Set("d8_check_cloud_provider_config", 1, nil)
				err1 := fmt.Errorf(findErrorLines(err.Error()))
				return err1
			}
		}
	}
	requirements.SaveValue(CheckCloudProviderConfigRaw, false)
	input.MetricsCollector.Expire("d8_check_cloud_provider_config")
	return nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/requirements/check-config",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "static_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"kube-system",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"d8-static-cluster-configuration",
			}},
			FilterFunc: applyProviderClusterConfigurationSecretFilter,
		},
	},
}, CheckStaticClusterConfig)

func CheckStaticClusterConfig(input *go_hook.HookInput) error {
	snap := input.Snapshots["static_cluster_configuration"]
	if len(snap) > 0 {
		secret := snap[0].(*v1.Secret)
		if YAML, ok := secret.Data["static-cluster-configuration.yaml"]; ok && len(YAML) > 0 {
			err := config.CheckParseConfigFromData(string(YAML))
			if err != nil {
				requirements.SaveValue(CheckStaticClusterConfigRaw, true)
				input.MetricsCollector.Set("d8_check_static_cluster_config", 1, nil)
				return fmt.Errorf(findErrorLines(err.Error()))
			}
		}
	}
	input.Logger.Info("1007")
	requirements.SaveValue(CheckStaticClusterConfigRaw, false)
	input.MetricsCollector.Expire("d8_check_static_cluster_config")
	return nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/requirements/check-config",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"kube-system",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"d8-cluster-configuration",
			}},
			FilterFunc: applyProviderClusterConfigurationSecretFilter,
		},
	},
}, CheckClusterConfig)

func CheckClusterConfig(input *go_hook.HookInput) error {
	snap := input.Snapshots["cluster_configuration"]
	if len(snap) > 0 {
		secret := snap[0].(*v1.Secret)
		if YAML, ok := secret.Data["cluster-configuration.yaml"]; ok && len(YAML) > 0 {
			err := config.CheckParseConfigFromData(string(YAML))
			if err != nil {
				requirements.SaveValue(CheckStaticClusterConfigRaw, true)
				input.MetricsCollector.Set("d8_check_cluster_config", 1, nil)
				return fmt.Errorf(findErrorLines(err.Error()))
			}
		}
	}
	requirements.SaveValue(CheckStaticClusterConfigRaw, false)
	input.MetricsCollector.Expire("d8_check_cluster_config")
	return nil
}

func findErrorLines(text string) string {
	lines := strings.Split(text, "\n")
	var result strings.Builder
	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], "error occurred:") {
			result.WriteString(lines[i] + "\n")
			if i+1 < len(lines) {
				result.WriteString(lines[i+1] + "\n")
			}
		}
	}
	return result.String()
}
