package monitoring_and_autoscaling

import (
	"net/http"
	"time"

	"github.com/tidwall/gjson"

	"upmeter/pkg/checks"
)

func newPrometheusEndpointChecker(kubeAccessor *KubeAccessor, endpoint string, timeout time.Duration) Checker {
	verifier := prometheusAPIVerifier{
		endpoint:     endpoint,
		kubeAccessor: kubeAccessor,
	}
	checker := newHTTPChecker(insecureClient, verifier)
	return withTimeout(checker, timeout)
}

type prometheusAPIVerifier struct {
	endpoint     string
	kubeAccessor *KubeAccessor
}

func (v prometheusAPIVerifier) Request() *http.Request {
	req, err := newGetRequest(v.endpoint, v.kubeAccessor.ServiceAccountToken())
	if err != nil {
		panic(err)
	}
	return req
}

/*
	Expecting JSON like this

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
func (v prometheusAPIVerifier) Verify(body []byte) checks.Error {
	path := "data.result.0.value.1"
	value := gjson.Get(string(body), path)

	if value.String() != "1" {
		return checks.ErrFail(`cannot find "1" in path %q prometheus data %q`, path, body)
	}

	return nil
}
