/*
Copyright 2021 Flant CJSC

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

package manager

import (
	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe"
	"d8.io/upmeter/pkg/probe/calculated"
)

type Manager struct {
	runners     []*check.Runner
	calculators []*calculated.Probe
}

func New(access kubernetes.Access) *Manager {
	m := &Manager{}
	m.runners = filterRunners(probe.Load(access))
	m.calculators = filterCalculators(calculated.Load())
	return m
}

func (m *Manager) Runners() []*check.Runner {
	return m.runners
}

func (m *Manager) Calculators() []*calculated.Probe {
	return m.calculators
}

func filterRunners(ps []*check.Runner) []*check.Runner {
	newList := make([]*check.Runner, 0)

	for _, p := range ps {
		if check.IsProbeEnabled(p.ProbeRef().Id()) {
			newList = append(newList, p)
		}
	}

	return newList
}

func filterCalculators(ps []*calculated.Probe) []*calculated.Probe {
	newList := make([]*calculated.Probe, 0)

	for _, p := range ps {
		if check.IsProbeEnabled(p.ProbeRef().Id()) {
			newList = append(newList, p)
		}
	}

	return newList
}
