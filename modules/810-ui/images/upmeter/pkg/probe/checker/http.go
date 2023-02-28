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
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"d8.io/upmeter/pkg/check"
)

// httpChecker implements the checker for HTTP endpoints
type httpChecker struct {
	client   *http.Client
	verifier httpVerifier
	req      *http.Request
}

func newHTTPChecker(client *http.Client, verifier httpVerifier) check.Checker {
	return &httpChecker{
		client:   client,
		verifier: verifier,
	}
}

func (c *httpChecker) Check() check.Error {
	c.req = c.verifier.Request()

	// The request body is closed inside
	body, err := doRequest(c.client, c.req)
	if err != nil {
		return err
	}

	return c.verifier.Verify(body)
}

// httpVerifier defines HTTP request and body verification for an HTTP endpoint check
type httpVerifier interface {
	// Request to endpoint
	Request() *http.Request

	// Verify the response body from endpoint
	Verify(body []byte) check.Error
}

// newGetRequest prepares request object for given URL with auth token
func newGetRequest(endpoint, authToken, userAgent string) (*http.Request, check.Error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, check.ErrUnknown("cannot create request: %s", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
	req.Header.Set("User-Agent", userAgent)

	return req, nil
}

// doRequest sends the request to the endpoint with passed client
func doRequest(client *http.Client, req *http.Request) ([]byte, check.Error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, check.ErrFail("cannot dial %q: %v", req.URL, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, check.ErrFail("cannot read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, check.ErrFail(
			"HTTP: %s %s returned status %d: %q",
			req.Method, req.URL.String(), resp.StatusCode, body)
	}

	return body, nil
}

// newInsecureClient creates http.Client omitting TLS verificaton. Useful for accessing APIs via
// kube-rbac-proxy which has self-signed certificates.
func newInsecureClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: timeout,
	}
}
