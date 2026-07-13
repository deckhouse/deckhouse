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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	disableLoadBalancerConfigMapName      = "dvp-cloud-controller-manager-disable-lb"
	disableLoadBalancerConfigMapNamespace = "d8-cloud-provider-dvp"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "disable_load_balancer_config_map",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{disableLoadBalancerConfigMapNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{disableLoadBalancerConfigMapName},
			},
			FilterFunc: applyDisableLoadBalancerConfigMapFilter,
		},
	},
}, handleDisableLoadBalancer)

func applyDisableLoadBalancerConfigMapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func handleDisableLoadBalancer(_ context.Context, input *go_hook.HookInput) error {
	disable := len(input.Snapshots.Get("disable_load_balancer_config_map")) > 0
	input.Values.Set("cloudProviderDvp.internal.disableLoadBalancer", disable)
	return nil
}
