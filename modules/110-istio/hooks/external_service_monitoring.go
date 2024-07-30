/*
Copyright 2023 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// There is an issue in [istio](https://github.com/istio/istio/issues/20703) with [staled solution](https://github.com/istio/istio/issues/37331)
// istio renders for External Services with ports listener "0.0.0.0:port" which catch all the traffic to the port. It is a problem for services out of istio registry.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/istio/external-service-monitoring",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "services",
			ApiVersion: "v1",
			Kind:       "Service",
			FilterFunc: applyServiceFilter,
			// ignore d8 services, we don't use misconfigured external services
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"deckhouse"},
					},
				}},
		},
	},
}, handleExternalNameService)

func handleExternalNameService(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("d8_istio_service")
	snapshot := input.Snapshots["services"]

	for _, snap := range snapshot {
		if snap == nil {
			continue
		}

		service := snap.(externalService)

		input.MetricsCollector.Set("d8_istio_irrelevant_service", 1, map[string]string{"namespace": service.Namespace, "name": service.Name}, metrics.WithGroup("d8_istio_service"))
	}

	return nil
}

func applyServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var service v1.Service

	err := sdk.FromUnstructured(obj, &service)
	if err != nil {
		return nil, err
	}

	if service.Spec.Type != v1.ServiceTypeExternalName {
		return nil, nil
	}

	if len(service.Spec.Ports) == 0 {
		return nil, nil
	}

	return externalService{
		Namespace: service.Namespace,
		Name:      service.Name,
	}, nil
}

type externalService struct {
	Namespace string
	Name      string
}
