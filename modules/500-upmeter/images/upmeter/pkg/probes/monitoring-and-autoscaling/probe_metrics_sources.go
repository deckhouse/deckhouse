package monitoring_and_autoscaling

import (
	"time"

	"upmeter/pkg/checks"
)

/*
Metrics sources
only if "monitoring-kubernetes" module enabled

Period: 10s
Timeout: 5s

CHECK:
All node-exporter pods are ready (in daemonset status)

CHECK:
At least one pod of kube-state-metrics is ready
*/

func NewNodeExporterPodsProbe() *checks.Probe {
	var (
		period    = 10 * time.Second
		timeout   = 5 * time.Second
		namespace = "d8-monitoring"
		dsName    = "node-exporter"
	)

	pr := newProbe("metrics-sources", period)

	checker := newAllDaemonsetPodsReadyChecker(
		newKubeAccessor(pr),
		timeout,
		namespace,
		dsName,
	)

	pr.RunFn = RunFn(pr, checker, "node-exporter")

	return pr
}

func NewKubeStateMetricsPodsProbe() *checks.Probe {
	var (
		period        = 10 * time.Second
		timeout       = 5 * time.Second
		namespace     = "d8-monitoring"
		labelSelector = "app=kube-state-metrics"
	)

	pr := newProbe("metrics-sources", period)

	checker := newAnyPodReadyChecker(
		newKubeAccessor(pr),
		timeout,
		namespace,
		labelSelector,
	)

	pr.RunFn = RunFn(pr, checker, "kube-state-metrics")

	return pr
}
