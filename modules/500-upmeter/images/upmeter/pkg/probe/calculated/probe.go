package calculated

import (
	"upmeter/pkg/check"
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

// Probe combines check.DowntimeEpisode for included probe IDs.
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
