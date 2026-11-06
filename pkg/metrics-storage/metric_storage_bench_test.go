// Copyright 2026 Flant JSC
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

package metricsstorage_test

import (
	"fmt"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
)

// The benchmarks below call the storage the way shell-operator, addon-operator and
// deckhouse-controller do.

// hookRunBuckets are the buckets of the hook run histograms.
var hookRunBuckets = []float64{0.0, 0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10}

func newBenchmarkStorage() *metricsstorage.MetricStorage {
	return metricsstorage.NewMetricStorage(
		metricsstorage.WithNewRegistry(),
		metricsstorage.WithLogger(log.NewNop()),
	)
}

// hookRunLabels are the labels of a hook run metric.
func hookRunLabels(hook int) map[string]string {
	return map[string]string{
		"hook":       fmt.Sprintf("hook-%d", hook),
		"binding":    "kubernetes",
		"queue":      "main",
		"module":     "node-manager",
		"activation": "",
	}
}

// BenchmarkMetricStorage_Gather is a scrape of hook run histograms, gauges and counters.
func BenchmarkMetricStorage_Gather(b *testing.B) {
	for _, series := range []int{100, 1000} {
		b.Run(fmt.Sprintf("series=%d", series), func(b *testing.B) {
			storage := newBenchmarkStorage()

			for hook := 0; hook < series; hook++ {
				labels := hookRunLabels(hook)
				storage.HistogramObserve("hook_run_seconds", 0.3, labels, hookRunBuckets)
				storage.GaugeSet("hook_run_max_rss_bytes", float64(hook), labels)
				storage.CounterAdd("hook_run_success_total", 1, labels)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := storage.Gather(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkMetricStorage_HistogramObserveRegistered is a kube event: the histogram is registered
// at start and observed with the monitor labels and nil buckets.
func BenchmarkMetricStorage_HistogramObserveRegistered(b *testing.B) {
	storage := newBenchmarkStorage()

	labelNames := []string{"hook", "binding", "module", "namespace", "queue", "kind"}
	if _, err := storage.RegisterHistogram("kube_event_duration_seconds", labelNames, hookRunBuckets); err != nil {
		b.Fatal(err)
	}

	labels := make([]map[string]string, 100)
	for i := range labels {
		labels[i] = map[string]string{
			"hook":      fmt.Sprintf("hook-%d", i),
			"binding":   "pods",
			"module":    "node-manager",
			"namespace": "",
			"queue":     "main",
			"kind":      "Pod",
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.HistogramObserve("kube_event_duration_seconds", 0.003, labels[i%len(labels)], nil)
	}
}

// hookRunOperations are the operations of a go hook run: the hook expires its group and sets its gauges again.
func hookRunOperations(hook, series int) []operation.MetricOperation {
	group := fmt.Sprintf("hook-%d", hook)
	ops := []operation.MetricOperation{{Group: group, Action: operation.ActionExpireMetrics}}

	for s := 0; s < series; s++ {
		ops = append(ops, operation.MetricOperation{
			Name:   fmt.Sprintf("d8_hook_%d_metric_%d", hook, s%2),
			Group:  group,
			Action: operation.ActionGaugeSet,
			Value:  floatPtr(float64(s)),
			Labels: map[string]string{"name": fmt.Sprintf("object-%d", s), "namespace": "d8-system"},
		})
	}

	return ops
}

// BenchmarkMetricStorage_ApplyBatchOperations_HookRun is a go hook run applied to the hook metric
// storage that keeps the series of the other hooks.
func BenchmarkMetricStorage_ApplyBatchOperations_HookRun(b *testing.B) {
	for _, size := range []struct{ hooks, series int }{{50, 20}, {200, 50}} {
		b.Run(fmt.Sprintf("hooks=%d/series=%d", size.hooks, size.series), func(b *testing.B) {
			storage := newBenchmarkStorage()

			runs := make([][]operation.MetricOperation, size.hooks)
			labels := make([]map[string]string, size.hooks)
			for hook := range runs {
				runs[hook] = hookRunOperations(hook, size.series)
				labels[hook] = map[string]string{"hook": fmt.Sprintf("hook-%d", hook), "module": "node-manager"}
				if err := storage.ApplyBatchOperations(runs[hook], labels[hook]); err != nil {
					b.Fatal(err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hook := i % size.hooks
				if err := storage.ApplyBatchOperations(runs[hook], labels[hook]); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
