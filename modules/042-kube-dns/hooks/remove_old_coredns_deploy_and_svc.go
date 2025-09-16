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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
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
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   applyServiceFilter,
		},
	},
}, removeKubeDNSDeployAndService)

func removeKubeDNSDeployAndService(_ context.Context, input *go_hook.HookInput) error {
	input.PatchCollector.DeleteNonCascading("apps/v1", "Deployment", "kube-system", "coredns")

	kubeDNSSVCIsClusterIPTypeSnap := input.Snapshots.Get("kube_dns_svc")
	if len(kubeDNSSVCIsClusterIPTypeSnap) > 0 {
		var startKubeDNSSVCIsClusterIPTypeSnap bool
		err := kubeDNSSVCIsClusterIPTypeSnap[0].UnmarshalTo(&startKubeDNSSVCIsClusterIPTypeSnap)
		if err != nil {
			return fmt.Errorf("failed to unmarshal kube_dns_svc snapshot: %w", err)
		}

		if startKubeDNSSVCIsClusterIPTypeSnap {
			input.PatchCollector.Delete("v1", "Service", "kube-system", "kube-dns")
		}
	}

	return nil
}
