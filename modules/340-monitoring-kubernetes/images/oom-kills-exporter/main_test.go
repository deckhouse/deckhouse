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

// Тест checkMetricExistenceByLabels
func TestCheckMetricExistenceByLabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_metric"}, []string{"label1", "label2"})
	registry.MustRegister(counter)

	labels := map[string]string{"label1": "value1", "label2": "value2"}
	counter.With(labels).Inc()

	exists := checkMetricExistenceByLabels("test_metric", labels, registry)
	assert.True(t, exists, "Metric with labels should exist in registry")

	labels = map[string]string{"label1": "value1", "label2": "nonexistent"}
	exists = checkMetricExistenceByLabels("test_metric", labels, registry)
	assert.False(t, exists, "Metric with non-existent labels should not exist")
}

// Тест getContainerIDFromLog
func TestGetContainerIDFromLog(t *testing.T) {
	line := "oom-kill: task_memcg=/kubepods/burstable/pod123"
	containerID := getContainerIDFromLog(line)
	assert.Equal(t, "/kubepods/burstable/pod123", containerID, "Should extract correct task_memcg")

	line = "oom-kill: no task_memcg present"
	containerID = getContainerIDFromLog(line)
	assert.Equal(t, "", containerID, "Should return empty string if task_memcg is not present")
}
