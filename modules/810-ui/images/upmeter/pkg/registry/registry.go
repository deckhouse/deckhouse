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

package registry

import (
	"sort"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/probe"
	"d8.io/upmeter/pkg/probe/calculated"
	"d8.io/upmeter/pkg/set"
)

type Registry struct {
	// runners contains allowed check runners
	runners []*check.Runner

	// calculators contains calculators probes definitions
	calculators []*calculated.Probe
}

func New(runLoader *probe.Loader, calcLoader *calculated.Loader) *Registry {
	return &Registry{
		runners:     runLoader.Load(),
		calculators: calcLoader.Load(),
	}
}

func (r *Registry) Runners() []*check.Runner {
	return r.runners
}

func (r *Registry) Calculators() []*calculated.Probe {
	return r.calculators
}

type ProbeLister interface {
	Groups() []string
	Probes() []check.ProbeRef
}

// NewProbeLister returns the lister of known groups and probes
func NewProbeLister(listers ...ProbeLister) *RegistryProbeLister {
	return &RegistryProbeLister{
		groups: collectGroups(listers...),
		probes: collectProbes(listers...),
	}
}

type RegistryProbeLister struct {
	// groups contain loaded groups
	groups []string

	// probes refs of contain loaded probes
	probes []check.ProbeRef
}

func (pl *RegistryProbeLister) Probes() []check.ProbeRef {
	return pl.probes
}

func (pl *RegistryProbeLister) Groups() []string {
	return pl.groups
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
	refs := []check.ProbeRef{}
	seen := set.New()
	for _, prober := range ls {
		for _, ref := range prober.Probes() {
			if seen.Has(ref.Id()) {
				continue
			}
			seen.Add(ref.Id())

			refs = append(refs, ref)
		}
	}

	sort.Sort(check.ByProbeRef(refs))
	return refs
}
