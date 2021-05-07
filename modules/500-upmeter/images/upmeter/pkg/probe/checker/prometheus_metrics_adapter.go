package checker

import (
	"net/http"
	"time"

	"github.com/tidwall/gjson"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// MetricsAdapterApiAvailable is a checker constructor and configurator
type MetricsAdapterApiAvailable struct {
	Access   *kubernetes.Access
	Timeout  time.Duration
	Endpoint string
}

func (c MetricsAdapterApiAvailable) Checker() check.Checker {
	verifier := metricsAdapterAPIVerifier{
		endpoint:     c.Endpoint,
		kubeAccessor: c.Access,
	}
	checker := newHTTPChecker(insecureClient, verifier)
	return withTimeout(checker, c.Timeout)
}

type metricsAdapterAPIVerifier struct {
	endpoint     string
	kubeAccessor *kubernetes.Access
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
func (v metricsAdapterAPIVerifier) Verify(body []byte) check.Error {
	value := gjson.Get(string(body), "items.0.value")
	if value.String() == "" {
		return check.ErrFail("got zero value, body = %s", body)
	}
	return nil
}
