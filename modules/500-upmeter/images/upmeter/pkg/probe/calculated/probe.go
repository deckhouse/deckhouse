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

package calculated

import (
	"d8.io/upmeter/pkg/check"
)

func Load() []*Probe {
	configs := []config{
		{
			group: "monitoring-and-autoscaling",
			probe: "horizontal-pod-autoscaler",
			mergeIds: []string{
				"monitoring-and-autoscaling/prometheus-metrics-adapter",
				"control-plane/controller-manager",
			},
		},
	}

	probes := make([]*Probe, 0)
	for _, c := range configs {
		probes = append(probes, c.Probe())
	}
	return probes
}

// config is a convenient wrapper to create calculated probe
type config struct {
	group    string
	probe    string
	mergeIds []string
}

func (c config) Probe() *Probe {
	ref := &check.ProbeRef{
		Group: c.group,
		Probe: c.probe,
	}
	return &Probe{ref, c.mergeIds}
}

// Probe combines check.Episode for included probe IDs.
type Probe struct {
	ref      *check.ProbeRef
	mergeIds []string
}

func (p *Probe) ProbeRef() check.ProbeRef {
	return *p.ref
}

func (p *Probe) MergeIds() []string {
	ids := make([]string, len(p.mergeIds))
	copy(ids, p.mergeIds)
	return ids
}
