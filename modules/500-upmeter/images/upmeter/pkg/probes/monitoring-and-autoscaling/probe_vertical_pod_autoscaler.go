package monitoring_and_autoscaling

import (
	"time"

	"upmeter/pkg/checks"
)

/*
There must be working VPA service
(only if vertical-pod-autoscaler module enabled)

Period: 10s
Timeout: 5s

CHECK:
At least one vpa-updater pod is ready

CHECK:
At least one vpa-recommender pod is ready

CHECK:
At least one vpa-admission-controller pod is ready

*/

func NewVPAAdmissionProbe() *checks.Probe {
	return newVPAProbe("vpa-admission-controller")
}

func NewVPARecommenderProbe() *checks.Probe {
	return newVPAProbe("vpa-recommender")
}

func NewVPAUpdaterProbe() *checks.Probe {
	return newVPAProbe("vpa-updater")
}

func newVPAProbe(name string) *checks.Probe {
	var (
		period        = 10 * time.Second
		timeout       = 5 * time.Second
		namespace     = "kube-system"
		labelSelector = "app=" + name
	)

	pr := newProbe("vertical-pod-autoscaler", period)

	checker := newAnyPodReadyChecker(
		newKubeAccessor(pr),
		timeout,
		namespace,
		labelSelector,
	)

	pr.RunFn = RunFn(pr, checker, name)

	return pr
}
