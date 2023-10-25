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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

func applyServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	service := &v1.Service{}
	err := sdk.FromUnstructured(obj, service)
	if err != nil {
		return nil, fmt.Errorf("cannot create service object: %v", err)
	}

	return service.Spec.Type == v1.ServiceTypeClusterIP, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},

	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kube_dns_svc",
			ApiVersion: "v1",
			Kind:       "Service",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kube-dns"},
			},
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   applyServiceFilter,
		},
	},
}, removeKubeDNSDeployAndService)

func removeKubeDNSDeployAndService(input *go_hook.HookInput) error {
	input.PatchCollector.Delete("apps/v1", "Deployment", "kube-system", "coredns", object_patch.NonCascading())

	kubeDNSSVCIsClusterIPTypeSnap := input.Snapshots["kube_dns_svc"]
	if len(kubeDNSSVCIsClusterIPTypeSnap) > 0 {
		if kubeDNSSVCIsClusterIPTypeSnap[0].(bool) {
			input.PatchCollector.Delete("v1", "Service", "kube-system", "kube-dns")
		}
	}

	return nil
}
