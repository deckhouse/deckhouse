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

package collectors

import (
	"unicode/utf8"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// Collect sends the metrics below instead of the ones of prometheus.MustNewConstMetric and
// prometheus.MustNewConstHistogram. They write the same dto, but a series takes a few block
// allocations per scrape instead of three per label and three per bucket, and nothing is sorted.

// labelPairsTemplate is the sorted label set of a desc. It is built once per desc, so a scrape
// does not sort the label pairs of every series as prometheus.MakeLabelPairs does.
type labelPairsTemplate struct {
	// names holds the name of each variable label in the desc order, shared by all series.
	names []*string
	// order maps a position in the sorted label set to a variable label index, or to -1 for a constant label.
	order []int
	// constPairs holds the constant label pairs of the desc at their sorted positions.
	constPairs []*dto.LabelPair
}

// newLabelPairsTemplate returns nil for an invalid desc: Collect then falls back to the prometheus
// constructors, which panic on it.
func newLabelPairsTemplate(desc *prometheus.Desc, labelNames []string) *labelPairsTemplate {
	// The label values are placeholders: the written label pairs give the sorted label names
	// and the constant label pairs of the desc.
	m, err := prometheus.NewConstMetric(desc, prometheus.UntypedValue, 0, make([]string, len(labelNames))...)
	if err != nil {
		return nil
	}

	sample := &dto.Metric{}
	if err = m.Write(sample); err != nil {
		return nil
	}

	variable := make(map[string]int, len(labelNames))
	for idx, name := range labelNames {
		variable[name] = idx
	}

	t := &labelPairsTemplate{
		names:      make([]*string, len(labelNames)),
		order:      make([]int, len(sample.Label)),
		constPairs: make([]*dto.LabelPair, len(sample.Label)),
	}

	for pos, pair := range sample.Label {
		idx, ok := variable[pair.GetName()]
		if !ok {
			t.order[pos] = -1
			t.constPairs[pos] = pair
			continue
		}

		t.order[pos] = idx
		t.names[idx] = pair.Name
	}

	return t
}

// valid reports whether the prometheus constructors accept the label values for the desc of the template.
func (t *labelPairsTemplate) valid(labelValues []string) bool {
	if t == nil || len(labelValues) != len(t.names) {
		return false
	}

	for _, value := range labelValues {
		if !utf8.ValidString(value) {
			return false
		}
	}

	return true
}

// labelPair keeps a label pair with the value it points to, so the pairs of a series take one allocation.
type labelPair struct {
	pair  dto.LabelPair
	value string
}

// labelPairs returns the label pairs of a series as prometheus.MakeLabelPairs does.
func (t *labelPairsTemplate) labelPairs(labelValues []string) []*dto.LabelPair {
	if len(t.order) == 0 {
		return nil
	}

	// Without variable labels MakeLabelPairs returns the constant label pairs of the desc.
	if len(labelValues) == 0 {
		return t.constPairs
	}

	values := make([]labelPair, len(labelValues))
	pairs := make([]*dto.LabelPair, len(t.order))

	for pos, idx := range t.order {
		if idx < 0 {
			pairs[pos] = t.constPairs[pos]
			continue
		}

		lp := &values[idx]
		lp.value = labelValues[idx]
		lp.pair.Name = t.names[idx]
		lp.pair.Value = &lp.value
		pairs[pos] = &lp.pair
	}

	return pairs
}

// counterMetric writes the dto of prometheus.MustNewConstMetric with prometheus.CounterValue.
type counterMetric struct {
	desc    *prometheus.Desc
	labels  []*dto.LabelPair
	value   float64
	counter dto.Counter
}

func newCounterMetric(desc *prometheus.Desc, t *labelPairsTemplate, value float64, labelValues []string) prometheus.Metric {
	if !t.valid(labelValues) {
		return prometheus.MustNewConstMetric(desc, prometheus.CounterValue, value, labelValues...)
	}

	m := &counterMetric{
		desc:   desc,
		labels: t.labelPairs(labelValues),
		value:  value,
	}
	m.counter.Value = &m.value

	return m
}

func (m *counterMetric) Desc() *prometheus.Desc {
	return m.desc
}

func (m *counterMetric) Write(out *dto.Metric) error {
	out.Label = m.labels
	out.Counter = &m.counter
	out.Gauge = nil
	out.Untyped = nil

	return nil
}

// gaugeMetric writes the dto of prometheus.MustNewConstMetric with prometheus.GaugeValue.
type gaugeMetric struct {
	desc   *prometheus.Desc
	labels []*dto.LabelPair
	value  float64
	gauge  dto.Gauge
}

func newGaugeMetric(desc *prometheus.Desc, t *labelPairsTemplate, value float64, labelValues []string) prometheus.Metric {
	if !t.valid(labelValues) {
		return prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labelValues...)
	}

	m := &gaugeMetric{
		desc:   desc,
		labels: t.labelPairs(labelValues),
		value:  value,
	}
	m.gauge.Value = &m.value

	return m
}

func (m *gaugeMetric) Desc() *prometheus.Desc {
	return m.desc
}

func (m *gaugeMetric) Write(out *dto.Metric) error {
	out.Label = m.labels
	out.Counter = nil
	out.Gauge = &m.gauge
	out.Untyped = nil

	return nil
}

// histogramBucket keeps a bucket with the values it points to, so the buckets of a series take one allocation.
type histogramBucket struct {
	bucket     dto.Bucket
	count      uint64
	upperBound float64
}

// histogramMetric writes the dto of prometheus.MustNewConstHistogram.
type histogramMetric struct {
	desc      *prometheus.Desc
	labels    []*dto.LabelPair
	count     uint64
	sum       float64
	histogram dto.Histogram
}

// newHistogramMetric takes the sorted upper bounds and the cumulative count of each of them.
func newHistogramMetric(desc *prometheus.Desc, t *labelPairsTemplate, count uint64, sum float64, upperBounds []float64, counts []uint64, labelValues []string) prometheus.Metric {
	if !t.valid(labelValues) {
		buckets := make(map[float64]uint64, len(upperBounds))
		for i, upperBound := range upperBounds {
			buckets[upperBound] = counts[i]
		}

		return prometheus.MustNewConstHistogram(desc, count, sum, buckets, labelValues...)
	}

	// MustNewConstHistogram takes the buckets as a map, so of equal upper bounds only the last one is written.
	unique := 0
	for i := range upperBounds {
		if isOverwritten(upperBounds, i) {
			continue
		}
		unique++
	}

	values := make([]histogramBucket, unique)
	buckets := make([]*dto.Bucket, 0, unique)

	for i := range upperBounds {
		if isOverwritten(upperBounds, i) {
			continue
		}

		b := &values[len(buckets)]
		b.count = counts[i]
		b.upperBound = upperBounds[i]
		b.bucket.CumulativeCount = &b.count
		b.bucket.UpperBound = &b.upperBound
		buckets = append(buckets, &b.bucket)
	}

	m := &histogramMetric{
		desc:   desc,
		labels: t.labelPairs(labelValues),
		count:  count,
		sum:    sum,
	}
	m.histogram.SampleCount = &m.count
	m.histogram.SampleSum = &m.sum
	m.histogram.Bucket = buckets

	return m
}

// isOverwritten reports whether the next sorted upper bound is equal to the i-th one.
func isOverwritten(upperBounds []float64, i int) bool {
	return i+1 < len(upperBounds) && upperBounds[i+1] == upperBounds[i]
}

func (m *histogramMetric) Desc() *prometheus.Desc {
	return m.desc
}

func (m *histogramMetric) Write(out *dto.Metric) error {
	out.Histogram = &m.histogram
	out.Label = m.labels

	return nil
}
