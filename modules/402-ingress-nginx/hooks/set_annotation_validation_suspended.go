/*
Copyright 2025 Flant JSC

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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
)

const validationSuspendMetricName = "ingress_nginx_validation_suspended"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/ingress-nginx",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "ingressNginxControllers",
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "IngressNginxController",
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   setAnnotationValidationSuspendedFilterIngressNginxController,
		},
		{
			Name:       "ingressNginxControllersConfigMap",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"ingress-nginx-validation-suspended"},
			},
			NamespaceSelector:            internal.NsSelector(),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   setAnnotationValidationSuspendedFilterConfigMap,
		},
	},
}, setAnnotationValidationSuspendedHandleIngressNginxControllers)

func setAnnotationValidationSuspendedFilterConfigMap(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1.ConfigMap
	if err := sdk.FromUnstructured(obj, &cm); err != nil {
		return nil, err
	}
	return cm, nil
}

func setAnnotationValidationSuspendedFilterIngressNginxController(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ctrl internal.IngressNginxController
	if err := sdk.FromUnstructured(obj, &ctrl); err != nil {
		return nil, err
	}
	return ctrl, nil
}

func setAnnotationValidationSuspendedHandleIngressNginxControllers(_ context.Context, input *go_hook.HookInput) error {
	controllersSnapshot := input.Snapshots.Get("ingressNginxControllers")
	configMapSnapshot := input.Snapshots.Get("ingressNginxControllersConfigMap")
	configMapExists := len(configMapSnapshot) > 0

	// Reset metric before calculations
	input.MetricsCollector.Expire(validationSuspendMetricName)

	// Less than 5 controllers → nothing to do
	if len(controllersSnapshot) < 5 {
		return nil
	}

	// First run → set blocking annotations on all controllers
	if !configMapExists {
		setValidationSuspendedAnnotationToAll(controllersSnapshot, input)
		input.MetricsCollector.Set(validationSuspendMetricName, 1.0, nil)
	}

	// If at least one controller has annotation → set metric
	if anyControllerHasAnnotationValidationSuspended(controllersSnapshot) {
		input.MetricsCollector.Set(validationSuspendMetricName, 1.0, nil)
	}

	return nil
}

func setValidationSuspendedAnnotationToAll(controllers []pkg.Snapshot, input *go_hook.HookInput) {
	for _, item := range controllers {
		var ctrl internal.IngressNginxController
		if err := item.UnmarshalTo(&ctrl); err != nil {
			continue
		}

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					internal.IngressNginxControllerSuspendAnnotation: "",
				},
			},
		}
		input.PatchCollector.PatchWithMerge(patch, "deckhouse.io/v1", "IngressNginxController", ctrl.Namespace, ctrl.Name)
	}
}

func anyControllerHasAnnotationValidationSuspended(controllers []pkg.Snapshot) bool {
	for _, item := range controllers {
		var ctrl internal.IngressNginxController
		if err := item.UnmarshalTo(&ctrl); err == nil {
			if _, ok := ctrl.Annotations[internal.IngressNginxControllerSuspendAnnotation]; ok {
				return true
			}
		}
	}
	return false
}
