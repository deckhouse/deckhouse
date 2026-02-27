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
	"net/http"
	"time"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// HTTPEndpointAvailable verifies that an endpoint responds with HTTP 200.
type HTTPEndpointAvailable struct {
	Access   kubernetes.Access
	Timeout  time.Duration
	Endpoint string
}

func (c HTTPEndpointAvailable) Checker() check.Checker {
	verifier := httpEndpointVerifier{
		endpoint: c.Endpoint,
		access:   c.Access,
	}
	checker := newHTTPChecker(newInsecureClient(3*c.Timeout), verifier)
	return withTimeout(checker, c.Timeout)
}

type httpEndpointVerifier struct {
	endpoint string
	access   kubernetes.Access
}

func (v httpEndpointVerifier) Request() *http.Request {
	req, err := newGetRequest(v.endpoint, v.access.ServiceAccountToken(), v.access.UserAgent())
	if err != nil {
		panic(err)
	}
	return req
}

func (v httpEndpointVerifier) Verify(_ []byte) check.Error {
	return nil
}
