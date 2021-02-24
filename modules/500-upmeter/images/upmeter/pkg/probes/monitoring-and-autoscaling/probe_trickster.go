package monitoring_and_autoscaling

import (
	"fmt"
	"time"

	"upmeter/pkg/checks"
)

/*
There must be working trickster service
(only if prometheus module enabled)

CHECK:
At least one trickster pod is ready
Period: 10s

CHECK:
Metrics can be retrieved from trickster
	GET /main/api/v1/query?query=vector(1) must return 1
Period: 10s
Timeout: 5s
*/

func NewTricksterPodsProbe() *checks.Probe {
	const (
		period  = 10 * time.Second
		timeout = 5 * time.Second

		namespace     = "d8-monitoring"
		labelSelector = "app=trickster"
	)

	pr := newProbe("trickster", period)

	kubeAccessor := newKubeAccessor(pr)
	checker := newAnyPodReadyChecker(kubeAccessor, timeout, namespace, labelSelector)

	pr.RunFn = RunFn(pr, checker, "pods")

	return pr
}

func NewTricksterAPIProbe() *checks.Probe {
	const (
		period  = 10 * time.Second
		timeout = 5 * time.Second

		namespace = "d8-monitoring"
		service   = "trickster"
		port      = 443
	)

	// trickster.d8-monitoring:443
	endpoint := fmt.Sprintf("https://%s.%s:%d/trickster/main/api/v1/query?query=vector(1)", service, namespace, port)

	pr := newProbe("trickster", period)
	kubeAccessor := newKubeAccessor(pr)
	checker := newPrometheusEndpointChecker(kubeAccessor, endpoint, timeout)

	pr.RunFn = RunFn(pr, checker, "api")

	return pr
}
