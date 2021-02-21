package manager

import (
	"github.com/flant/shell-operator/pkg/kube"

	"upmeter/pkg/checks"
	"upmeter/pkg/probes"
)

type ProbeManager struct {
	probes []*checks.Probe
}

func (m *ProbeManager) Probes() []*checks.Probe {
	return m.probes
}

func NewProbeManager() *ProbeManager {
	return &ProbeManager{}
}

func (m *ProbeManager) Init() {
	m.probes = FilterDisabled(probes.Load())
}

func (m *ProbeManager) InitProbes(ch chan checks.Result, client kube.KubernetesClient, token string) {
	for _, p := range m.probes {
		_ = p.Init()
		p.WithResultChan(ch)
		p.WithKubernetesClient(client)
		p.WithServiceAccountToken(token)
	}
}

func FilterDisabled(ps []*checks.Probe) []*checks.Probe {
	var newList = make([]*checks.Probe, 0)

	for _, p := range ps {
		if checks.IsProbeEnabled(p.Id()) {
			newList = append(newList, p)
		}
	}

	return newList
}
