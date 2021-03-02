package manager

import (
	"github.com/flant/shell-operator/pkg/kube"

	"upmeter/pkg/checks"
	"upmeter/pkg/probes"
	"upmeter/pkg/probes/calculated"
)

type ProbeManager struct {
	probes     []*checks.Probe
	calcProbes []*calculated.Probe
}

func NewProbeManager() *ProbeManager {
	return &ProbeManager{}
}

func (m *ProbeManager) Init() {
	m.probes = filterDisabledProbes(probes.Load())
	m.calcProbes = filterDisabledCalcProbes(calculated.Load())
}

func (m *ProbeManager) Probes() []*checks.Probe {
	return m.probes
}

func (m *ProbeManager) Calculators() []*calculated.Probe {
	return m.calcProbes
}

func (m *ProbeManager) InitProbes(ch chan checks.Result, client kube.KubernetesClient, token string) {
	for _, p := range m.probes {
		_ = p.Init()
		p.WithResultChan(ch)
		p.WithKubernetesClient(client)
		p.WithServiceAccountToken(token)
	}
}

func filterDisabledProbes(ps []*checks.Probe) []*checks.Probe {
	var newList = make([]*checks.Probe, 0)

	for _, p := range ps {
		if checks.IsProbeEnabled(p.Id()) {
			newList = append(newList, p)
		}
	}

	return newList
}

func filterDisabledCalcProbes(ps []*calculated.Probe) []*calculated.Probe {
	var newList = make([]*calculated.Probe, 0)

	for _, p := range ps {
		if checks.IsProbeEnabled(p.Id()) {
			newList = append(newList, p)
		}
	}

	return newList
}
