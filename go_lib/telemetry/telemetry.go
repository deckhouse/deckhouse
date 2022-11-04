/*
Copyright 2022 Flant JSC

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

package telemetry

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
)

type metricsCollector struct {
	collector go_hook.MetricsCollector
}

type Option struct {
	Group string
}

type Options []*Option

func NewOptions() Options {
	return []*Option{{}}
}

func (o Options) WithGroup(name string) Options {
	o[0].Group = name
	return o
}

// MetricsCollector collects metric's records for exporting them as a batch
type MetricsCollector interface {
	// Inc increments the specified Counter metric
	Inc(name string, labels map[string]string, opt ...Options)
	// Add adds custom value for the specified Counter metric
	Add(name string, value float64, labels map[string]string, opt ...Options)
	// Set specifies the custom value for the Gauge metric
	Set(name string, value float64, labels map[string]string, opt ...Options)
	// Expire marks metric's group as expired
	Expire(group string)
}

func NewTelemetryMetricCollector(input *go_hook.HookInput) MetricsCollector {
	return &metricsCollector{
		collector: input.MetricsCollector,
	}
}

// Inc increments the specified Counter metric
func (m *metricsCollector) Inc(name string, labels map[string]string, opt ...Options) {
	m.collector.Inc(m.name(name), labels, m.opts(opt...)...)
}

// Add adds custom value for the specified Counter metric
func (m *metricsCollector) Add(name string, value float64, labels map[string]string, opt ...Options) {
	m.collector.Add(m.name(name), value, labels, m.opts(opt...)...)
}

// Set specifies the custom value for the Gauge metric
func (m *metricsCollector) Set(name string, value float64, labels map[string]string, opt ...Options) {
	m.collector.Set(m.name(name), value, labels, m.opts(opt...)...)
}

// Expire marks metric's group as expired
func (m *metricsCollector) Expire(group string) {
	m.collector.Expire(m.name(group))
}

func (m *metricsCollector) opts(opts ...Options) []metrics.Option {
	if len(opts) == 1 {
		if opts[0][0].Group != "" {
			return []metrics.Option{
				metrics.WithGroup(m.name(opts[0][0].Group)),
			}
		}
	}

	return make([]metrics.Option, 0, 0)
}

func (m *metricsCollector) name(n string) string {
	return WrapName(n)
}

func WrapName(n string) string {
	return fmt.Sprintf("d8_telemetry_%s", n)
}
