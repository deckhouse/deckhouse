// Copyright 2025 Flant JSC
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

package storage

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	promtest "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

func Test_CounterAdd(t *testing.T) {
	g := NewWithT(t)

	logger := log.NewLogger()
	log.SetDefault(logger)

	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	v := NewGroupedVault()

	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())

	expect := `
# HELP metric_total metric_total
# TYPE metric_total counter
metric_total{lbl="val"} 1
`
	err := promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.ExpireGroupMetrics("group1")

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())

	// Expect no metric with lbl="val"
	expect = ``
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val2"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())

	// Expect metric_total with new label value
	expect = `
# HELP metric_total metric_total
# TYPE metric_total counter
metric_total{lbl="val2"} 1
`
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val2"})
	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val222"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())

	// Expect metric_total with 2 label values
	expect = `
# HELP metric_total metric_total
# TYPE metric_total counter
metric_total{lbl="val2"} 2
metric_total{lbl="val222"} 1
`
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.ExpireGroupMetrics("group1")
	v.CounterAdd("group1", "metric_total", 1.0, map[string]string{"lbl": "val"})
	v.CounterAdd("group2", "metric2_total", 1.0, map[string]string{"lbl": "val222"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())
	// Expect metric_total is updated and metric2_total
	expect = `
# HELP metric_total metric_total
# TYPE metric_total counter
metric_total{lbl="val"} 1
# HELP metric2_total metric2_total
# TYPE metric2_total counter
metric2_total{lbl="val222"} 1
`
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total", "metric2_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.ExpireGroupMetrics("group1")
	v.CounterAdd("group2", "metric2_total", 1.0, map[string]string{"lbl": "val222"})
	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())
	// Expect metric_total is updated and metric2_total is updated and metric_total left as is
	expect = `
# HELP metric2_total metric2_total
# TYPE metric2_total counter
metric2_total{lbl="val222"} 2
`
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total", "metric2_total")
	g.Expect(err).ShouldNot(HaveOccurred())

	v.ExpireGroupMetrics("group1")
	v.ExpireGroupMetrics("group2")

	// Expect all metric instances sharing the same name to share equal labelsets respectively

	v.GaugeSet("group1", "metric_total1", 1.0, map[string]string{"a": "A"})
	v.GaugeSet("group1", "metric_total1", 2.0, map[string]string{"c": "C"})
	v.GaugeSet("group1", "metric_total1", 3.0, map[string]string{"a": "A", "b": "B"})
	v.GaugeSet("group1", "metric_total1", 5.0, map[string]string{"a": "A"})
	v.GaugeSet("group1", "metric_total2", 1.0, map[string]string{"a": "A1"})
	v.GaugeSet("group1", "metric_total2", 2.0, map[string]string{"c": "C2"})
	v.GaugeSet("group1", "metric_total2", 3.0, map[string]string{"a": "A3", "b": "B3"})

	v.CounterAdd("group2", "metric_total3", 1.0, map[string]string{"lbl": "val222"})
	v.CounterAdd("group2", "metric_total3", 1.0, map[string]string{"ord": "ord222"})
	v.CounterAdd("group2", "metric_total3", 4.0, map[string]string{"lbl": "val222"})
	v.CounterAdd("group2", "metric_total3", 9.0, map[string]string{"ord": "ord222"})
	v.CounterAdd("group2", "metric_total3", 99.0, map[string]string{"lbl": "val222", "ord": "ord222"})
	v.CounterAdd("group2", "metric_total3", 9.0, map[string]string{"lbl": "val222", "ord": "ord222"})

	v.CounterAdd("group3", "metric_total4", 9.0, map[string]string{"d": "d1"})
	v.CounterAdd("group3", "metric_total4", 99.0, map[string]string{"a": "a1", "b": "b1", "c": "c1", "d": "d1"})
	v.CounterAdd("group3", "metric_total4", 19.0, map[string]string{"c": "c2"})
	v.CounterAdd("group3", "metric_total4", 29.0, map[string]string{})
	v.CounterAdd("group3", "metric_total4", 39.0, map[string]string{"j": "j1"})
	v.CounterAdd("group3", "metric_total4", 1.0, map[string]string{"j": "j1"})
	v.CounterAdd("group3", "metric_total4", 1.0, map[string]string{"a": "", "b": "", "c": "", "d": "", "j": "j1"})
	v.CounterAdd("group3", "metric_total5", 7.0, map[string]string{"g": "g1"})
	v.CounterAdd("group3", "metric_total5", 11.0, map[string]string{"foo": "bar"})

	g.Expect(buf.String()).ShouldNot(ContainSubstring("error"), "error occurred in log: %s", buf.String())

	expect = `
# HELP metric_total1 metric_total1
# TYPE metric_total1 gauge
metric_total1{a="A", b="", c=""} 5
metric_total1{a="", b="", c="C"} 2
metric_total1{a="A", b="B", c=""} 3
# HELP metric_total2 metric_total2
# TYPE metric_total2 gauge
metric_total2{a="A1", b="", c=""} 1
metric_total2{a="", b="", c="C2"} 2
metric_total2{a="A3", b="B3", c=""} 3
# HELP metric_total3 metric_total3
# TYPE metric_total3 counter
metric_total3{lbl="val222", ord=""} 5
metric_total3{lbl="", ord="ord222"} 10
metric_total3{lbl="val222", ord="ord222"} 108
# HELP metric_total4 metric_total4
# TYPE metric_total4 counter
metric_total4{a="", b="", c="", d="d1", j=""} 9
metric_total4{a="a1", b="b1", c="c1", d="d1", j=""} 99
metric_total4{a="", b="", c="c2", d="", j=""} 19
metric_total4{a="", b="", c="", d="", j=""} 29
metric_total4{a="", b="", c="", d="", j="j1"} 41
# HELP metric_total5 metric_total5
# TYPE metric_total5 counter
metric_total5{foo="", g="g1"} 7
metric_total5{foo="bar", g=""} 11
`

	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total1", "metric_total2", "metric_total3", "metric_total4", "metric_total5")
	g.Expect(err).ShouldNot(HaveOccurred())

	expect = `
# HELP metric_total1 metric_total1
# TYPE metric_total1 gauge
metric_total1{a="A", b="", c=""} 5
metric_total1{a="", b="", c="C"} 2
metric_total1{a="A", b="B", c=""} 3
# HELP metric_total2 metric_total2
# TYPE metric_total2 gauge
metric_total2{a="A1", b="", c=""} 1
metric_total2{a="", b="", c="C2"} 2
metric_total2{a="A3", b="B3", c=""} 3
# HELP metric_total3 metric_total3
# TYPE metric_total3 counter
metric_total3{lbl="val222", ord=""} 5
metric_total3{lbl="", ord="ord222"} 10
metric_total3{lbl="val222", ord="ord222"} 108
# HELP metric_total4 metric_total4
# TYPE metric_total4 counter
metric_total4{a="", b="", c="", d="d1", j=""} 9
metric_total4{a="a1", b="b1", c="c1", d="d1", j=""} 99
metric_total4{a="", b="", c="c2", d="", j=""} 19
metric_total4{a="", b="", c="", d="", j=""} 29
metric_total4{a="", b="", c="", d="", j="j1"} 41
`

	v.ExpireGroupMetricByName("group3", "metric_total5")
	err = promtest.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expect), "metric_total1", "metric_total2", "metric_total3", "metric_total4", "metric_total5")
	g.Expect(err).ShouldNot(HaveOccurred())
}

func newIsolatedVault() (*GroupedVault, *prometheus.Registry) {
	reg := prometheus.NewRegistry()
	v := NewGroupedVault(
		options.WithRegistry(reg),
		options.WithLogger(log.NewNop()),
	)
	return v, reg
}

func TestGroupedVault_NewGroupedVault(t *testing.T) {
	t.Run("with logger option", func(t *testing.T) {
		logger := log.NewNop()
		v := NewGroupedVault(options.WithLogger(logger))
		assert.NotNil(t, v)
		assert.NotNil(t, v.Registerer())
	})

	t.Run("with registry option", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		v := NewGroupedVault(options.WithRegistry(reg))
		assert.NotNil(t, v)
		assert.Equal(t, reg, v.Registerer())
	})

	t.Run("with new registry option", func(t *testing.T) {
		v := NewGroupedVault(options.WithNewRegistry())
		assert.NotNil(t, v)
		assert.NotNil(t, v.Registerer())
	})

	t.Run("default registerer when no options", func(t *testing.T) {
		v := NewGroupedVault()
		assert.NotNil(t, v)
		assert.Equal(t, prometheus.DefaultRegisterer, v.Registerer())
	})
}

func TestGroupedVault_Collector(t *testing.T) {
	t.Run("returns registry when set", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		v := NewGroupedVault(options.WithRegistry(reg))
		assert.Equal(t, reg, v.Collector())
	})

	t.Run("returns self when no registry", func(t *testing.T) {
		v := NewGroupedVault(options.WithLogger(log.NewNop()))
		collector := v.Collector()
		assert.Equal(t, v, collector)
	})
}

func TestGroupedVault_RegisterGauge(t *testing.T) {
	t.Run("basic registration", func(t *testing.T) {
		v, reg := newIsolatedVault()
		gauge, err := v.RegisterGauge("test_gauge", []string{"env"})
		require.NoError(t, err)
		require.NotNil(t, gauge)

		gauge.Set(42.0, map[string]string{"env": "prod"})

		expect := `
# HELP test_gauge test_gauge
# TYPE test_gauge gauge
test_gauge{env="prod"} 42
`
		err = promtest.GatherAndCompare(reg, strings.NewReader(expect), "test_gauge")
		assert.NoError(t, err)
	})

	t.Run("re-registration returns same collector", func(t *testing.T) {
		v, _ := newIsolatedVault()
		g1, err := v.RegisterGauge("test_gauge", []string{"env"})
		require.NoError(t, err)

		g2, err := v.RegisterGauge("test_gauge", []string{"env"})
		require.NoError(t, err)
		assert.Equal(t, g1, g2)
	})

	t.Run("type mismatch returns error", func(t *testing.T) {
		v, _ := newIsolatedVault()
		_, err := v.RegisterCounter("metric_name", []string{"label"})
		require.NoError(t, err)

		_, err = v.RegisterGauge("metric_name", []string{"label"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "counter")
	})

	t.Run("with help and constant labels", func(t *testing.T) {
		v, _ := newIsolatedVault()
		gauge, err := v.RegisterGauge("test_gauge", []string{"env"},
			options.WithHelp("custom help"),
			options.WithConstantLabels(map[string]string{"service": "api"}),
		)
		require.NoError(t, err)
		assert.NotNil(t, gauge)
	})

	t.Run("label expansion on re-registration", func(t *testing.T) {
		v, reg := newIsolatedVault()
		_, err := v.RegisterGauge("test_gauge", []string{"a"})
		require.NoError(t, err)

		_, err = v.RegisterGauge("test_gauge", []string{"a", "b"})
		require.NoError(t, err)

		v.GaugeSet("g1", "test_gauge", 5.0, map[string]string{"a": "A", "b": "B"})

		expect := `
# HELP test_gauge test_gauge
# TYPE test_gauge gauge
test_gauge{a="A",b="B"} 5
`
		err = promtest.GatherAndCompare(reg, strings.NewReader(expect), "test_gauge")
		assert.NoError(t, err)
	})
}

func TestGroupedVault_RegisterHistogram(t *testing.T) {
	t.Run("basic registration", func(t *testing.T) {
		v, reg := newIsolatedVault()
		hist, err := v.RegisterHistogram("test_hist", []string{"method"}, []float64{0.1, 1.0, 10.0})
		require.NoError(t, err)
		require.NotNil(t, hist)

		hist.Observe(0.5, map[string]string{"method": "GET"})

		expect := `
# HELP test_hist test_hist
# TYPE test_hist histogram
test_hist_bucket{method="GET",le="0.1"} 0
test_hist_bucket{method="GET",le="1"} 1
test_hist_bucket{method="GET",le="10"} 1
test_hist_bucket{method="GET",le="+Inf"} 1
test_hist_sum{method="GET"} 0.5
test_hist_count{method="GET"} 1
`
		err = promtest.GatherAndCompare(reg, strings.NewReader(expect), "test_hist")
		assert.NoError(t, err)
	})

	t.Run("type mismatch returns error", func(t *testing.T) {
		v, _ := newIsolatedVault()
		_, err := v.RegisterCounter("my_metric", []string{"label"})
		require.NoError(t, err)

		_, err = v.RegisterHistogram("my_metric", []string{"label"}, []float64{1.0})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "counter")
	})
}

func TestGroupedVault_RegisterCounter_TypeMismatch(t *testing.T) {
	v, _ := newIsolatedVault()
	_, err := v.RegisterGauge("shared_name", []string{"l"})
	require.NoError(t, err)

	_, err = v.RegisterCounter("shared_name", []string{"l"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gauge")
}

func TestGroupedVault_GaugeSet(t *testing.T) {
	v, reg := newIsolatedVault()

	v.GaugeSet("g1", "my_gauge", 100.0, map[string]string{"region": "us"})

	expect := `
# HELP my_gauge my_gauge
# TYPE my_gauge gauge
my_gauge{region="us"} 100
`
	err := promtest.GatherAndCompare(reg, strings.NewReader(expect), "my_gauge")
	assert.NoError(t, err)

	v.GaugeSet("g1", "my_gauge", 200.0, map[string]string{"region": "us"})

	expect = `
# HELP my_gauge my_gauge
# TYPE my_gauge gauge
my_gauge{region="us"} 200
`
	err = promtest.GatherAndCompare(reg, strings.NewReader(expect), "my_gauge")
	assert.NoError(t, err)
}

func TestGroupedVault_GaugeAdd(t *testing.T) {
	v, reg := newIsolatedVault()

	v.GaugeAdd("g1", "add_gauge", 10.0, map[string]string{"k": "v"})
	v.GaugeAdd("g1", "add_gauge", 5.0, map[string]string{"k": "v"})

	expect := `
# HELP add_gauge add_gauge
# TYPE add_gauge gauge
add_gauge{k="v"} 15
`
	err := promtest.GatherAndCompare(reg, strings.NewReader(expect), "add_gauge")
	assert.NoError(t, err)
}

func TestGroupedVault_HistogramObserve(t *testing.T) {
	v, reg := newIsolatedVault()

	v.HistogramObserve("g1", "my_hist", 0.5, map[string]string{"op": "read"}, []float64{0.1, 1.0, 10.0})
	v.HistogramObserve("g1", "my_hist", 5.0, map[string]string{"op": "read"}, []float64{0.1, 1.0, 10.0})

	expect := `
# HELP my_hist my_hist
# TYPE my_hist histogram
my_hist_bucket{op="read",le="0.1"} 0
my_hist_bucket{op="read",le="1"} 1
my_hist_bucket{op="read",le="10"} 2
my_hist_bucket{op="read",le="+Inf"} 2
my_hist_sum{op="read"} 5.5
my_hist_count{op="read"} 2
`
	err := promtest.GatherAndCompare(reg, strings.NewReader(expect), "my_hist")
	assert.NoError(t, err)
}

func TestGroupedVault_ExpireGroupMetrics(t *testing.T) {
	v, reg := newIsolatedVault()

	v.GaugeSet("groupA", "expire_gauge", 1.0, map[string]string{"x": "1"})
	v.GaugeSet("groupB", "expire_gauge", 2.0, map[string]string{"x": "2"})

	v.ExpireGroupMetrics("groupA")

	expect := `
# HELP expire_gauge expire_gauge
# TYPE expire_gauge gauge
expire_gauge{x="2"} 2
`
	err := promtest.GatherAndCompare(reg, strings.NewReader(expect), "expire_gauge")
	assert.NoError(t, err)
}

func TestGroupedVault_ExpireGroupMetricByName(t *testing.T) {
	v, reg := newIsolatedVault()

	v.CounterAdd("g1", "counter_a", 10.0, map[string]string{"k": "v"})
	v.CounterAdd("g1", "counter_b", 20.0, map[string]string{"k": "v"})

	v.ExpireGroupMetricByName("g1", "counter_a")

	expectA := ``
	err := promtest.GatherAndCompare(reg, strings.NewReader(expectA), "counter_a")
	assert.NoError(t, err)

	expectB := `
# HELP counter_b counter_b
# TYPE counter_b counter
counter_b{k="v"} 20
`
	err = promtest.GatherAndCompare(reg, strings.NewReader(expectB), "counter_b")
	assert.NoError(t, err)
}

func TestGroupedVault_ExpireGroupMetricByName_NonExistent(t *testing.T) {
	v, _ := newIsolatedVault()
	assert.NotPanics(t, func() {
		v.ExpireGroupMetricByName("group", "nonexistent")
	})
}

func TestGroupedVault_DescribeAndCollect(t *testing.T) {
	v, _ := newIsolatedVault()

	v.CounterAdd("g1", "dc_counter", 1.0, map[string]string{"a": "1"})
	v.GaugeSet("g1", "dc_gauge", 42.0, map[string]string{"b": "2"})

	descCh := make(chan *prometheus.Desc, 10)
	v.Describe(descCh)
	close(descCh)

	descs := make([]*prometheus.Desc, 0)
	for d := range descCh {
		descs = append(descs, d)
	}
	assert.Len(t, descs, 2)

	metricCh := make(chan prometheus.Metric, 10)
	v.Collect(metricCh)
	close(metricCh)

	metrics := make([]prometheus.Metric, 0)
	for m := range metricCh {
		metrics = append(metrics, m)
	}
	assert.Len(t, metrics, 2)
}

func TestGroupedVault_DescribeAndCollect_Empty(t *testing.T) {
	v, _ := newIsolatedVault()

	descCh := make(chan *prometheus.Desc, 10)
	v.Describe(descCh)
	close(descCh)
	assert.Len(t, descCh, 0)

	metricCh := make(chan prometheus.Metric, 10)
	v.Collect(metricCh)
	close(metricCh)
	assert.Len(t, metricCh, 0)
}

func TestGroupedVault_ConcurrentAccess(t *testing.T) {
	v, _ := newIsolatedVault()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(3)
		go func(id int) {
			defer wg.Done()
			labels := map[string]string{"worker": "counter"}
			for j := 0; j < 50; j++ {
				v.CounterAdd("group", "conc_counter", 1.0, labels)
			}
		}(i)
		go func(id int) {
			defer wg.Done()
			labels := map[string]string{"worker": "gauge"}
			for j := 0; j < 50; j++ {
				v.GaugeSet("group", "conc_gauge", float64(j), labels)
			}
		}(i)
		go func(id int) {
			defer wg.Done()
			labels := map[string]string{"worker": "hist"}
			for j := 0; j < 50; j++ {
				v.HistogramObserve("group", "conc_hist", float64(j)*0.1, labels, []float64{1.0, 5.0})
			}
		}(i)
	}
	wg.Wait()
}

func TestGroupedVault_MultipleGroupsExpire(t *testing.T) {
	v, reg := newIsolatedVault()

	v.GaugeSet("g1", "mg_gauge", 1.0, map[string]string{"src": "g1"})
	v.GaugeSet("g2", "mg_gauge", 2.0, map[string]string{"src": "g2"})
	v.GaugeSet("g3", "mg_gauge", 3.0, map[string]string{"src": "g3"})

	v.ExpireGroupMetrics("g1")
	v.ExpireGroupMetrics("g3")

	expect := `
# HELP mg_gauge mg_gauge
# TYPE mg_gauge gauge
mg_gauge{src="g2"} 2
`
	err := promtest.GatherAndCompare(reg, strings.NewReader(expect), "mg_gauge")
	assert.NoError(t, err)
}

func TestGroupedVault_NilLabels(t *testing.T) {
	v, reg := newIsolatedVault()

	v.CounterAdd("g1", "nil_counter", 5.0, nil)
	v.GaugeSet("g1", "nil_gauge", 10.0, nil)
	v.HistogramObserve("g1", "nil_hist", 1.0, nil, []float64{0.5, 1.0, 5.0})

	expectCounter := `
# HELP nil_counter nil_counter
# TYPE nil_counter counter
nil_counter 5
`
	err := promtest.GatherAndCompare(reg, strings.NewReader(expectCounter), "nil_counter")
	assert.NoError(t, err)

	expectGauge := `
# HELP nil_gauge nil_gauge
# TYPE nil_gauge gauge
nil_gauge 10
`
	err = promtest.GatherAndCompare(reg, strings.NewReader(expectGauge), "nil_gauge")
	assert.NoError(t, err)

	families, err := reg.Gather()
	require.NoError(t, err)
	found := false
	for _, f := range families {
		if f.GetName() == "nil_hist" {
			found = true
			break
		}
	}
	assert.True(t, found, "nil_hist metric should be gathered")
}

// Benchmarks

func BenchmarkGroupedVault_CounterAdd(b *testing.B) {
	v, _ := newIsolatedVault()
	labels := map[string]string{"env": "prod", "service": "api"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.CounterAdd("group", "bench_counter", 1.0, labels)
	}
}

func BenchmarkGroupedVault_GaugeSet(b *testing.B) {
	v, _ := newIsolatedVault()
	labels := map[string]string{"env": "prod", "service": "api"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.GaugeSet("group", "bench_gauge", float64(i), labels)
	}
}

func BenchmarkGroupedVault_GaugeAdd(b *testing.B) {
	v, _ := newIsolatedVault()
	labels := map[string]string{"env": "prod", "service": "api"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.GaugeAdd("group", "bench_gauge_add", 1.0, labels)
	}
}

func BenchmarkGroupedVault_HistogramObserve(b *testing.B) {
	v, _ := newIsolatedVault()
	labels := map[string]string{"env": "prod", "service": "api"}
	buckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.HistogramObserve("group", "bench_hist", float64(i%100)*0.01, labels, buckets)
	}
}

func BenchmarkGroupedVault_RegisterCounter(b *testing.B) {
	v, _ := newIsolatedVault()
	// First call registers; subsequent calls hit the fast path.
	v.RegisterCounter("bench_counter", []string{"env"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.RegisterCounter("bench_counter", []string{"env"})
	}
}

func BenchmarkGroupedVault_RegisterGauge(b *testing.B) {
	v, _ := newIsolatedVault()
	v.RegisterGauge("bench_gauge", []string{"env"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.RegisterGauge("bench_gauge", []string{"env"})
	}
}

func BenchmarkGroupedVault_RegisterHistogram(b *testing.B) {
	v, _ := newIsolatedVault()
	buckets := []float64{0.1, 1.0, 10.0}
	v.RegisterHistogram("bench_hist", []string{"env"}, buckets)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.RegisterHistogram("bench_hist", []string{"env"}, buckets)
	}
}

func BenchmarkGroupedVault_ExpireGroupMetrics(b *testing.B) {
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		v, _ := newIsolatedVault()
		for j := 0; j < 50; j++ {
			v.CounterAdd("target", "bench_counter", 1.0, map[string]string{"idx": string(rune(j))})
			v.GaugeSet("target", "bench_gauge", float64(j), map[string]string{"idx": string(rune(j))})
		}
		b.StartTimer()
		v.ExpireGroupMetrics("target")
		b.StopTimer()
	}
}

func BenchmarkGroupedVault_CounterAdd_Parallel(b *testing.B) {
	v, _ := newIsolatedVault()
	labels := map[string]string{"env": "prod"}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			v.CounterAdd("group", "bench_counter_par", 1.0, labels)
		}
	})
}

func BenchmarkGroupedVault_GaugeSet_Parallel(b *testing.B) {
	v, _ := newIsolatedVault()
	labels := map[string]string{"env": "prod"}
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			v.GaugeSet("group", "bench_gauge_par", float64(i), labels)
			i++
		}
	})
}

func BenchmarkGroupedVault_HistogramObserve_Parallel(b *testing.B) {
	v, _ := newIsolatedVault()
	labels := map[string]string{"env": "prod"}
	buckets := []float64{0.1, 1.0, 10.0}
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			v.HistogramObserve("group", "bench_hist_par", float64(i%100)*0.01, labels, buckets)
			i++
		}
	})
}
