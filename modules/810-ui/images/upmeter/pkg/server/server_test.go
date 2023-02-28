/*
Copyright 2023 Flant JSC

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

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"d8.io/upmeter/pkg/check"
)

// Test how all the known probes and groups are presented
func Test_newProbeLister(t *testing.T) {
	pl := newProbeLister([]string{}, &DynamicProbesConfig{})

	allProbesSorted := []check.ProbeRef{
		{Group: "control-plane", Probe: "apiserver"},
		{Group: "control-plane", Probe: "basic-functionality"},
		{Group: "control-plane", Probe: "cert-manager"},
		{Group: "control-plane", Probe: "controller-manager"},
		{Group: "control-plane", Probe: "namespace"},
		{Group: "control-plane", Probe: "scheduler"},
		{Group: "deckhouse", Probe: "cluster-configuration"},
		{Group: "extensions", Probe: "cluster-autoscaler"},
		{Group: "extensions", Probe: "cluster-scaling"},
		{Group: "extensions", Probe: "dashboard"},
		{Group: "extensions", Probe: "dex"},
		{Group: "extensions", Probe: "grafana"},
		{Group: "extensions", Probe: "openvpn"},
		{Group: "extensions", Probe: "prometheus-longterm"},
		{Group: "load-balancing", Probe: "load-balancer-configuration"},
		{Group: "load-balancing", Probe: "metallb"},
		{Group: "monitoring-and-autoscaling", Probe: "horizontal-pod-autoscaler"},
		{Group: "monitoring-and-autoscaling", Probe: "key-metrics-present"},
		{Group: "monitoring-and-autoscaling", Probe: "metrics-sources"},
		{Group: "monitoring-and-autoscaling", Probe: "prometheus"},
		{Group: "monitoring-and-autoscaling", Probe: "prometheus-metrics-adapter"},
		{Group: "monitoring-and-autoscaling", Probe: "trickster"},
		{Group: "monitoring-and-autoscaling", Probe: "vertical-pod-autoscaler"},
		{Group: "synthetic", Probe: "access"},
		{Group: "synthetic", Probe: "dns"},
		{Group: "synthetic", Probe: "neighbor"},
		{Group: "synthetic", Probe: "neighbor-via-service"},
	}

	allGroupsSorted := []string{
		"control-plane",
		"deckhouse",
		"extensions",
		"load-balancing",
		"monitoring-and-autoscaling",
		"synthetic",
	}

	assert.Equal(t, allProbesSorted, pl.Probes())
	assert.Equal(t, allGroupsSorted, pl.Groups())
}

// Test how all the known probes and groups are presented including dynamic probes
func Test_newProbeLister_with_dynamic(t *testing.T) {
	pl := newProbeLister([]string{}, &DynamicProbesConfig{
		IngressControllers: []string{"main", "main-w-pp"},
		NodeGroups:         []string{"system", "frontend", "worker"},
	})

	allProbesSorted := []check.ProbeRef{
		{Group: "control-plane", Probe: "apiserver"},
		{Group: "control-plane", Probe: "basic-functionality"},
		{Group: "control-plane", Probe: "cert-manager"},
		{Group: "control-plane", Probe: "controller-manager"},
		{Group: "control-plane", Probe: "namespace"},
		{Group: "control-plane", Probe: "scheduler"},
		{Group: "deckhouse", Probe: "cluster-configuration"},
		{Group: "extensions", Probe: "cluster-autoscaler"},
		{Group: "extensions", Probe: "cluster-scaling"},
		{Group: "extensions", Probe: "dashboard"},
		{Group: "extensions", Probe: "dex"},
		{Group: "extensions", Probe: "grafana"},
		{Group: "extensions", Probe: "openvpn"},
		{Group: "extensions", Probe: "prometheus-longterm"},
		{Group: "load-balancing", Probe: "load-balancer-configuration"},
		{Group: "load-balancing", Probe: "metallb"},
		{Group: "monitoring-and-autoscaling", Probe: "horizontal-pod-autoscaler"},
		{Group: "monitoring-and-autoscaling", Probe: "key-metrics-present"},
		{Group: "monitoring-and-autoscaling", Probe: "metrics-sources"},
		{Group: "monitoring-and-autoscaling", Probe: "prometheus"},
		{Group: "monitoring-and-autoscaling", Probe: "prometheus-metrics-adapter"},
		{Group: "monitoring-and-autoscaling", Probe: "trickster"},
		{Group: "monitoring-and-autoscaling", Probe: "vertical-pod-autoscaler"},
		{Group: "nginx", Probe: "main"},
		{Group: "nginx", Probe: "main-w-pp"},
		{Group: "nodegroups", Probe: "frontend"},
		{Group: "nodegroups", Probe: "system"},
		{Group: "nodegroups", Probe: "worker"},
		{Group: "synthetic", Probe: "access"},
		{Group: "synthetic", Probe: "dns"},
		{Group: "synthetic", Probe: "neighbor"},
		{Group: "synthetic", Probe: "neighbor-via-service"},
	}

	allGroupsSorted := []string{
		"control-plane",
		"deckhouse",
		"extensions",
		"load-balancing",
		"monitoring-and-autoscaling",
		"nginx",
		"nodegroups",
		"synthetic",
	}

	assert.Equal(t, allProbesSorted, pl.Probes())
	assert.Equal(t, allGroupsSorted, pl.Groups())
}
