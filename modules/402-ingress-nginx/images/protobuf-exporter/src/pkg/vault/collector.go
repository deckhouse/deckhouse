/*
Copyright 2021 Flant JSC

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

package vault

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"

	"github.com/flant/protobuf_exporter/pkg/stats"
)

type ConstMetricCollector interface {
	GetType() MappingType
	Describe(ch chan<- *prometheus.Desc)
	Collect(ch chan<- prometheus.Metric)
	Store(labelsHash uint64, labels []string, timestamp time.Time, value interface{})
	Clear(now time.Time)
}

var (
	_ ConstMetricCollector = (*ConstHistogramCollector)(nil)
	_ ConstMetricCollector = (*ConstCounterCollector)(nil)
	_ ConstMetricCollector = (*ConstGaugeCollector)(nil)
)

type StampedHistogramMetric struct {
	Count       uint64
	Sum         float64 // TODO: use uint64 like in prometheus for atomic operations
	Buckets     map[float64]uint64
	LabelValues []string
	LastUpdate  time.Time
}

// CopyBuckets deep copies buckets to avoid changes during iteration on promhttp load
func (s *StampedHistogramMetric) CopyBuckets() map[float64]uint64 {
	copyBuckets := make(map[float64]uint64, len(s.Buckets))
	for key, value := range s.Buckets {
		copyBuckets[key] = value
	}
	return copyBuckets
}

type StampedCounterMetric struct {
	Value       uint64
	LabelValues []string
	LastUpdate  time.Time
}

type StampedGaugeMetric struct {
	Value       float64
	LabelValues []string
	LastUpdate  time.Time
}

type BucketValue struct {
	Sum     float64
	Count   uint64
	Buckets map[float64]uint64
}

type ConstHistogramCollector struct {
	mtx sync.RWMutex

	collection map[uint64]StampedHistogramMetric
	desc       *prometheus.Desc
	mapping    Mapping
}

func NewConstHistogramCollector(mapping Mapping) *ConstHistogramCollector {
	desc := prometheus.NewDesc(mapping.Name, mapping.Help, mapping.LabelNames, nil)
	return &ConstHistogramCollector{mapping: mapping, collection: make(map[uint64]StampedHistogramMetric), desc: desc}
}

func (c *ConstHistogramCollector) GetType() MappingType {
	return c.mapping.Type
}

func (c *ConstHistogramCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *ConstHistogramCollector) Collect(ch chan<- prometheus.Metric) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for _, s := range c.collection {
		metric, err := prometheus.NewConstHistogram(c.desc, s.Count, s.Sum, s.CopyBuckets(), s.LabelValues...)
		if err != nil {
			log.Warnf("prepare histogram: %v", err)
			stats.Errors.WithLabelValues("prepare-histogram").Inc()
			continue
		}
		ch <- metric
	}
}

func (c *ConstHistogramCollector) Store(labelsHash uint64, labels []string, timestamp time.Time, value interface{}) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	histogramValue := value.(BucketValue)

	storedMetric, ok := c.collection[labelsHash]
	if !ok {
		storedMetric = StampedHistogramMetric{Buckets: make(map[float64]uint64, len(c.mapping.Buckets)), LabelValues: labels}
	}

	storedMetric.Count += histogramValue.Count
	storedMetric.Sum += histogramValue.Sum

	incrementer := uint64(0)
	for _, bucket := range c.mapping.Buckets {
		if receivedBucketCount, ok := histogramValue.Buckets[bucket]; ok {
			incrementer += receivedBucketCount
		}
		storedMetric.Buckets[bucket] += incrementer
	}

	storedMetric.LastUpdate = timestamp
	c.collection[labelsHash] = storedMetric
}

func (c *ConstHistogramCollector) Clear(now time.Time) {
	if c.mapping.TTL == 0 {
		return
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()

	for labelsHash, singleMetric := range c.collection {
		if singleMetric.LastUpdate.Add(c.mapping.TTL).Before(now) {
			delete(c.collection, labelsHash)
		}
	}
}

type ConstCounterCollector struct {
	mtx sync.RWMutex

	collection map[uint64]StampedCounterMetric
	desc       *prometheus.Desc
	mapping    Mapping
}

func NewConstCounterCollector(mapping Mapping) *ConstCounterCollector {
	desc := prometheus.NewDesc(mapping.Name, mapping.Help, mapping.LabelNames, nil)
	return &ConstCounterCollector{mapping: mapping, collection: make(map[uint64]StampedCounterMetric), desc: desc}
}

func (c *ConstCounterCollector) GetType() MappingType {
	return c.mapping.Type
}

func (c *ConstCounterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *ConstCounterCollector) Collect(ch chan<- prometheus.Metric) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for _, s := range c.collection {
		metric, err := prometheus.NewConstMetric(c.desc, prometheus.CounterValue, float64(s.Value), s.LabelValues...)
		if err != nil {
			log.Warnf("prepare counter: %v", err)
			stats.Errors.WithLabelValues("prepare-counter").Inc()
			continue
		}
		ch <- metric
	}
}

func (c *ConstCounterCollector) Store(labelsHash uint64, labels []string, timestamp time.Time, value interface{}) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	counterValue := value.(uint64)
	storedMetric, ok := c.collection[labelsHash]
	if !ok {
		storedMetric = StampedCounterMetric{Value: counterValue, LabelValues: labels}
	} else {
		atomic.AddUint64(&storedMetric.Value, counterValue)
	}

	storedMetric.LastUpdate = timestamp
	c.collection[labelsHash] = storedMetric
}

func (c *ConstCounterCollector) Clear(now time.Time) {
	if c.mapping.TTL == 0 {
		return
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()

	for labelsHash, singleMetric := range c.collection {
		if singleMetric.LastUpdate.Add(c.mapping.TTL).Before(now) {
			delete(c.collection, labelsHash)
		}
	}
}

type ConstGaugeCollector struct {
	mtx sync.RWMutex

	collection map[uint64]StampedGaugeMetric
	desc       *prometheus.Desc
	mapping    Mapping
}

func NewConstGaugeCollector(mapping Mapping) *ConstGaugeCollector {
	desc := prometheus.NewDesc(mapping.Name, mapping.Help, mapping.LabelNames, nil)
	return &ConstGaugeCollector{mapping: mapping, collection: make(map[uint64]StampedGaugeMetric), desc: desc}
}

func (c *ConstGaugeCollector) GetType() MappingType {
	return c.mapping.Type
}

func (c *ConstGaugeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *ConstGaugeCollector) Collect(ch chan<- prometheus.Metric) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for _, s := range c.collection {
		metric, err := prometheus.NewConstMetric(c.desc, prometheus.GaugeValue, s.Value, s.LabelValues...)
		if err != nil {
			log.Warnf("prepare gauge: %v", err)
			stats.Errors.WithLabelValues("prepare-gauge").Inc()
			continue
		}
		ch <- metric
	}
}

func (c *ConstGaugeCollector) Store(labelsHash uint64, labels []string, timestamp time.Time, value interface{}) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	gaugeValue := value.(float64)
	storedMetric, ok := c.collection[labelsHash]
	if !ok {
		storedMetric = StampedGaugeMetric{Value: gaugeValue, LabelValues: labels}
	}

	storedMetric.Value = gaugeValue
	storedMetric.LastUpdate = timestamp
	c.collection[labelsHash] = storedMetric
}

func (c *ConstGaugeCollector) Clear(now time.Time) {
	if c.mapping.TTL == 0 {
		return
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()

	for labelsHash, singleMetric := range c.collection {
		if singleMetric.LastUpdate.Add(c.mapping.TTL).Before(now) {
			delete(c.collection, labelsHash)
		}
	}
}
