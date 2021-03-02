package monitoring_and_autoscaling

import (
	"time"

	"upmeter/pkg/checks"
)

const groupName = "monitoring-and-autoscaling"

func RunFn(probe *checks.Probe, checker Checker, checkName string) func() {
	return func() {
		checkStatus := checks.StatusSuccess
		err := checker.Check()
		if err != nil {
			probe.LogEntry().Errorf(err.Error())
			checkStatus = err.Status()
		}
		probe.ResultCh <- probe.CheckResult(checkName, checkStatus)
	}
}

func newProbe(name string, period time.Duration) *checks.Probe {
	ref := &checks.ProbeRef{
		Group: groupName,
		Probe: name,
	}
	return &checks.Probe{
		Period: period,
		Ref:    ref,
	}
}

func LoadGroup() []*checks.Probe {
	return []*checks.Probe{
		// Prometheus
		NewPrometheusPodsProbe(),
		NewPrometheusAPIProbe(),

		// Trickster
		NewTricksterPodsProbe(),
		NewTricksterAPIProbe(),

		// Prometheus Metrics Adapter
		NewPrometheusMetricsAdapterPodsProbe(),
		NewPrometheusMetricsAdapterAPIProbe(),

		// Vertical Pod Autoscaler
		NewVPAAdmissionProbe(),
		NewVPARecommenderProbe(),
		NewVPAUpdaterProbe(),

		// Metrics sources
		NewNodeExporterPodsProbe(),
		NewKubeStateMetricsPodsProbe(),

		// Key metrics presence
		NewKubeStateMetricsMetricsProbe(),
		NewNodeExporterMetricsProbe(),
		NewKubeletMetricsProbe(),
	}
}
