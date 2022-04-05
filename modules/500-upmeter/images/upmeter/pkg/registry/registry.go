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

package registry

import (
	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/probe"
	"d8.io/upmeter/pkg/probe/calculated"
	"d8.io/upmeter/pkg/set"
)

type ProbeLister interface {
	Groups() []string
	Probes() []check.ProbeRef
}

type Registry struct {
	// groups contain loaded groups
	groups []string

	// probes refs of contain loaded probes
	probes []check.ProbeRef

	// runners contains allowed check runners
	runners []*check.Runner

	// calculators contains calculators probes definitions
	calculators []*calculated.Probe
}

func New(runLoader *probe.Loader, calcLoader *calculated.Loader, disabled []string) *Registry {
	ftr := newSkippedFilter(disabled)
	runners := runLoader.Load(ftr)
	calculators := calcLoader.Load(ftr)

	return &Registry{
		runners:     runners,
		calculators: calculators,

		groups: collectGroups(runLoader, calcLoader),
		probes: collectProbes(runLoader, calcLoader),
	}
}

func (r *Registry) Runners() []*check.Runner {
	return r.runners
}

func (r *Registry) Calculators() []*calculated.Probe {
	return r.calculators
}

func (r *Registry) Probes() []check.ProbeRef {
	return r.probes
}

func (r *Registry) Groups() []string {
	return r.groups
}

func collectGroups(ls ...ProbeLister) []string {
	groups := set.StringSet{}
	for _, grouper := range ls {
		for _, g := range grouper.Groups() {
			groups.Add(g)
		}
	}
	return groups.Slice()
}

func collectProbes(ls ...ProbeLister) []check.ProbeRef {
	probes := []check.ProbeRef{}
	for _, prober := range ls {
		probes = append(probes, prober.Probes()...)
	}
	return probes
}

func newSkippedFilter(disabled []string) filter {
	return filter{refs: set.New(disabled...)}
}

type filter struct {
	refs set.StringSet
}

func (f filter) Enabled(ref check.ProbeRef) bool {
	return !(f.refs.Has(ref.Id()) || f.refs.Has(ref.Group) || f.refs.Has(ref.Group+"/"))
}
