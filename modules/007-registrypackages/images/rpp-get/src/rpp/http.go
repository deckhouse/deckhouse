/*
Copyright 2026 Flant JSC

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

package rpp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var digestRegexp = regexp.MustCompile(`^[a-z0-9]+:[a-z0-9]+$`)

type httpStatusError struct {
	packageURL string
	statusCode int
	body       string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("access to %s returned HTTP %d: %s", e.packageURL, e.statusCode, e.body)
}

type httpClient struct {
	client     *http.Client
	endpoints  []string
	token      string
	repository string
	path       string
}

func newHTTPClient(cfg Config) *httpClient {
	return &httpClient{
		client:     newBaseHTTPClient(),
		endpoints:  cfg.Endpoints,
		token:      cfg.Token,
		repository: cfg.Repository,
		path:       cfg.Path,
	}
}

func newBaseHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   connectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: responseHeaderTimeout,
		TLSHandshakeTimeout:   connectTimeout,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		},
	}

	return &http.Client{Transport: transport}
}

func (c *httpClient) Get(ctx context.Context, digest string) (*http.Response, error) {
	if err := validateDigest(digest); err != nil {
		return nil, err
	}
	digest = strings.TrimSpace(digest)

	if len(c.endpoints) == 0 {
		return nil, errNoEndpoints
	}

	endpoint := c.endpoints[rand.IntN(len(c.endpoints))]

	packageURL := buildPackageURL(endpoint, digest, c.repository, c.path)
	return c.doGet(ctx, packageURL, c.token)
}

func (c *httpClient) doGet(ctx context.Context, packageURL, token string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, packageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", packageURL, err)
	}

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 255))
		response.Body.Close()
		return nil, &httpStatusError{
			packageURL: packageURL,
			statusCode: response.StatusCode,
			body:       strings.TrimSpace(string(body)),
		}
	}

	return response, nil
}

func validateDigest(digest string) error {
	if !digestRegexp.MatchString(digest) {
		return errInvalidDigest
	}

	return nil
}

func shouldRetryFetch(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, errInvalidDigest) ||
		errors.Is(err, errNoEndpoints) ||
		errors.Is(err, errNoToken) {
		return false
	}

	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		code := statusErr.statusCode
		return code == http.StatusRequestTimeout ||
			code == http.StatusTooManyRequests ||
			code >= http.StatusInternalServerError
	}

	return true
}

func buildPackageURL(endpoint, digest, repository, path string) string {
	values := url.Values{"digest": []string{digest}}

	if repository != "" {
		values.Set("repository", repository)
	}

	if path != "" {
		values.Set("path", path)
	}

	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	return "https://" + endpoint + "/package?" + values.Encode()
}
