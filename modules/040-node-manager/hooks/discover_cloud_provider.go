/*
Copyright 2021 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyCloudProviderSecretFilter loads data section from Secret and tries to decode json in all top fields.
func applyCloudProviderSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return decodeDataFromSecret(obj)
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
	Queue:        "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cloud_provider_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"kube-system",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"d8-node-manager-cloud-provider",
			}},
			FilterFunc: applyCloudProviderSecretFilter,
		},
	},
}, discoverCloudProviderHandler)

func discoverCloudProviderHandler(input *go_hook.HookInput) error {
	secret := input.Snapshots["cloud_provider_secret"]
	if len(secret) == 0 {
		if input.Values.Exists("nodeManager.internal.cloudProvider") {
			input.Values.Remove("nodeManager.internal.cloudProvider")
		}
		return nil
	}
	input.Values.Set("nodeManager.internal.cloudProvider", secret[0])
	return nil
}
