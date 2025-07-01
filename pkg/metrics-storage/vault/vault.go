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

package vault

import (
	"fmt"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
	labelspkg "github.com/deckhouse/deckhouse/pkg/metrics-storage/labels"
)

type GroupedVault struct {
	collectors            map[string]collectors.ConstCollector
	mtx                   sync.Mutex
	registerer            prometheus.Registerer
	resolveMetricNameFunc func(name string) string
}

func NewGroupedVault(resolveMetricNameFunc func(name string) string) *GroupedVault {
	return &GroupedVault{
		collectors:            make(map[string]collectors.ConstCollector),
		resolveMetricNameFunc: resolveMetricNameFunc,
	}
}

func (v *GroupedVault) Registerer() prometheus.Registerer {
	return v.registerer
}

func (v *GroupedVault) SetRegisterer(r prometheus.Registerer) {
	v.registerer = r
}

// ExpireGroupMetrics takes each collector in collectors and clear all metrics by group.
func (v *GroupedVault) ExpireGroupMetrics(group string) {
	v.mtx.Lock()
	for _, collector := range v.collectors {
		collector.ExpireGroupMetrics(group)
	}
	v.mtx.Unlock()
}

// ExpireGroupMetricByName gets a collector by its name and clears all metrics inside the collector by the group.
func (v *GroupedVault) ExpireGroupMetricByName(group, name string) {
	metricName := v.resolveMetricNameFunc(name)
	v.mtx.Lock()
	collector, ok := v.collectors[metricName]
	if ok {
		collector.ExpireGroupMetrics(group)
	}
	v.mtx.Unlock()
}

func (v *GroupedVault) GetOrCreateCounterCollector(name string, labelNames []string) (*collectors.ConstCounterCollector, error) {
	metricName := v.resolveMetricNameFunc(name)
	v.mtx.Lock()
	defer v.mtx.Unlock()
	collector, ok := v.collectors[metricName]
	if !ok {
		collector = collectors.NewConstCounterCollector(metricName, labelNames)
		if err := v.registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("counter '%s' %v registration: %v", metricName, labelNames, err)
		}
		v.collectors[metricName] = collector
	} else if !labelspkg.IsSubset(collector.LabelNames(), labelNames) {
		collector.UpdateLabels(labelNames)
	}
	if counter, ok := collector.(*collectors.ConstCounterCollector); ok {
		return counter, nil
	}
	return nil, fmt.Errorf("counter %v collector requested, but %s %v collector exists", labelNames, collector.Type(), collector.LabelNames())
}

func (v *GroupedVault) GetOrCreateGaugeCollector(name string, labelNames []string) (*collectors.ConstGaugeCollector, error) {
	metricName := v.resolveMetricNameFunc(name)
	v.mtx.Lock()
	defer v.mtx.Unlock()
	collector, ok := v.collectors[metricName]
	if !ok {
		collector = collectors.NewConstGaugeCollector(metricName, labelNames)
		if err := v.registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("gauge '%s' %v registration: %v", metricName, labelNames, err)
		}
		v.collectors[metricName] = collector
	} else if !labelspkg.IsSubset(collector.LabelNames(), labelNames) {
		collector.UpdateLabels(labelNames)
	}

	if gauge, ok := collector.(*collectors.ConstGaugeCollector); ok {
		return gauge, nil
	}
	return nil, fmt.Errorf("gauge %v collector requested, but %s %v collector exists", labelNames, collector.Type(), collector.LabelNames())
}

func (v *GroupedVault) CounterAdd(group string, name string, value float64, labels map[string]string) {
	metricName := v.resolveMetricNameFunc(name)
	c, err := v.GetOrCreateCounterCollector(metricName, labelspkg.LabelNames(labels))
	if err != nil {
		log.Error("CounterAdd", log.Err(err))
		return
	}
	c.Add(group, value, labels)
}

func (v *GroupedVault) GaugeSet(group string, name string, value float64, labels map[string]string) {
	metricName := v.resolveMetricNameFunc(name)
	c, err := v.GetOrCreateGaugeCollector(metricName, labelspkg.LabelNames(labels))
	if err != nil {
		log.Error("GaugeSet", log.Err(err))
		return
	}
	c.Set(group, value, labels)
}
