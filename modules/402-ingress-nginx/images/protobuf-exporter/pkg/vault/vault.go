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
	"bytes"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const labelsSeparator = byte(255)

type MetricsVault struct {
	metrics []ConstMetricCollector
	now     func() time.Time
}

func NewVault() *MetricsVault {
	return &MetricsVault{now: time.Now}
}

func (v *MetricsVault) RegisterMappings(mappings []Mapping) error {
	for _, mapping := range mappings {
		switch mapping.Type {
		case CounterMapping:
			collector := NewConstCounterCollector(mapping)
			v.metrics = append(v.metrics, collector)

			if err := prometheus.Register(collector); err != nil {
				return fmt.Errorf("mapping registration: %v", err)
			}
		case GaugeMapping:
			collector := NewConstGaugeCollector(mapping)
			v.metrics = append(v.metrics, collector)

			if err := prometheus.Register(collector); err != nil {
				return fmt.Errorf("mapping registration: %v", err)
			}
		case HistogramMapping:
			collector := NewConstHistogramCollector(mapping)
			v.metrics = append(v.metrics, collector)

			if err := prometheus.Register(collector); err != nil {
				return fmt.Errorf("mapping registration: %v", err)
			}
		default:
			return fmt.Errorf("unknown mapping type %s", mapping.Type)
		}
	}
	return nil
}

func (v *MetricsVault) StoreHistogram(index int, labels []string, count uint64, sum float64, buckets map[float64]uint64) error {
	binding := v.metrics[index]
	if binding.GetType() != HistogramMapping {
		return fmt.Errorf("wrong mapping for index #%v", index)
	}
	binding.Store(hashLabels(labels), labels, v.now(), BucketValue{Count: count, Sum: sum, Buckets: buckets})
	return nil
}

func (v *MetricsVault) StoreCounter(index int, labels []string, value uint64) error {
	binding := v.metrics[index]
	if binding.GetType() != CounterMapping {
		return fmt.Errorf("wrong mapping for index #%v", index)
	}
	binding.Store(hashLabels(labels), labels, v.now(), value)
	return nil
}

func (v *MetricsVault) StoreGauge(index int, labels []string, value float64) error {
	binding := v.metrics[index]
	if binding.GetType() != GaugeMapping {
		return fmt.Errorf("wrong mapping for index #%v", index)
	}
	binding.Store(hashLabels(labels), labels, v.now(), value)
	return nil
}

func hashLabels(labels []string) uint64 {
	hasher := fnv.New64a()
	var hashbuf bytes.Buffer
	for _, labelValue := range labels {
		hashbuf.WriteString(labelValue)
		hashbuf.WriteByte(labelsSeparator)
	}

	_, _ = hasher.Write(hashbuf.Bytes())
	return hasher.Sum64()
}

func (v *MetricsVault) RemoveStaleMetrics() {
	currentTime := v.now()

	for _, m := range v.metrics {
		m.Clear(currentTime)
	}
}
