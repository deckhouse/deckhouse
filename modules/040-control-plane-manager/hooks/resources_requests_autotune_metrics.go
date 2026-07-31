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

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

const (
	autotuneMinMilliCPU = int64(10)
	autotuneMinMemory   = int64(15 * 1024 * 1024) // 15 MiB
)

// componentUsageFunc reads lookback-average usage for a control-plane component.
// Returns (value, ok, err); ok=false means no usable datapoint (cold start / missing series).
type componentUsageFunc func(ctx context.Context, dc dependency.Container, component string, resourceName resourceKind) (float64, bool, error)

// fetchComponentUsage is the production metrics client; overridable in unit tests.
var fetchComponentUsage componentUsageFunc = fetchComponentUsageFromMetricsAPI

func clampRecommendation(raw float64, resourceName resourceKind, nodeBudget int64) int64 {
	var v int64
	switch resourceName {
	case resourceCPU:
		// PromQL returns cores; convert to millicpu.
		v = int64(math.Ceil(raw * 1000))
		if v < autotuneMinMilliCPU {
			v = autotuneMinMilliCPU
		}
	case resourceMemory:
		v = int64(math.Ceil(raw))
		if v < autotuneMinMemory {
			v = autotuneMinMemory
		}
	}
	if nodeBudget > 0 && v > nodeBudget {
		v = nodeBudget
	}
	return v
}

// fetchRecs collects clamped recommendations for one measurement.
// On per-component fetch errors it sets usageOK=false and continues; missing
// series (ok=false, err=nil) simply omit that component from the map.
func fetchRecs(
	ctx context.Context,
	dc dependency.Container,
	fetch componentUsageFunc,
	resourceName resourceKind,
	overridden bool,
	nodeBudget int64,
	onErr func(component string, err error),
) (map[string]int64, bool) {
	recs := make(map[string]int64, len(controlPlaneComponents))
	usageOK := true
	if overridden {
		return recs, true
	}
	for _, comp := range controlPlaneComponents {
		v, ok, ferr := fetch(ctx, dc, comp, resourceName)
		if ferr != nil {
			usageOK = false
			if onErr != nil {
				onErr(comp, ferr)
			}
			continue
		}
		if ok {
			recs[comp] = clampRecommendation(v, resourceName, nodeBudget)
		}
	}
	return recs, usageOK
}

func completeComponentRecs(recs map[string]int64) bool {
	if len(recs) < len(controlPlaneComponents) {
		return false
	}
	for _, comp := range controlPlaneComponents {
		if _, ok := recs[comp]; !ok {
			return false
		}
	}
	return true
}

// customMetricValueList is the subset of custom.metrics.k8s.io MetricValueList we need.
type customMetricValueList struct {
	Items []struct {
		Value resource.Quantity `json:"value"`
	} `json:"items"`
}

func fetchComponentUsageFromMetricsAPI(ctx context.Context, dc dependency.Container, component string, resourceName resourceKind) (float64, bool, error) {
	client, err := dc.GetK8sClient()
	if err != nil {
		return 0, false, fmt.Errorf("get k8s client: %w", err)
	}

	container := componentContainer[component]
	metric := autotuneMetricNameFor(resourceName)

	pods, err := client.CoreV1().Pods(kubeSystemNS).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("component=%s,tier=control-plane", container),
	})
	if err != nil {
		return 0, false, fmt.Errorf("list control-plane pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return 0, false, fmt.Errorf("no pods matching component=%s,tier=control-plane", container)
	}

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
		snippet := string(body)
		if len(snippet) > 256 {
			snippet = snippet[:256] + "…"
		}
		return 0, false, fmt.Errorf("decode metrics response for %s: %w; body=%s", podName, err, snippet)
	}
	if len(list.Items) == 0 {
		// Empty list is a real miss (PromQL returned no series). Surface it so
		// schedule logs show why memory/cpu was skipped instead of failing silently.
		return 0, false, fmt.Errorf("GET %s: empty MetricValueList", path)
	}

	// custom.metrics encodes samples as milli-quantities; AsApproximateFloat64
	// yields the natural unit (cores for cpu, bytes for memory).
	v := list.Items[0].Value.AsApproximateFloat64()
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0, false, fmt.Errorf("GET %s: non-positive metric value %q", path, list.Items[0].Value.String())
	}
	return v, true, nil
}
