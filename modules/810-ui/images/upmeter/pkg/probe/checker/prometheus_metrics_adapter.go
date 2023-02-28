/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	Access   kubernetes.Access
	Timeout  time.Duration
	Endpoint string
}

func (c MetricsAdapterApiAvailable) Checker() check.Checker {
	verifier := metricsAdapterAPIVerifier{
		endpoint:     c.Endpoint,
		kubeAccessor: c.Access,
	}
	checker := newHTTPChecker(newInsecureClient(3*c.Timeout), verifier)
	return withTimeout(checker, c.Timeout)
}

type metricsAdapterAPIVerifier struct {
	endpoint     string
	kubeAccessor kubernetes.Access
}

func (v metricsAdapterAPIVerifier) Request() *http.Request {
	req, err := newGetRequest(v.endpoint, v.kubeAccessor.ServiceAccountToken(), v.kubeAccessor.UserAgent())
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
	      "timestamp": "2023-02-16T08:05:18Z",
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
