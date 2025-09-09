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
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

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

func addQueriesToStateFromSnapshots(state *internal.MetricsQueriesState, input *go_hook.HookInput, snapName, metricType string) error {
	for metric, err := range sdkobjectpatch.SnapshotIter[internal.CustomMetric](input.Snapshots.Get(snapName)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'CustomMetric' snapshots: %w", err)
		}

		if _, ok := internal.AllMetricsTypes[metricType]; !ok {
			input.Logger.Warn("Incorrect custom metric type. Skip", slog.String("type", metric.Type))
			continue
		}

		state.AddMetric(&metric)
	}

	return nil
}

// this hook move custom metrics queries from our k8s resources into values.
// from values, we generate config for
// prometheus-adapter and prometheus reverse proxy with helm templates
// see ../templates/config-map.yaml
func setCustomMetricsQueriesToValues(_ context.Context, input *go_hook.HookInput) error {
	state := internal.NewMetricsQueryValues()

	for metricType := range internal.AllMetricsTypes {
		if err := addQueriesToStateFromSnapshots(state, input, MetricKind(metricType), metricType); err != nil {
			return fmt.Errorf("failed to add queries to state from snapshots: %w", err)
		}
		// yes, for namespace type we do not have cluster metrics
		// but iteration over snapshot skip it
		if err := addQueriesToStateFromSnapshots(state, input, ClusterMetricKind(metricType), metricType); err != nil {
			return fmt.Errorf("failed to add queries to state from snapshots: %w", err)
		}
	}

	// replace all queries fully
	input.Values.Set(internal.MetricsStatePathToRoot, state.State)

	return nil
}
