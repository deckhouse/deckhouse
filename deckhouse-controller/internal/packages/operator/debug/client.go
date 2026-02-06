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

package debug

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Client communicates with a debug Server over a Unix domain socket using HTTP.
// All requests are routed through the socket; the HTTP host is ignored by the transport.
type Client struct {
	socketPath string
	httpClient *http.Client
}

// NewClient creates a Client that connects to the debug server at the given Unix socket path.
// Returns an error if the socket file does not exist.
func NewClient(socketPath string) (*Client, error) {
	if _, err := os.Stat(socketPath); err != nil {
		return nil, fmt.Errorf("stat socket file '%s': %w", socketPath, err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				dialer := &net.Dialer{
					Timeout: 10 * time.Second,
				}
				return dialer.DialContext(ctx, "unix", socketPath)
			},
			DisableKeepAlives: true,
		},
	}

	return &Client{
		socketPath: socketPath,
		httpClient: client,
	}, nil
}

// Close releases transport resources held by the underlying HTTP client.
func (c *Client) Close() {
	if c.httpClient != nil {
		if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
}

// Get sends a GET request to the debug server. Path segments are joined to form the URL.
// Returns the response body or an error if the request fails or the server returns a non-2xx status.
func (c *Client) Get(ctx context.Context, paths ...string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, makeURL(paths...), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	return c.do(req)
}

// do executes an HTTP request and returns the response body.
// Returns an error for non-2xx status codes.
func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	if _, err = io.Copy(buf, resp.Body); err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, buf.String())
	}

	return buf.Bytes(), nil
}

// makeURL constructs an HTTP URL for the Unix socket transport from path segments.
func makeURL(paths ...string) string {
	return fmt.Sprintf("http://unix/%s", filepath.Join(paths...))
}
