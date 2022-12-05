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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "metrics",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "service",
			ApiVersion: "v1",
			Kind:       "Service",
			FilterFunc: applyLegacyServiceAnnotationFilter,
		},
	},
}, legacyServiceAnnotation)

const (
	legacyServiceAnnotationGroup      = "kube_dns_legacy_service_annotation"
	legacyServiceAnnotationMetricName = "d8_kube_dns_deprecated_service_annotation"
)

func legacyServiceAnnotation(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(legacyServiceAnnotationGroup)

	snap := input.Snapshots["service"]
	for _, obj := range snap {
		if obj == nil {
			continue
		}

		svc := obj.(*Service)
		input.MetricsCollector.Set(legacyServiceAnnotationMetricName, 1, map[string]string{"service_namespace": svc.Namespace, "service_name": svc.Name}, metrics.WithGroup(legacyServiceAnnotationGroup))
	}

	return nil
}

func applyLegacyServiceAnnotationFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	svc := &v1.Service{}
	err := sdk.FromUnstructured(obj, svc)
	if err != nil {
		return nil, err
	}

	service := &Service{
		Name:      svc.Name,
		Namespace: svc.Namespace,
	}

	if _, ok := svc.Annotations["service.alpha.kubernetes.io/tolerate-unready-endpoints"]; ok {
		// we'll skip Services with a proper spec field set
		if svc.Spec.PublishNotReadyAddresses {
			return nil, nil
		}

		return service, err
	}

	return nil, nil
}

type Service struct {
	Name      string
	Namespace string
}
