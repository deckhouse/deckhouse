/*
Copyright 2026 Flant JSC

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

package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	autotuneMinMilliCPU    = int64(10)
	autotuneMinMemoryBytes = int64(15 * 1000 * 1000) // 15M
	maxSaneUsage           = float64(1 << 50)
)

func prometheusMetricsAvailable(input *go_hook.HookInput) bool {
	enabled := set.NewFromValues(input.Values, "global.enabledModules")
	return enabled.Has("prometheus") && enabled.Has("prometheus-metrics-adapter")
}

// A component missing from the result was not measured, which is not the same as
// measured at zero.
func readComponentUsage(ctx context.Context, deps resolveDeps, kind resourceKind) (usageByComponent, bool) {
	usage := make(usageByComponent, len(controlPlaneComponents))
	someFetchFailed := false

	for _, comp := range controlPlaneComponents {
		measured, ok, err := fetchComponentUsage(ctx, deps.dc, comp, kind)
		if err != nil {
			someFetchFailed = true
			deps.input.Logger.Warn("autotune: metrics API fetch failed", "resource", kind, "component", comp, "error", err)
			continue
		}
		if !ok {
			continue
		}
		if measured > maxSaneUsage {
			// Dropped rather than capped, which would look like real usage to decide().
			deps.input.Logger.Warn("autotune: implausible usage from the metrics API, ignoring the datapoint",
				"resource", kind, "component", comp, "value", measured)
			continue
		}
		usage[comp] = clampUsage(measured, kind)
	}

	return usage, someFetchFailed
}

// Must not also cap to headroom: decide() compares the true measured value
// against the baseline, so a capped one would make shrinking headroom — another
// pod landing on the master — look like usage itself had dropped.
func clampUsage(raw float64, kind resourceKind) int64 {
	var v int64
	switch kind {
	case resourceCPU:
		v = int64(math.Ceil(raw * 1000))
		if v < autotuneMinMilliCPU {
			v = autotuneMinMilliCPU
		}
	case resourceMemory:
		v = int64(math.Round(raw))
		if v < autotuneMinMemoryBytes {
			v = autotuneMinMemoryBytes
		}
	}
	return v
}

// Overridable in unit tests.
var fetchComponentUsage = fetchComponentUsageFromMetricsAPI

type customMetricValueList struct {
	Items []struct {
		Value resource.Quantity `json:"value"`
	} `json:"items"`
}

func fetchComponentUsageFromMetricsAPI(ctx context.Context, dc dependency.Container, component string, kind resourceKind) (float64, bool, error) {
	client, err := dc.GetK8sClient()
	if err != nil {
		return 0, false, fmt.Errorf("get k8s client: %w", err)
	}

	container := componentMeta[component].container
	metric := "d8-cpm-autotune-" + string(kind)

	pods, err := client.CoreV1().Pods(kubeSystemNS).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("component=%s,tier=control-plane", container),
		// Or a pod left behind by a decommissioned master keeps contributing its value.
		FieldSelector: notTerminatedPods.String(),
	})
	if err != nil {
		return 0, false, fmt.Errorf("list control-plane pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return 0, false, fmt.Errorf("no pods matching component=%s,tier=control-plane", container)
	}

	var loudest float64
	measured := false
	var lastErr error
	for i := range pods.Items {
		value, ok, err := fetchPodMetric(ctx, client.CoreV1().RESTClient(), pods.Items[i].Name, metric)
		switch {
		case err != nil:
			lastErr = err
		case ok && (!measured || value > loudest):
			loudest = value
			measured = true
		}
	}
	if !measured && lastErr != nil {
		return 0, false, lastErr
	}
	return loudest, measured, nil
}

// Three states, hence both a bool and an error: a value, a miss, or a failure. A
// miss is routine for a pod younger than the adapter's relist interval.
func fetchPodMetric(ctx context.Context, rc rest.Interface, podName, metric string) (float64, bool, error) {
	path := fmt.Sprintf(
		"/apis/custom.metrics.k8s.io/v1beta1/namespaces/%s/pods/%s/%s",
		kubeSystemNS, podName, metric,
	)
	body, err := rc.Get().AbsPath(path).Do(ctx).Raw()
	if err != nil {
		if apierrors.IsNotFound(err) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("GET %s: %w", path, err)
	}

	var list customMetricValueList
	if err := json.Unmarshal(body, &list); err != nil {
		return 0, false, fmt.Errorf("decode metrics response for %s: %w; body=%.256s", podName, err, body)
	}
	if len(list.Items) == 0 {
		return 0, false, nil
	}

	v := list.Items[0].Value.AsApproximateFloat64()
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0, false, fmt.Errorf("GET %s: non-positive metric value %q", path, list.Items[0].Value.String())
	}
	return v, true, nil
}
