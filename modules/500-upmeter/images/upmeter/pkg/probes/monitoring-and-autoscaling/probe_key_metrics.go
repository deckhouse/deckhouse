package monitoring_and_autoscaling

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/tidwall/gjson"

	"upmeter/pkg/checks"
)

/*
Key metrics are present ("key-metrics-present")
Works only if "monitoring-kubernetes" module enabled

CHECK:
Prometheus has metrics of kube-state-metrics
Period: 15s
Timeout: 5s

CHECK:
Prometheus has metrics of node-exporter
Period: 15s
Timeout: 5s

CHECK:
Prometheus has metrics of kubelet
Period: 15s
Timeout: 5s

*/

func NewKubeStateMetricsMetricsProbe() *checks.Probe {
	return newMetricsPresenceProbe("kube-state-metrics", "kube_state_metrics_list_total")
}

func NewNodeExporterMetricsProbe() *checks.Probe {
	return newMetricsPresenceProbe("node-exporter", "node_exporter_build_info")
}

func NewKubeletMetricsProbe() *checks.Probe {
	return newMetricsPresenceProbe("kubelet", "kubelet_node_name")
}

func newMetricsPresenceProbe(checkName, metricName string) *checks.Probe {
	const (
		period  = 15 * time.Second
		timeout = 5 * time.Second

		namespace = "d8-monitoring"
		service   = "prometheus"
		port      = 9090
	)

	pr := newProbe("key-metrics-present", period)
	kubeAccessor := newKubeAccessor(pr)

	baseUrl := fmt.Sprintf("https://%s.%s:%d/api/v1/query", service, namespace, port)
	checker := metricsPresenceChecker(kubeAccessor, timeout, baseUrl, metricName)

	pr.RunFn = RunFn(pr, checker, checkName)

	return pr
}

func metricsPresenceChecker(kubeAccessor *KubeAccessor, timeout time.Duration, baseUrl, metricName string) Checker {
	verifier := &metricPresenceVerifier{
		endpoint:     baseEndpoint(baseUrl, metricName),
		kubeAccessor: kubeAccessor,
	}

	checker := newHTTPChecker(insecureClient, verifier)

	return withTimeout(checker, timeout)
}

type metricPresenceVerifier struct {
	endpoint     string
	kubeAccessor *KubeAccessor
}

func (v *metricPresenceVerifier) Request() *http.Request {
	req, err := newGetRequest(v.endpoint, v.kubeAccessor.ServiceAccountToken())
	if err != nil {
		panic(err)
	}
	return req

}

/*
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [                 <- we check that the array is not empty
      {
        "metric": {},
        "value": [
          1614179019.102,
          "24"                  <- mut not be zero
        ]
      }
    ]
  }
}

*/
func (v *metricPresenceVerifier) Verify(body []byte) checks.Error {
	resultPath := "data.result"
	result := gjson.Get(string(body), resultPath)

	if !result.IsArray() {
		return checks.ErrFail("cannot parse path %q in prometheus response %q", resultPath, body)
	}

	if len(result.Array()) == 0 {
		return checks.ErrFail("no metrics in prometheus response (did not count)")
	}

	countPath := "data.result.0.value.1"
	count := gjson.Get(string(body), countPath)
	if count.String() == "0" {
		return checks.ErrFail("no metrics in prometheus response (zero count)")
	}

	return nil
}

func baseEndpoint(baseUrl, metricName string) string {
	endpoint, err := url.Parse(baseUrl)
	if err != nil {
		panic(fmt.Errorf("cannot parse baseUrl: %v", err))
	}

	query := make(url.Values)
	// e.g. ?query=count(kubelet_node_name)
	query.Set("query", fmt.Sprintf("count(%s)", metricName))
	endpoint.RawQuery = query.Encode()

	return endpoint.String()
}
