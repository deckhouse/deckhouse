/*
Copyright 2025 Flant JSC

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

package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestEnabledLabel(t *testing.T) {
	labels := map[string]string{
		"extended-monitoring.deckhouse.io/enabled": "true",
	}
	assert.Equal(t, 1.0, enabledLabel(labels))

	labels["extended-monitoring.deckhouse.io/enabled"] = "false"
	assert.Equal(t, 0.0, enabledLabel(labels))

	delete(labels, "extended-monitoring.deckhouse.io/enabled")
	assert.Equal(t, 1.0, enabledLabel(labels))
}

func TestThresholdLabel(t *testing.T) {
	labels := map[string]string{
		"threshold.extended-monitoring.deckhouse.io/cpu": "80",
	}
	assert.Equal(t, 80.0, thresholdLabel(labels, "cpu", 100.0))

	labels["threshold.extended-monitoring.deckhouse.io/cpu"] = "invalid"
	assert.Equal(t, 100.0, thresholdLabel(labels, "cpu", 100.0))
}

func TestRecordMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	nodeEnabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_node_enabled"},
		[]string{"node"},
	)
	reg.MustRegister(nodeEnabled)

	nodeEnabled.WithLabelValues("node1").Add(1)
	nodeEnabled.WithLabelValues("node2").Add(0)

	metricFamilies, err := reg.Gather()
	assert.NoError(t, err)
	assert.Len(t, metricFamilies, 1)

	metricFamily := metricFamilies[0]
	assert.Equal(t, "test_node_enabled", metricFamily.GetName())
	assert.Len(t, metricFamily.Metric, 2)

	for _, metric := range metricFamily.Metric {
		nodeName := metric.GetLabel()[0].GetValue()
		value := metric.GetCounter().GetValue()
		if nodeName == "node1" {
			assert.Equal(t, 1.0, value)
		} else if nodeName == "node2" {
			assert.Equal(t, 0.0, value)
		} else {
			t.Errorf("Unexpected node: %s", nodeName)
		}
	}
}
