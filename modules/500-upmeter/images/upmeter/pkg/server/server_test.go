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

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"d8.io/upmeter/pkg/check"
)

// Test how all the known probes and groups are presented
func Test_newProbeLister(t *testing.T) {
	pl := newProbeLister([]string{})

	allProbesSorted := []check.ProbeRef{
		{Group: "control-plane", Probe: "access"},
		{Group: "control-plane", Probe: "basic-functionality"},
		{Group: "control-plane", Probe: "controller-manager"},
		{Group: "control-plane", Probe: "namespace"},
		{Group: "control-plane", Probe: "scheduler"},
		{Group: "deckhouse", Probe: "cluster-configuration"},
		{Group: "load-balancing", Probe: "load-balancer-configuration"},
		{Group: "load-balancing", Probe: "metallb"},
		{Group: "monitoring-and-autoscaling", Probe: "horizontal-pod-autoscaler"},
		{Group: "monitoring-and-autoscaling", Probe: "key-metrics-present"},
		{Group: "monitoring-and-autoscaling", Probe: "metrics-sources"},
		{Group: "monitoring-and-autoscaling", Probe: "prometheus"},
		{Group: "monitoring-and-autoscaling", Probe: "prometheus-metrics-adapter"},
		{Group: "monitoring-and-autoscaling", Probe: "trickster"},
		{Group: "monitoring-and-autoscaling", Probe: "vertical-pod-autoscaler"},
		{Group: "scaling", Probe: "cluster-autoscaler"},
		{Group: "scaling", Probe: "cluster-scaling"},
		{Group: "synthetic", Probe: "access"},
		{Group: "synthetic", Probe: "dns"},
		{Group: "synthetic", Probe: "neighbor"},
		{Group: "synthetic", Probe: "neighbor-via-service"},
	}

	allGroupsSorted := []string{
		"control-plane",
		"deckhouse",
		"load-balancing",
		"monitoring-and-autoscaling",
		"scaling",
		"synthetic",
	}

	assert.Equal(t, allProbesSorted, pl.Probes())
	assert.Equal(t, allGroupsSorted, pl.Groups())
}
