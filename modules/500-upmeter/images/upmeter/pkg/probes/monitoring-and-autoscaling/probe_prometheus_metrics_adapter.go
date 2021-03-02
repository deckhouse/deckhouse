package monitoring_and_autoscaling

import (
	"net/http"
	"time"

	"github.com/tidwall/gjson"

	"upmeter/pkg/checks"
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

func NewPrometheusMetricsAdapterPodsProbe() *checks.Probe {
	const (
		period  = 5 * time.Second
		timeout = 5 * time.Second

		namespace     = "d8-monitoring"
		labelSelector = "app=prometheus-metrics-adapter"
	)

	pr := newProbe("prometheus-metrics-adapter", period)
	kubeAccessor := newKubeAccessor(pr)
	checker := newAnyPodReadyChecker(kubeAccessor, timeout, namespace, labelSelector)

	pr.RunFn = RunFn(pr, checker, "pods")

	return pr
}

func NewPrometheusMetricsAdapterAPIProbe() *checks.Probe {
	const (
		period   = 5 * time.Second
		timeout  = 5 * time.Second
		endpoint = "https://kubernetes.default/apis/custom.metrics.k8s.io/v1beta1/namespaces/d8-upmeter/metrics/memory_1m"
	)

	pr := newProbe("prometheus-metrics-adapter", period)
	kubeAccessor := newKubeAccessor(pr)
	checker := newPrometheusMetricsAdapterEndpointChecker(kubeAccessor, endpoint, timeout)

	pr.RunFn = RunFn(pr, checker, "api")

	return pr
}

type metricsAdapterAPIVerifier struct {
	endpoint     string
	kubeAccessor *KubeAccessor
}

func newPrometheusMetricsAdapterEndpointChecker(kubeAccessor *KubeAccessor, endpoint string, timeout time.Duration) Checker {
	verifier := metricsAdapterAPIVerifier{
		endpoint:     endpoint,
		kubeAccessor: kubeAccessor,
	}
	checker := newHTTPChecker(insecureClient, verifier)
	return withTimeout(checker, timeout)

}

func (v metricsAdapterAPIVerifier) Request() *http.Request {
	req, err := newGetRequest(v.endpoint, v.kubeAccessor.ServiceAccountToken())
	if err != nil {
		panic(err)
	}
	return req
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
func (v metricsAdapterAPIVerifier) Verify(body []byte) checks.Error {
	value := gjson.Get(string(body), "items.0.value")
	if value.String() == "" {
		return checks.ErrFail("got zero value, body = %s", body)
	}
	return nil
}
