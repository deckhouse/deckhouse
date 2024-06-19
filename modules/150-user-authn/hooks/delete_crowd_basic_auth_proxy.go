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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TODO(ipaqsa): can be deleted after 1.65
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/user-authn/delete-crowd-proxy-ingress",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "crowd-proxy-ingress",
			ApiVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			FilterFunc: filterIngressName,
		},
	},
}, deleteCrowdIngress)

func filterIngressName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ingress v1.Ingress
	if err := sdk.FromUnstructured(obj, &ingress); err != nil {
		return nil, err
	}
	if ingress.Name != "crowd-basic-auth-proxy" {
		return nil, nil
	}
	return ingress.Name, nil
}

func deleteCrowdIngress(input *go_hook.HookInput) (err error) {
	for _, snap := range input.Snapshots["crowd-proxy-ingress"] {
		if snap == nil {
			continue
		}
		input.PatchCollector.Delete("networking.k8s.io/v1", "Ingress", "d8-user-authn", snap.(string))
	}
	return nil
}
