package monitoring_and_autoscaling

import (
	"time"

	"github.com/tidwall/gjson"

	"upmeter/pkg/checks"
	"upmeter/pkg/probes/control-plane"
)

/*
There must be working prometheus-metrics-adapter service
(only if prometheus-metrics-adapter module enabled)

CHECK:
At least one prometheus-metrics-adapter pod is ready
Period: 5s

CHECK:
Metrics prometheus-metrics-adapter return non-zero value
	GET /apis/custom.metrics.k8s.io/v1beta1/namespaces/d8-upmeter/metrics/memory_1m must return non-zero value
Period: 10s
Timeout: 5s
*/

func NewPromMetricsAdapterProbe() *checks.Probe {
	const (
		probePeriod  = 5 * time.Second
		probeTimeout = 5 * time.Second

		namespace     = "d8-monitoring"
		labelSelector = "app=prometheus-metrics-adapter"
	)

	endpoint := "https://kubernetes.default/apis/custom.metrics.k8s.io/v1beta1/namespaces/d8-upmeter/metrics/memory_1m"
	checker := metricsAdapterChecker{
		namespace:     namespace,
		labelSelector: labelSelector,
		endpoint:      endpoint,
	}

	nsProbeRef := checks.ProbeRef{
		Group: groupName,
		Probe: "prometheus-metrics-adapter",
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

type metricsAdapterChecker struct {
	commonPodChecker

	namespace     string
	labelSelector string
	endpoint      string
}

func (c metricsAdapterChecker) Endpoint() string {
	return c.endpoint
}

/*
Expecting this with non-zero value

{
  "kind": "MetricValueList",
  "apiVersion": "custom.metrics.k8s.io/v1beta1",
  "metadata": {
    "selfLink": "/apis/custom.metrics.k8s.io/v1beta1/namespaces/d8-upmeter/metrics/memory_1m"
  },
  "items": [
    {
      "describedObject": {
        "kind": "Namespace",
        "name": "d8-upmeter",
        "apiVersion": "/v1"
      },
      "metricName": "memory_1m",
      "timestamp": "2021-02-16T08:05:18Z",
      "value": "73252864"                               <- we check this
    }
  ]
}
*/
// Verify checks that value is non-zero
func (c metricsAdapterChecker) Verify(body []byte) checks.Error {
	value := gjson.Get(string(body), "items.0.value")
	if value.Float() == 0 {
		return checks.ErrFail("metrics adapter responded with zero value, body = %s", body)
	}
	return nil
}
