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

func New(access *kubernetes.Access) *Manager {
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
