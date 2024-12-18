// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docs_builder

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
)

func NewClient(httpClient d8http.Client) *Client {
	return &Client{httpClient: httpClient}
}

type Client struct {
	httpClient d8http.Client
}

func (c *Client) SendDocumentation(ctx context.Context, baseAddr string, moduleName, moduleVersion string, docsArchive io.Reader) error {
	url := fmt.Sprintf("%s/api/v1/doc/%s/%s", baseAddr, moduleName, moduleVersion)
	response, statusCode, err := c.sendRequest(ctx, http.MethodPost, url, docsArchive)
	if err != nil {
		return fmt.Errorf("POST %q: %w", url, err)
	}

	if statusCode != http.StatusCreated {
		return fmt.Errorf("POST %q: [%d] %q", url, statusCode, response)
	}

	return nil
}

func (c *Client) DeleteDocumentation(ctx context.Context, baseAddr string, moduleName string) error {
	url := fmt.Sprintf("%s/api/v1/doc/%s", baseAddr, moduleName)
	response, statusCode, err := c.sendRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("DELETE %q: %w", url, err)
	}

	if statusCode != http.StatusNoContent {
		return fmt.Errorf("DELETE %q: [%d] %q", url, statusCode, response)
	}

	return nil
}

func (c *Client) BuildDocumentation(ctx context.Context, docsBuilderBasePath string) error {
	url := fmt.Sprintf("%s/api/v1/build", docsBuilderBasePath)
	response, statusCode, err := c.sendRequest(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("POST %q: %w", url, err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("POST %q: [%d] %q", url, statusCode, response)
	}

	return nil
}

func (c *Client) CheckBuilderHealth(ctx context.Context, baseAddr string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/healthz", baseAddr)
	response, statusCode, err := c.sendRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("GET %q: %w", url, err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("GET %q: [%d] %q", url, statusCode, response)
	}

	return nil
}

func (c *Client) sendRequest(ctx context.Context, method string, url string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()

	dataBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, fmt.Errorf("read all: %w", err)
	}

	return dataBytes, res.StatusCode, nil
}
