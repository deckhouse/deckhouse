package monitoring_and_autoscaling

import (
	"fmt"
	"time"

	"upmeter/pkg/checks"
	control_plane "upmeter/pkg/probes/control-plane"
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

func NewTricksterProbe() *checks.Probe {
	const (
		probePeriod  = 10 * time.Second
		probeTimeout = 5 * time.Second

		namespace     = "d8-monitoring"
		labelSelector = "app=trickster"
		service       = "trickster"
		port          = 443
	)

	// trickster.d8-monitoring:443
	endpoint := fmt.Sprintf("https://%s.%s:%d/trickster/main/api/v1/query?query=vector(1)", service, namespace, port)
	checker := promChecker{
		namespace:     namespace,
		labelSelector: labelSelector,
		endpoint:      endpoint,
		client:        insecureClient,
	}

	nsProbeRef := checks.ProbeRef{
		Group: groupName,
		Probe: "trickster",
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
