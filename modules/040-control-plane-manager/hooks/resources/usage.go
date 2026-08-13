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

// Measured usage of the control-plane components, read through
// custom.metrics.k8s.io (prometheus-metrics-adapter). This is the input the
// dynamic resolver decides on; what it decides lives in resolve_dynamic.go.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	autotuneMinMilliCPU    = int64(10)
	autotuneMinMemoryBytes = int64(15 * 1000 * 1000) // 15M
)

// prometheusMetricsAvailable is true when prometheus and prometheus-metrics-adapter
// are enabled — the custom.metrics.k8s.io path the dynamic resolver needs.
func prometheusMetricsAvailable(input *go_hook.HookInput) bool {
	enabled := set.NewFromValues(input.Values, "global.enabledModules")
	return enabled.Has("prometheus") && enabled.Has("prometheus-metrics-adapter")
}

// readComponentUsage measures every control-plane component. The second result
// is false when at least one component could not be read at all — which is not
// the same as a component measured at zero.
func readComponentUsage(ctx context.Context, rctx resolveContext, kind resourceKind) (usageByComponent, bool) {
	usage := make(usageByComponent, len(controlPlaneComponents))
	noFetchErrors := true

	for _, comp := range controlPlaneComponents {
		measured, ok, err := fetchComponentUsage(ctx, rctx.dc, comp, kind)
		if err != nil {
			noFetchErrors = false
			rctx.input.Logger.Warn("autotune: metrics API fetch failed", "resource", kind, "component", comp, "error", err)
			continue
		}
		if ok {
			usage[comp] = clampUsage(measured, kind)
		}
	}

	return usage, noFetchErrors
}

// clampUsage only normalizes units and enforces the sane-minimum floor. It must
// NOT also cap to headroom: decide() needs the true measured value to compare
// against the baseline — capping it here would make shrinking headroom (another
// pod landing on the master) look like usage actually dropped, triggering a
// spurious decideLower. The headroom ceiling belongs solely to the raise-side
// gate in dynamicResolver.gateRaises, which enforces it by summing the proposed
// requests.
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
	})
	if err != nil {
		return 0, false, fmt.Errorf("list control-plane pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return 0, false, fmt.Errorf("no pods matching component=%s,tier=control-plane", container)
	}

	// The loudest replica wins: requests are rendered identically for every
	// master, so they have to cover the busiest one.
	var maxVal float64
	found := false
	var lastErr error
	for i := range pods.Items {
		v, ok, ferr := fetchPodMetric(ctx, client, pods.Items[i].Name, metric)
		if ferr != nil {
			lastErr = ferr
			continue
		}
		if !ok {
			continue
		}
		if !found || v > maxVal {
			maxVal = v
			found = true
		}
	}
	if !found && lastErr != nil {
		return 0, false, lastErr
	}
	return maxVal, found, nil
}

func fetchPodMetric(ctx context.Context, client k8s.Client, podName, metric string) (float64, bool, error) {
	path := fmt.Sprintf(
		"/apis/custom.metrics.k8s.io/v1beta1/namespaces/%s/pods/%s/%s",
		kubeSystemNS, podName, metric,
	)
	body, err := client.CoreV1().RESTClient().Get().AbsPath(path).Do(ctx).Raw()
	if err != nil {
		return 0, false, fmt.Errorf("GET %s: %w", path, err)
	}

	var list customMetricValueList
	if err := json.Unmarshal(body, &list); err != nil {
		return 0, false, fmt.Errorf("decode metrics response for %s: %w; body=%.256s", podName, err, body)
	}
	if len(list.Items) == 0 {
		return 0, false, fmt.Errorf("GET %s: empty MetricValueList", path)
	}

	v := list.Items[0].Value.AsApproximateFloat64()
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0, false, fmt.Errorf("GET %s: non-positive metric value %q", path, list.Items[0].Value.String())
	}
	return v, true, nil
}
