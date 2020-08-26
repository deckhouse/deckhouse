package manager

import (
	"github.com/flant/shell-operator/pkg/kube"
	"os"
	"strings"
	"upmeter/pkg/probe/types"
	"upmeter/pkg/probers"
)

type ProbeManager struct {
	ProberList []types.Prober
}

func NewProbeManager() *ProbeManager {
	return &ProbeManager{}
}

func (m *ProbeManager) Init() {
	proberList := probers.Load()

	enabled := os.Getenv("UPMETER_ENABLED_PROBES")
	if enabled != "" {
		enabledList := strings.Split(enabled, ",")
		newList := []types.Prober{}
		for _, prober := range proberList {
			for _, enablePrefix := range enabledList {
				if strings.HasPrefix(prober.ProbeId(), enablePrefix) {
					newList = append(newList, prober)
					break
				}
			}
		}
		proberList = newList
	}

	disabled := os.Getenv("UPMETER_DISABLED_PROBES")
	if disabled != "" {
		disabledList := strings.Split(enabled, ",")
		newList := []types.Prober{}
		for _, prober := range proberList {
			isEnabled := true
			for _, disablePrefix := range disabledList {
				if strings.HasPrefix(prober.ProbeId(), disablePrefix) {
					isEnabled = false
					break
				}
			}
			if isEnabled {
				newList = append(newList, prober)
			}
		}
		proberList = newList
	}

	m.ProberList = proberList
}

func (m *ProbeManager) InitProbes(ch chan types.ProbeResult, client kube.KubernetesClient) {
	for _, p := range m.ProberList {
		_ = p.Init()
		p.WithResultChan(ch)
		p.WithKubernetesClient(client)
	}
}

func (m *ProbeManager) Probers() []types.Prober {
	return m.ProberList
}
