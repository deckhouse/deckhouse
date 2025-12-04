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

package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	dto "github.com/prometheus/client_model/go"
)

func TestGetContainerIDFromLog(t *testing.T) {
	logLine := "oom-kill:constraint=CONSTRAINT_MEMCG,nodemask=(null),cpuset=9f02d9fa0049eb2655fc83c765f142362b2cb403b57b70ba3185071015ca3b64,mems_allowed=0-1,oom_memcg=/kubepods/burstable/podd11ab7b0-d6db-4a24-a7de-4a2faf1e6980/9f02d9fa0049eb2655fc83c765f142362b2cb403b57b70ba3185071015ca3b64,task_memcg=/kubepods/burstable/podd11ab7b0-d6db-4a24-a7de-4a2faf1e6980/9f02d9fa0049eb2655fc83c765f142362b2cb403b57b70ba3185071015ca3b64,task=prometheus-conf,pid=3401999,uid=0"
	podUID, containerID := getContainerIDFromLog(logLine)
	assert.Equal(t, "d11ab7b0-d6db-4a24-a7de-4a2faf1e6980", podUID)
	assert.Equal(t, "9f02d9fa0049eb2655fc83c765f142362b2cb403b57b70ba3185071015ca3b64", containerID)

	logLine = "oom-kill: no task_memcg present"
	podUID, containerID = getContainerIDFromLog(logLine)
	assert.Equal(t, "", podUID)
	assert.Equal(t, "", containerID)

	logLine = "random log line"
	podUID, containerID = getContainerIDFromLog(logLine)
	assert.Equal(t, "", podUID)
	assert.Equal(t, "", containerID)
}

func TestPrometheusEnsureSeriesAndCount(t *testing.T) {
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_klog_pod_oomkill",
		Help: "Test metric",
	}, []string{"container_name", "namespace", "pod_uid", "pod_name"})

	kubernetesCounterVec = counter

	labels := map[string]string{
		"io.kubernetes.container.name": "test-container",
		"io.kubernetes.pod.namespace":  "default",
		"io.kubernetes.pod.uid":        "pod123",
		"io.kubernetes.pod.name":       "mypod",
	}

	prometheusEnsureSeries(labels)

	prometheusCount(labels)
	prometheusCount(labels)

	metric, err := kubernetesCounterVec.GetMetricWith(map[string]string{
		"container_name": "test-container",
		"namespace":      "default",
		"pod_uid":        "pod123",
		"pod_name":       "mypod",
	})
	assert.NoError(t, err)

	pb := &dto.Metric{}
	err = metric.Write(pb)
	assert.NoError(t, err)
	assert.Equal(t, 2.0, pb.GetCounter().GetValue())
}

