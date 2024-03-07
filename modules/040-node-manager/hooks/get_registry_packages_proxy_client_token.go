/*
Copyright 2024 Flant JSC

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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "registry_packages_proxy_client_token",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"d8-registry-packages-proxy",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"registry-packages-proxy-client-token",
			}},
			FilterFunc: applyRegistryPackagesProxyClientTokenSecretFilter,
		},
	},
}, handleRegistryPackagesProxyClientTokenSecret)

func applyRegistryPackagesProxyClientTokenSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := v1.Secret{}

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	return string(secret.Data["token"]), nil
}

func handleRegistryPackagesProxyClientTokenSecret(input *go_hook.HookInput) error {
	tokens := input.Snapshots["registry_packages_proxy_client_token"]
	if len(tokens) == 0 {
		if input.Values.Exists("nodeManager.internal.registryPackagesProxyClientToken") {
			input.Values.Remove("nodeManager.internal.registryPackagesProxyClientToken")
		}

		return nil
	}

	input.Values.Set("nodeManager.internal.registryPackagesProxyClientToken", tokens[0].(string))

	return nil
}
