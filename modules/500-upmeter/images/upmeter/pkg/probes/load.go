package probes

import (
	"upmeter/pkg/checks"
	"upmeter/pkg/probes/control-plane"
	"upmeter/pkg/probes/monitoring-and-autoscaling"
	"upmeter/pkg/probes/synthetic"
)

// Load creates instances of available Probes
func Load() []*checks.Probe {
	res := make([]*checks.Probe, 0)
	res = append(res, control_plane.LoadGroup()...)
	res = append(res, synthetic.LoadGroup()...)
	res = append(res, monitoring_and_autoscaling.LoadGroup()...)
	return res
}
