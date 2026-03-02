/*
Copyright 2021 Flant JSC

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
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/tidwall/gjson"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// PrometheusAPIAvailable is a checker constructor and configurator
type PrometheusAPIAvailable struct {
	Access   kubernetes.Access
	Timeout  time.Duration
	Endpoint string
}

func (c PrometheusAPIAvailable) Checker() check.Checker {
	verifier := prometheusAPIVerifier{
		endpoint: c.Endpoint,
		access:   c.Access,
	}
	checker := newHTTPChecker(newInsecureClient(3*c.Timeout), verifier)
	return withTimeout(checker, c.Timeout)
}

type prometheusAPIVerifier struct {
	endpoint string
	access   kubernetes.Access
}

func (v prometheusAPIVerifier) Request() *http.Request {
	req, err := newGetRequest(v.endpoint, v.access.ServiceAccountToken(), v.access.UserAgent())
	if err != nil {
		panic(err)
	}
	return req
}

/*
Expected JSON

	{
	  "status": "success",
	  "data": {
	    "resultType": "vector",
	    "result": [
	      {
	        "metric": {},
	        "value": [
	          1613143228.991,
	          "1"               <- we check this
	        ]
	      }
	    ]
	  }
	}
*/
func (v prometheusAPIVerifier) Verify(body []byte) check.Error {
	path := "data.result.0.value.1"
	value := gjson.Get(string(body), path)

	if value.String() != "1" {
		return check.ErrFail(`cannot find "1" in path %q prometheus data %q`, path, body)
	}

	return nil
}

// MetricPresentInPrometheus is a checker constructor and configurator
type MetricPresentInPrometheus struct {
	Access   kubernetes.Access
	Timeout  time.Duration
	Endpoint string
	Metric   string
}

func (c MetricPresentInPrometheus) Checker() check.Checker {
	verifier := &metricPresenceVerifier{
		access:   c.Access,
		endpoint: addMetricQuery(c.Endpoint, c.Metric),
	}
	checker := newHTTPChecker(newInsecureClient(3*c.Timeout), verifier)
	return withTimeout(checker, c.Timeout)
}

type metricPresenceVerifier struct {
	access   kubernetes.Access
	endpoint string
}

func (v *metricPresenceVerifier) Request() *http.Request {
	req, err := newGetRequest(v.endpoint, v.access.ServiceAccountToken(), v.access.UserAgent())
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
	    "result": [                 <- array must not be empty
	      {
	        "metric": {},
	        "value": [
	          1614179019.102,
	          "24"                  <- string number must not be zero
	        ]
	      }
	    ]
	  }
	}
*/
func (v *metricPresenceVerifier) Verify(body []byte) check.Error {
	present, err := isMetricPresentInPrometheusResponse(body)
	if err != nil {
		return check.ErrFail("cannot parse prometheus response: %v", err)
	}
	if !present {
		return check.ErrFail("no metrics in prometheus response")
	}
	return nil
}

func isMetricPresentInPrometheusResponse(body []byte) (bool, error) {
	resultPath := "data.result"
	result := gjson.GetBytes(body, resultPath)
	if !result.IsArray() {
		return false, fmt.Errorf("cannot parse path %q in Prometheus response", resultPath)
	}

	if len(result.Array()) == 0 {
		return false, nil
	}

	countPath := "data.result.0.value.1"
	count := gjson.GetBytes(body, countPath)
	if !count.Exists() {
		return false, nil
	}

	return count.String() != "0", nil
}

func addMetricQuery(baseURL, metricName string) string {
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		panic(fmt.Errorf("cannot parse baseURL: %v", err))
	}

	query := make(url.Values)
	// e.g. ?query=count(kubelet_node_name)
	query.Set("query", fmt.Sprintf("count(%s)", metricName))
	endpoint.RawQuery = query.Encode()

	return endpoint.String()
}
