package monitoring_and_autoscaling

import (
	"fmt"
	"time"

	"upmeter/pkg/checks"
)

/*
There must be working prometheus pods
(only if prometheus module enabled)

CHECK:
At least one prometheus pod is ready
Period: 10s

CHECK:
Metrics can be retrieved from prometheus
	GET /api/v1/query?query=vector(1) must return 1
Period: 10s
Timeout: 5s
*/

func NewPrometheusPodsProbe() *checks.Probe {
	const (
		period  = 10 * time.Second
		timeout = 5 * time.Second

		namespace     = "d8-monitoring"
		labelSelector = "app=prometheus,prometheus=main"
	)

	pr := newProbe("prometheus", period)

	kubeAccessor := newKubeAccessor(pr)
	checker := newAnyPodReadyChecker(kubeAccessor, timeout, namespace, labelSelector)
	pr.RunFn = RunFn(pr, checker, "pods")

	return pr
}

func NewPrometheusAPIProbe() *checks.Probe {
	const (
		period  = 10 * time.Second
		timeout = 5 * time.Second

		namespace = "d8-monitoring"
		service   = "prometheus"
		port      = 9090
	)

	// prometheus.d8-monitoring:9090
	endpoint := fmt.Sprintf("https://%s.%s:%d/api/v1/query?query=vector(1)", service, namespace, port)

	pr := newProbe("prometheus", period)

	kubeAccessor := newKubeAccessor(pr)
	checker := newPrometheusEndpointChecker(kubeAccessor, endpoint, timeout)
	pr.RunFn = RunFn(pr, checker, "api")

	return pr
}
