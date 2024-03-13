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
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type loadBalancerService struct {
	name     string
	hostname string
	ip       string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:       "/modules/ingress-nginx/service-discover-address",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ingress-loadbalancer-service",
			ApiVersion: "v1",
			Kind:       "Service",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"deckhouse-service-type": "provider-managed",
				},
			},
			FilterFunc: filterIngressServiceAddress,
		},
	},
}, updateIngressAddress)

func filterIngressServiceAddress(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var svc corev1.Service

	if err := sdk.FromUnstructured(obj, &svc); err != nil {
		return nil, err
	}
	if svc.Status.LoadBalancer.Ingress != nil && len(svc.Status.LoadBalancer.Ingress) != 0 {
		return loadBalancerService{
			name:     svc.Labels["name"],
			ip:       svc.Status.LoadBalancer.Ingress[0].IP,
			hostname: svc.Status.LoadBalancer.Ingress[0].Hostname,
		}, nil
	}
	return nil, nil
}

func updateIngressAddress(input *go_hook.HookInput) error {
	snaps := input.Snapshots["ingress-loadbalancer-service"]
	for _, snap := range snaps {
		if snap == nil {
			continue
		}
		svc := snap.(loadBalancerService)
		patch := map[string]interface{}{
			"status": map[string]interface{}{
				"loadBalancer": map[string]interface{}{
					"ip":       svc.ip,
					"hostname": svc.hostname,
				},
			},
		}
		input.PatchCollector.MergePatch(patch, "deckhouse.io/v1", "IngressNginxController",
			"", svc.name, object_patch.IgnoreMissingObject(), object_patch.WithSubresource("/status"))
	}
	return nil
}
