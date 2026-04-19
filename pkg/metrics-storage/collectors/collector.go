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

const (
	labelsSeparator = byte(255)
	fnvOffset64     = uint64(14695981039346656037)
	fnvPrime64      = uint64(1099511628211)
)

func HashMetric(group string, labelValues []string) uint64 {
	h := fnvOffset64

	if group != "" {
		for i := 0; i < len(group); i++ {
			h ^= uint64(group[i])
			h *= fnvPrime64
		}
		h ^= uint64(labelsSeparator)
		h *= fnvPrime64
	}

	for _, lv := range labelValues {
		for i := 0; i < len(lv); i++ {
			h ^= uint64(lv[i])
			h *= fnvPrime64
		}
		h ^= uint64(labelsSeparator)
		h *= fnvPrime64
	}

	return h
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

func resolveGroup(opts []ConstCollectorOption) string {
	if len(opts) == 0 {
		return ""
	}
	var o ConstCollectorOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o.Group
}
