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

package collectors

import (
	"hash/fnv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/constraints"
)

type ConstCollector interface {
	Describe(ch chan<- *prometheus.Desc)
	Collect(ch chan<- prometheus.Metric)
	Type() string
	LabelNames() []string
	Name() string
	ExpireGroupMetrics(group string)
	UpdateLabels([]string)
}

func NewMetricValue[T constraints.Ordered](val T) *MetricValue[T] {
	return &MetricValue[T]{
		Value: val,
	}
}

type MetricValue[T constraints.Ordered] struct {
	mu    sync.Mutex
	Value T
}

func (v *MetricValue[T]) Set(value T) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Value = value
}

func (v *MetricValue[T]) Get() T {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.Value
}

func (v *MetricValue[T]) Add(value T) T {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Value += value
	return v.Value
}

type MetricDescription struct {
	Name        string
	Help        string
	LabelNames  []string
	ConstLabels map[string]string
}

const labelsSeparator = byte(255)

func HashMetric(group string, labelValues []string) uint64 {
	hasher := fnv.New64a()

	if group != "" {
		_, _ = hasher.Write([]byte(group))
		_, _ = hasher.Write([]byte{labelsSeparator})
	}

	for _, labelValue := range labelValues {
		_, _ = hasher.Write([]byte(labelValue))
		_, _ = hasher.Write([]byte{labelsSeparator})
	}
	return hasher.Sum64()
}

type ConstCollectorOptions struct {
	Group string
}

type ConstCollectorOption func(*ConstCollectorOptions)

// WithGroup sets a group for the counter collector.
func WithGroup(group string) ConstCollectorOption {
	return func(opts *ConstCollectorOptions) {
		opts.Group = group
	}
}

func NewConstCollectorOptions(opts ...ConstCollectorOption) *ConstCollectorOptions {
	options := &ConstCollectorOptions{
		Group: "",
	}

	for _, opt := range opts {
		opt(options)
	}

	return options
}
