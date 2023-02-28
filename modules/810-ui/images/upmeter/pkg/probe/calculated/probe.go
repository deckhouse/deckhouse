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

package calculated

import (
	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/probe"
	"d8.io/upmeter/pkg/set"
)

func NewLoader(filter probe.Filter, logger *log.Logger) *Loader {
	return &Loader{
		filter: filter,
		logger: logger,
	}
}

type Loader struct {
	filter probe.Filter
	logger *log.Logger

	groups []string
	probes []check.ProbeRef

	configs []config
}

func (l *Loader) collectConfigs() []config {
	if l.configs != nil {
		return l.configs
	}

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

	l.configs = make([]config, 0)
	for _, c := range configs {
		ref := check.ProbeRef{Group: c.group, Probe: c.probe}
		if !l.filter.Enabled(ref) {
			continue
		}
		l.configs = append(l.configs, c)
	}
	return l.configs
}

func (l *Loader) Load() []*Probe {
	probes := make([]*Probe, 0)
	for _, c := range l.collectConfigs() {
		p := c.Probe()
		probes = append(probes, p)
		l.logger.Infof("Registered calculated probe %s", p.ProbeRef().Id())
	}
	return probes
}

func (l *Loader) Groups() []string {
	if l.groups != nil {
		return l.groups
	}

	groups := set.New()
	for _, c := range l.collectConfigs() {
		groups.Add(c.group)
	}

	l.groups = groups.Slice()
	return l.groups
}

func (l *Loader) Probes() []check.ProbeRef {
	if l.probes != nil {
		return l.probes
	}

	l.probes = make([]check.ProbeRef, 0)
	for _, c := range l.collectConfigs() {
		l.probes = append(l.probes, *c.Probe().ref)
	}

	return l.probes
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
