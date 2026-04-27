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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	metricsServerEnabledValuesPath = "descheduler.internal.isMetricsServerEnabled"
	kubernetesMetricsAPIGroup      = "metrics.k8s.io"
)

// Register an APIService watch and set descheduler.internal.isMetricsServerEnabled when the
// metrics.k8s.io API group is served (e.g. by metrics-server). The group name is stable; the
// APIService resource name is {version}.{group} (e.g. v1beta1.metrics.k8s.io), so we match on
// spec.group instead of the object name.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/descheduler",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "metrics_kubernetes_io_api",
			ApiVersion: "apiregistration.k8s.io/v1",
			Kind:       "APIService",
			FilterFunc: filterKubernetesMetricsAPIService,
		},
	},
}, discoverKubernetesMetricsAPI)

func filterKubernetesMetricsAPIService(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	group, found, err := unstructured.NestedString(obj.Object, "spec", "group")
	if err != nil || !found {
		return nil, nil
	}
	if group != kubernetesMetricsAPIGroup {
		return nil, nil
	}
	return true, nil
}

func discoverKubernetesMetricsAPI(_ context.Context, input *go_hook.HookInput) error {
	enabled := len(input.Snapshots.Get("metrics_kubernetes_io_api")) > 0
	input.Values.Set(metricsServerEnabledValuesPath, enabled)
	return nil
}
