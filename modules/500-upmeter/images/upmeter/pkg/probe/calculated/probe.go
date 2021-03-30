package calculated

import (
	"fmt"

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

func (p *Probe) Id() string {
	return p.ref.Id()
}

// Calc merges episodes of multiple probes into a new episode.
//
// The algorithm maximizes fail, then unknown, then nodata, and then fills success with time left.
func (p *Probe) Calc(episodes map[string]*check.DowntimeEpisode, stepSeconds int64) (*check.DowntimeEpisode, error) {
	var result *check.DowntimeEpisode

	for _, id := range p.mergeIds {
		ep, ok := episodes[id]
		if !ok {
			return nil, fmt.Errorf("no episode for probe id=%s", id)
		}

		if result == nil {
			result = ep
			continue
		}

		worsen(result, ep, stepSeconds)
	}

	result.ProbeRef = *p.ref
	return result, nil
}
