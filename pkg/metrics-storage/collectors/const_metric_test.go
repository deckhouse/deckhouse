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

package collectors_test

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
)

// Collect must write exactly what prometheus.MustNewConstMetric and prometheus.MustNewConstHistogram
// write for the same series, and panic where they panic.

type constSeries struct {
	group  string
	labels map[string]string
	value  float64
}

var constMetricDescriptions = []collectors.MetricDescription{
	{Name: "no_labels"},
	{Name: "const_labels_only", Help: "help", ConstLabels: map[string]string{"zone": "a", "app": "b"}},
	{Name: "sorted_labels", LabelNames: []string{"a", "b"}},
	{
		Name:        "unsorted_labels",
		Help:        "help",
		LabelNames:  []string{"zeta", "alpha", "mid"},
		ConstLabels: map[string]string{"beta": "1", "omega": "2"},
	},
}

var constMetricSeries = []constSeries{
	{group: "group1", labels: map[string]string{"a": "x", "b": "", "zeta": "z", "alpha": "ünï", "mid": "m"}, value: 1},
	{group: "group1", labels: map[string]string{"a": `quo"te`, "b": "new\nline", "zeta": "", "alpha": `back\slash`, "mid": "日本"}, value: 3},
	{group: "group2", labels: map[string]string{"a": "x", "b": "", "zeta": "z", "alpha": "ünï", "mid": "m"}, value: 42},
	{group: "", labels: map[string]string{"a": "y"}, value: 7},
}

// written renders what a metric writes, so it can be compared with what the prometheus constructors write.
func written(t *testing.T, m prometheus.Metric) string {
	t.Helper()

	out := &dto.Metric{}
	require.NoError(t, m.Write(out))

	return m.Desc().String() + " " + out.String()
}

func collectWritten(t *testing.T, c prometheus.Collector) []string {
	t.Helper()

	ch := make(chan prometheus.Metric, 100)
	c.Collect(ch)
	close(ch)

	var res []string
	for m := range ch {
		res = append(res, written(t, m))
	}
	sort.Strings(res)

	return res
}

func describe(c prometheus.Collector) *prometheus.Desc {
	ch := make(chan *prometheus.Desc, 1)
	c.Describe(ch)

	return <-ch
}

func labelValues(labelNames []string, labels map[string]string) []string {
	values := make([]string, 0, len(labelNames))
	for _, name := range labelNames {
		values = append(values, labels[name])
	}

	return values
}

// expectedConstMetrics sums the series by group and label values and writes them with prometheus.MustNewConstMetric.
func expectedConstMetrics(t *testing.T, c prometheus.Collector, labelNames []string, valueType prometheus.ValueType, series []constSeries, set bool) []string {
	t.Helper()

	type key struct{ group, values string }
	values := map[key]float64{}
	labels := map[key][]string{}

	for _, s := range series {
		lv := labelValues(labelNames, s.labels)
		k := key{group: s.group, values: fmt.Sprintf("%q", lv)}
		labels[k] = lv
		if set {
			values[k] = s.value
			continue
		}
		values[k] += s.value
	}

	desc := describe(c)
	var res []string
	for k, v := range values {
		res = append(res, written(t, prometheus.MustNewConstMetric(desc, valueType, v, labels[k]...)))
	}
	sort.Strings(res)

	return res
}

func TestConstCounterCollector_CollectWritesConstMetrics(t *testing.T) {
	for _, d := range constMetricDescriptions {
		t.Run(d.Name, func(t *testing.T) {
			collector := collectors.NewConstCounterCollector(d)
			for _, s := range constMetricSeries {
				collector.Add(s.value, s.labels, collectors.WithGroup(s.group))
			}

			expected := expectedConstMetrics(t, collector, collector.LabelNames(), prometheus.CounterValue, constMetricSeries, false)
			assert.Equal(t, expected, collectWritten(t, collector))

			// A new label rebuilds the desc and the label order.
			added := constSeries{group: "group3", labels: map[string]string{"a": "x", "new": "n"}, value: 5}
			collector.UpdateLabels([]string{"new"})
			collector.Add(added.value, added.labels, collectors.WithGroup(added.group))

			updated := append(append([]constSeries{}, constMetricSeries...), added)
			expected = expectedConstMetrics(t, collector, collector.LabelNames(), prometheus.CounterValue, updated, false)
			assert.Equal(t, expected, collectWritten(t, collector))
		})
	}
}

func TestConstGaugeCollector_CollectWritesConstMetrics(t *testing.T) {
	series := append(append([]constSeries{}, constMetricSeries...),
		constSeries{group: "group3", labels: map[string]string{"a": "nan"}, value: math.NaN()},
		constSeries{group: "group3", labels: map[string]string{"a": "inf"}, value: math.Inf(-1)},
		constSeries{group: "group3", labels: map[string]string{"a": "negative"}, value: -2.5},
	)

	for _, d := range constMetricDescriptions {
		t.Run(d.Name, func(t *testing.T) {
			desc := d
			collector := collectors.NewConstGaugeCollector(&desc)
			for _, s := range series {
				collector.Set(s.value, s.labels, collectors.WithGroup(s.group))
			}

			expected := expectedConstMetrics(t, collector, collector.LabelNames(), prometheus.GaugeValue, series, true)
			assert.Equal(t, expected, collectWritten(t, collector))

			// A new label rebuilds the desc and the label order.
			added := constSeries{group: "group4", labels: map[string]string{"a": "x", "new": "n"}, value: 5}
			collector.UpdateLabels([]string{"new"})
			collector.Set(added.value, added.labels, collectors.WithGroup(added.group))

			updated := append(append([]constSeries{}, series...), added)
			expected = expectedConstMetrics(t, collector, collector.LabelNames(), prometheus.GaugeValue, updated, true)
			assert.Equal(t, expected, collectWritten(t, collector))
		})
	}
}

func TestConstHistogramCollector_CollectWritesConstHistograms(t *testing.T) {
	bucketSets := [][]float64{
		{0.1, 0.5, 1},
		{5, 1, 0.5},
		{1, 1, 2, 2, 2, 3},
		{-1, 0, 1},
		{0.5, math.Inf(1)},
		{math.Inf(-1), 0},
	}
	observations := []float64{-2, 0, 0.3, 1, 1.5, 2, 4, math.Inf(1)}

	for _, d := range constMetricDescriptions {
		for _, buckets := range bucketSets {
			t.Run(fmt.Sprintf("%s/%v", d.Name, buckets), func(t *testing.T) {
				desc := d
				collector := collectors.NewConstHistogramCollector(&desc, buckets)

				type key struct{ group, values string }
				observed := map[key][]float64{}
				labels := map[key][]string{}
				for i, s := range constMetricSeries {
					lv := labelValues(collector.LabelNames(), s.labels)
					k := key{group: s.group, values: fmt.Sprintf("%q", lv)}
					labels[k] = lv
					for _, v := range observations[i:] {
						collector.Observe(v, s.labels, collectors.WithGroup(s.group))
						observed[k] = append(observed[k], v)
					}
				}

				var expected []string
				for k, values := range observed {
					var sum float64
					counts := map[float64]uint64{}
					for _, upperBound := range collector.Buckets() {
						counts[upperBound] = 0
					}
					for _, v := range values {
						sum += v
						// Negative observations are not counted in buckets.
						if v < 0 {
							continue
						}
						for upperBound := range counts {
							if v <= upperBound {
								counts[upperBound]++
							}
						}
					}
					h := prometheus.MustNewConstHistogram(describe(collector), uint64(len(values)), sum, counts, labels[k]...)
					expected = append(expected, written(t, h))
				}
				sort.Strings(expected)

				assert.Equal(t, expected, collectWritten(t, collector))
			})
		}
	}
}

// panicValue returns what f panics with, or nil.
func panicValue(f func()) (res any) {
	defer func() {
		res = recover()
	}()
	f()

	return nil
}

func TestCollect_PanicsLikeConstMetrics(t *testing.T) {
	collect := func(c prometheus.Collector) func() {
		return func() {
			ch := make(chan prometheus.Metric, 10)
			c.Collect(ch)
		}
	}

	t.Run("invalid utf-8 label value", func(t *testing.T) {
		counter := collectors.NewConstCounterCollector(collectors.MetricDescription{Name: "counter", LabelNames: []string{"a"}})
		counter.Add(1, map[string]string{"a": "\xff"})
		expected := panicValue(func() { prometheus.MustNewConstMetric(describe(counter), prometheus.CounterValue, 1, "\xff") })
		require.NotNil(t, expected)
		assert.Equal(t, expected, panicValue(collect(counter)))

		gauge := collectors.NewConstGaugeCollector(&collectors.MetricDescription{Name: "gauge", LabelNames: []string{"a"}})
		gauge.Set(1, map[string]string{"a": "\xff"})
		assert.Equal(t, expected, panicValue(collect(gauge)))

		histogram := collectors.NewConstHistogramCollector(&collectors.MetricDescription{Name: "histogram", LabelNames: []string{"a"}}, []float64{1})
		histogram.Observe(1, map[string]string{"a": "\xff"})
		assert.Equal(t, expected, panicValue(collect(histogram)))
	})

	t.Run("invalid label name added to the desc", func(t *testing.T) {
		gauge := collectors.NewConstGaugeCollector(&collectors.MetricDescription{Name: "gauge", LabelNames: []string{"a"}})
		gauge.Set(1, map[string]string{"a": "x"})
		gauge.UpdateLabels([]string{"__reserved"})

		expected := panicValue(func() {
			prometheus.MustNewConstMetric(prometheus.NewDesc("gauge", "gauge", []string{"__reserved", "a"}, nil), prometheus.GaugeValue, 1, "", "x")
		})
		require.NotNil(t, expected)
		assert.Equal(t, expected, panicValue(collect(gauge)))
	})
}

func TestCollect_ConcurrentWithUpdates(t *testing.T) {
	counter := collectors.NewConstCounterCollector(collectors.MetricDescription{Name: "counter", LabelNames: []string{"a"}})
	gauge := collectors.NewConstGaugeCollector(&collectors.MetricDescription{Name: "gauge", LabelNames: []string{"a"}})
	histogram := collectors.NewConstHistogramCollector(&collectors.MetricDescription{Name: "histogram", LabelNames: []string{"a"}}, []float64{1, 2})

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				labels := map[string]string{"a": fmt.Sprintf("%d", j%10), fmt.Sprintf("l%d", i): "v"}
				counter.UpdateLabels([]string{fmt.Sprintf("l%d", i)})
				gauge.UpdateLabels([]string{fmt.Sprintf("l%d", i)})
				histogram.UpdateLabels([]string{fmt.Sprintf("l%d", i)})
				counter.Add(1, labels, collectors.WithGroup("group"))
				gauge.Set(float64(j), labels, collectors.WithGroup("group"))
				histogram.Observe(float64(j%3), labels, collectors.WithGroup("group"))
			}
		}(i)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				for _, c := range []prometheus.Collector{counter, gauge, histogram} {
					ch := make(chan prometheus.Metric, 1000)
					c.Collect(ch)
					close(ch)
					for m := range ch {
						assert.NoError(t, m.Write(&dto.Metric{}))
					}
				}
			}
		}()
	}
	wg.Wait()

	assert.Len(t, counter.LabelNames(), 5)
}
