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

package probe

import (
	"io/ioutil"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
)

func Test_NewProbeFilter(t *testing.T) {
	filter := NewProbeFilter([]string{"full/ref", "notslashed", "slashed/"})

	// exact matches
	assert.False(t, filter.Enabled(check.ProbeRef{Group: "full", Probe: "ref"}))
	assert.False(t, filter.Enabled(check.ProbeRef{Group: "notslashed", Probe: ""}))
	assert.False(t, filter.Enabled(check.ProbeRef{Group: "slashed", Probe: ""}))

	// probes under group notations
	assert.False(t, filter.Enabled(check.ProbeRef{Group: "notslashed", Probe: "probe"}))
	assert.False(t, filter.Enabled(check.ProbeRef{Group: "slashed", Probe: "probe"}))

	// not mentioned
	assert.True(t, filter.Enabled(check.ProbeRef{Group: "something", Probe: ""}))
	assert.True(t, filter.Enabled(check.ProbeRef{Group: "something", Probe: "else"}))
}

func TestLoader_Groups(t *testing.T) {
	unfiltered := &Loader{
		filter: NewProbeFilter([]string{}),
		access: kubernetes.FakeAccessor(),
		logger: newDummyLogger().Logger,
	}

	allGroups := []string{
		"control-plane",
		"deckhouse",
		"extensions",
		"load-balancing",
		"monitoring-and-autoscaling",
		"synthetic",
	}
	assert.Equal(t, allGroups, unfiltered.Groups())

	filtered := &Loader{
		filter: NewProbeFilter([]string{"deckhouse", "extensions/"}),
		access: kubernetes.FakeAccessor(),
		logger: newDummyLogger().Logger,
	}

	notAllGroups := []string{
		"control-plane",
		"load-balancing",
		"monitoring-and-autoscaling",
		"synthetic",
	}
	assert.Equal(t, notAllGroups, filtered.Groups())
}

func TestLoader_Probes(t *testing.T) {
	unfiltered := &Loader{
		filter: NewProbeFilter([]string{}),
		access: kubernetes.FakeAccessor(),
		logger: newDummyLogger().Logger,
	}

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

	assert.Equal(t, allProbesSorted, unfiltered.Probes())

	filtered := NewLoader(
		NewProbeFilter([]string{"deckhouse", "extensions/", "load-balancing/metallb", "nodegroups/spot"}),
		kubernetes.FakeAccessor(),
		nil, // nodeLister
		DynamicConfig{
			IngressNginxControllers: []string{"main", "main-w-pp"},
			NodeGroups:              []string{"system", "frontend", "worker", "spot"},
		},
		checker.NoopDoer{},
		newDummyLogger().Logger,
	)

	filteredProbesSorted := []check.ProbeRef{
		{Group: "control-plane", Probe: "apiserver"},
		{Group: "control-plane", Probe: "basic-functionality"},
		{Group: "control-plane", Probe: "cert-manager"},
		{Group: "control-plane", Probe: "controller-manager"},
		{Group: "control-plane", Probe: "namespace"},
		{Group: "control-plane", Probe: "scheduler"},
		// --    deckhouse/...
		// --    extensions/...
		{Group: "load-balancing", Probe: "load-balancer-configuration"},
		// --    load-balancing/metallb
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

	assert.Equal(t, filteredProbesSorted, filtered.Probes())
}

func newDummyLogger() *log.Entry {
	logger := log.New()

	// logger.Level = log.DebugLevel
	logger.SetOutput(ioutil.Discard)

	return log.NewEntry(logger)
}
