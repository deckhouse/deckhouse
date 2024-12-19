// Copyright 2021 Flant JSC
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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/301-prometheus-metrics-adapter/hooks/internal"
)

const customMetricAPIVersion = "deckhouse.io/v1beta1"

func MetricKind(t string) string {
	kind := internal.AllMetricsTypes[t]
	return fmt.Sprintf("%sMetric", kind)
}

func ClusterMetricKind(t string) string {
	kind := internal.AllMetricsTypes[t]
	return fmt.Sprintf("Cluster%sMetric", kind)
}

func namespacedMetricConf(metricType string) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       MetricKind(metricType),
		ApiVersion: customMetricAPIVersion,
		Kind:       MetricKind(metricType),
		FilterFunc: applyMetricFilter,
	}
}

func generateKubeHookConfig() []go_hook.KubernetesConfig {
	res := make([]go_hook.KubernetesConfig, 0, len(internal.AllMetricsTypes)*2)
	for metricType := range internal.MetricsTypesForNsAndCluster() {
		nsMetric := namespacedMetricConf(metricType)

		clusterMetric := go_hook.KubernetesConfig{
			Name:       ClusterMetricKind(metricType),
			ApiVersion: customMetricAPIVersion,
			Kind:       ClusterMetricKind(metricType),
			FilterFunc: applyMetricFilter,
		}

		res = append(res, nsMetric, clusterMetric)
	}

	res = append(res, namespacedMetricConf(internal.MetricNamespace))

	return res
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:      "/modules/prometheus_metrics_adapter/custom_metrics",
	Kubernetes: generateKubeHookConfig(),
}, setCustomMetricsQueriesToValues)

func applyMetricFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	metricType, err := internal.ExtractMetricTypeFromKind(obj.GetKind())
	if err != nil {
		return nil, fmt.Errorf("not extract custom metric type: %v - %s/%s", err, obj.GetNamespace(), obj.GetName())
	}

	query, _, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "query")
	if query == "" || err != nil {
		return nil, fmt.Errorf("not found query in custom metric object - %s/%s", obj.GetNamespace(), obj.GetName())
	}

	return internal.CustomMetric{
		Type:      metricType,
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
		Query:     query,
	}, nil
}

func addQueriesToStateFromSnapshots(state *internal.MetricsQueriesState, input *go_hook.HookInput, snapName, metricType string) {
	for _, m := range input.Snapshots[snapName] {
		metric := m.(internal.CustomMetric)

		if _, ok := internal.AllMetricsTypes[metricType]; !ok {
			input.Logger.Warnf("Incorrect custom metric type %s. Skip", metric.Type)
			continue
		}

		state.AddMetric(&metric)
	}
}

// this hook move custom metrics queries from our k8s resources into values.
// from values, we generate config for
// prometheus-adapter and prometheus reverse proxy with helm templates
// see ../templates/config-map.yaml
func setCustomMetricsQueriesToValues(input *go_hook.HookInput) error {
	state := internal.NewMetricsQueryValues()

	for metricType := range internal.AllMetricsTypes {
		addQueriesToStateFromSnapshots(state, input, MetricKind(metricType), metricType)
		// yes, for namespace type we do not have cluster metrics
		// but iteration over snapshot skip it
		addQueriesToStateFromSnapshots(state, input, ClusterMetricKind(metricType), metricType)
	}

	// replace all queries fully
	input.Values.Set(internal.MetricsStatePathToRoot, state.State)

	return nil
}
