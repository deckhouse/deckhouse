package manager

import (
	"github.com/flant/shell-operator/pkg/kube"

	"upmeter/pkg/probe/types"
	"upmeter/pkg/probers"
)

type ProbeManager struct {
	probers []types.Prober
}

func (m *ProbeManager) Probers() []types.Prober {
	return m.probers
}

func NewProbeManager() *ProbeManager {
	return &ProbeManager{}
}

func (m *ProbeManager) Init() {
	m.probers = FilterDisabledProbesFromProbers(probers.Load())
}

func (m *ProbeManager) InitProbes(ch chan types.ProbeResult, client kube.KubernetesClient) {
	for _, p := range m.probers {
		_ = p.Init()
		p.WithResultChan(ch)
		p.WithKubernetesClient(client)
	}
}

func FilterDisabledProbesFromProbers(probers []types.Prober) []types.Prober {
	var newList = make([]types.Prober, 0)

	for _, prober := range probers {
		if types.IsProbeEnabled(prober.ProbeId()) {
			newList = append(newList, prober)
		}
	}

	return newList
}
