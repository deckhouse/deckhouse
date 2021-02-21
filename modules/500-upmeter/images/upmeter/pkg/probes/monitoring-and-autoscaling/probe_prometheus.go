package monitoring_and_autoscaling

import (
	"fmt"
	"time"

	"upmeter/pkg/checks"
	control_plane "upmeter/pkg/probes/control-plane"
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

func NewPrometheusProbe() *checks.Probe {
	const (
		probePeriod  = 10 * time.Second
		probeTimeout = 5 * time.Second

		namespace     = "d8-monitoring"
		labelSelector = "app=prometheus,prometheus=main"
		service       = "prometheus"
		port          = 9090
	)

	// prometheus.d8-monitoring:9090
	endpoint := fmt.Sprintf("https://%s.%s:%d/api/v1/query?query=vector(1)", service, namespace, port)
	checker := promChecker{
		namespace:     namespace,
		labelSelector: labelSelector,
		endpoint:      endpoint,
		client:        insecureClient,
	}

	nsProbeRef := checks.ProbeRef{
		Group: groupName,
		Probe: "prometheus",
	}

	pr := &checks.Probe{
		Period: probePeriod,
		Ref:    &nsProbeRef,
	}

	pipeline := NewPodCheckPipeline(pr, probeTimeout, checker)

	pr.RunFn = func() {
		// Set Unknown result if API server is unavailable
		if !control_plane.CheckApiAvailable(pr) {
			return
		}
		pipeline.Go()
	}

	return pr
}
