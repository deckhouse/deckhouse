// Copyright 2021 Flant JSC
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

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

var (
	cloudProviderNameToModule = map[string]string{
		"OpenStack": "cloudProviderOpenstack",
		"AWS":       "cloudProviderAws",
		"GCP":       "cloudProviderGcp",
		"Yandex":    "cloudProviderYandex",
		"vSphere":   "cloudProviderVsphere",
		"Azure":     "cloudProviderAzure",
		"VCD":       "cloudProviderVcd",
		"Zvirt":     "cloudProviderZvirt",
	}
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cloud_config",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cluster-configuration"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: applyClusterConfigForProviderFilter,
		},
	},
}, enableCloudProvider)

func applyClusterConfigForProviderFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1core.Secret
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return "", err
	}

	clusterConf, ok := cm.Data["cluster-configuration.yaml"]
	if !ok {
		return "", nil
	}

	var parsedClusterConfig map[string]interface{}
	if err := yaml.Unmarshal(clusterConf, &parsedClusterConfig); err != nil {
		return nil, fmt.Errorf("cannot parse cluster configuration: %v", err)
	}

	if cloudConfig, ok := parsedClusterConfig["cloud"]; ok {
		provider, ok := cloudConfig.(map[string]interface{})["provider"]
		if ok {
			return provider.(string), nil
		}
	}

	return "", nil
}

func enableCloudProvider(input *go_hook.HookInput) error {
	cloudConfigSnap := input.Snapshots["cloud_config"]

	providerNameToEnable := ""

	if len(cloudConfigSnap) > 0 {
		providerNameToEnable = cloudConfigSnap[0].(string)
	} else {
		for providerName, module := range cloudProviderNameToModule {
			if input.ConfigValues.Exists(module) {
				providerNameToEnable = providerName
				break
			}
		}
	}

	for providerName, module := range cloudProviderNameToModule {
		moduleEnable := fmt.Sprintf("%sEnabled", module)
		if providerName == providerNameToEnable {
			input.Values.Set(moduleEnable, true)
		} else {
			input.Values.Remove(moduleEnable)
		}
	}

	return nil
}
