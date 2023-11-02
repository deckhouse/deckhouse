// Copyright 2023 Flant JSC
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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "service",
			ApiVersion:                   "v1",
			Kind:                         "Service",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(true),
			FilterFunc:                   applyMetricServiceFilter,
		},
	},
}, metricDeprecatedServiceAnnotaion)

type MetricServiceFiltered struct {
	Name                 string
	Namespace            string
	DeprecatedAnnotation bool
}

func applyMetricServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var svc v1.Service
	err := sdk.FromUnstructured(obj, &svc)
	if err != nil {
		return nil, err
	}

	// there is an annotation and publishNotReadyAddresses=false
	if _, isMapContainsKey := svc.Annotations["service.alpha.kubernetes.io/tolerate-unready-endpoints"]; isMapContainsKey && !svc.Spec.PublishNotReadyAddresses {
		return MetricServiceFiltered{Name: svc.Name, Namespace: svc.Namespace, DeprecatedAnnotation: true}, nil
	}

	return MetricServiceFiltered{Name: svc.Name, Namespace: svc.Namespace, DeprecatedAnnotation: false}, nil
}

func metricDeprecatedServiceAnnotaion(input *go_hook.HookInput) error {
	serviceSnap := input.Snapshots["service"]
	if len(serviceSnap) == 0 {
		return nil
	}

	for _, obj := range serviceSnap {
		svc := obj.(MetricServiceFiltered)
		var deprecatedAnnotationMetricValue = 0.0
		if svc.DeprecatedAnnotation {
			deprecatedAnnotationMetricValue = 1.0
		}
		input.MetricsCollector.Set(
			"coredns_service_deprecated_annotation",
			deprecatedAnnotationMetricValue,
			map[string]string{
				"namespace": svc.Namespace,
				"name":      svc.Name,
			},
			metrics.WithGroup("grp_coredns_service_deprecated_annotation"),
		)
	}

	return nil
}
