package monitoring_and_autoscaling

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"

	"upmeter/pkg/checks"
)

// httpChecker implements the checker for HTTP endpoints
type httpChecker struct {
	client   *http.Client
	verifier httpVerifier
	req      *http.Request
}

func newHTTPChecker(client *http.Client, verifier httpVerifier) Checker {
	return &httpChecker{
		client:   client,
		verifier: verifier,
	}
}

func (c *httpChecker) Check() checks.Error {
	c.req = c.verifier.Request()
	body, err := doRequest(c.client, c.req)
	if err != nil {
		return err
	}
	err = c.verifier.Verify(body)
	return err
}

func (c *httpChecker) BusyWith() string {
	// It might feel that here we can be caught by a race condition.
	// But in normal case request is always created before timeout message is used.
	if c.req == nil {
		return "(request not ready)"
	}
	return c.req.URL.String()
}

// httpVerifier defines HTTP request and body verification for an HTTP endpoint check
type httpVerifier interface {
	// Request to endpoint
	Request() *http.Request

	// Verify the response body from endpoint
	Verify(body []byte) checks.Error
}

// newGetRequest prepares request object for given URL with auth token
func newGetRequest(endpoint, authToken string) (*http.Request, checks.Error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, checks.ErrUnknownResult("cannot create request: %s", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	return req, nil
}

// doRequest sends the request to the endpoint with passed client
func doRequest(client *http.Client, req *http.Request) ([]byte, checks.Error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, checks.ErrUnknownResult("cannot dial %q: %v", req.URL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, checks.ErrFail("got HTTP status %q", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, checks.ErrFail("cannot read response body: %v", err)
	}
	defer resp.Body.Close()

	return body, nil
}

// Insecure transport is useful when kube-rbac-proxy generates self-signed certificates, causing cert validation error
var insecureClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}
