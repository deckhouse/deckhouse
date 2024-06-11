/*
Copyright 2024 Flant JSC

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

package monitoring

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	reconcilesCountTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "reconciles_count_total",
		Help: "Total number of times the resources were reconciled.",
	}, []string{"node", "controller"})

	reconcileDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "reconcile_duration_seconds",
		Help:       "How long in seconds reconciling of resource takes.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"node", "controller"})

	utilsCommandsDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "custom_utils_commands_duration_seconds",
		Help:       "How long in seconds utils commands execution takes.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"node", "controller", "command"})

	utilsCommandsExecutionCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "utils_commands_execution_count_total",
		Help: "Total number of times the util-command was executed.",
	}, []string{"node", "controller", "method"})

	utilsCommandsErrorsCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "utils_commands_errors_count_total",
		Help: "How many errors occurs during utils-command executions.",
	}, []string{"node", "controller", "method"})

	apiMethodsDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "api_commands_duration_seconds",
		Help:       "How long in seconds kube-api methods execution takes.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"node", "controller", "method"})

	apiMethodsExecutionCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "api_methods_execution_count_total",
		Help: "Total number of times the method was executed.",
	}, []string{"node", "controller", "method"})

	apiMethodsErrorsCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "api_methods_errors_count_total",
		Help: "How many errors occur during api-method executions.",
	}, []string{"node", "controller", "method"})

	noOperationalResourcesCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "no_operational_resources_count_total",
		Help: "How many LVMVolumeGroup resources are in Nooperational state.",
	}, []string{"resource"})
)

func init() {
	metrics.Registry.MustRegister(reconcilesCountTotal)
	metrics.Registry.MustRegister(reconcileDuration)
	metrics.Registry.MustRegister(utilsCommandsDuration)
	metrics.Registry.MustRegister(apiMethodsDuration)
	metrics.Registry.MustRegister(apiMethodsExecutionCount)
	metrics.Registry.MustRegister(apiMethodsErrorsCount)
	metrics.Registry.MustRegister(noOperationalResourcesCount)
}

type Metrics struct {
	node string
	c    clock.Clock
}

func GetMetrics(nodeName string) Metrics {
	return Metrics{
		node: nodeName,
		c:    clock.RealClock{},
	}
}

func (m Metrics) GetEstimatedTimeInSeconds(since time.Time) float64 {
	return m.c.Since(since).Seconds()
}

func (m Metrics) ReconcilesCountTotal(controllerName string) prometheus.Counter {
	return reconcilesCountTotal.WithLabelValues(m.node, controllerName)
}

func (m Metrics) ReconcileDuration(controllerName string) prometheus.Observer {
	return reconcileDuration.WithLabelValues(m.node, controllerName)
}

func (m Metrics) UtilsCommandsDuration(controllerName, command string) prometheus.Observer {
	return utilsCommandsDuration.WithLabelValues(m.node, controllerName, strings.ToLower(command))
}

func (m Metrics) UtilsCommandsExecutionCount(controllerName, command string) prometheus.Counter {
	return utilsCommandsExecutionCount.WithLabelValues(m.node, controllerName, strings.ToLower(command))
}

func (m Metrics) UtilsCommandsErrorsCount(controllerName, command string) prometheus.Counter {
	return utilsCommandsErrorsCount.WithLabelValues(m.node, controllerName, strings.ToLower(command))
}

func (m Metrics) ApiMethodsDuration(controllerName, method string) prometheus.Observer {
	return apiMethodsDuration.WithLabelValues(m.node, controllerName, strings.ToLower(method))
}

func (m Metrics) ApiMethodsExecutionCount(controllerName, method string) prometheus.Counter {
	return apiMethodsExecutionCount.WithLabelValues(m.node, controllerName, strings.ToLower(method))
}

func (m Metrics) ApiMethodsErrors(controllerName, method string) prometheus.Counter {
	return apiMethodsErrorsCount.WithLabelValues(m.node, controllerName, strings.ToLower(method))
}

func (m Metrics) NoOperationalResourcesCount(resourceName string) prometheus.Gauge {
	return noOperationalResourcesCount.WithLabelValues(strings.ToLower(resourceName))
}
