package monitoring_and_autoscaling

import (
	"upmeter/pkg/checks"
)

const groupName = "monitoring-and-autoscaling"

func LoadGroup() []*checks.Probe {
	return []*checks.Probe{
		NewPrometheusProbe(),
		NewTricksterProbe(),
		NewPromMetricsAdapterProbe(),
	}
}
