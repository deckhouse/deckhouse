// Copyright 2024 Flant JSC
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

package client

import (
	"context"
	"io"
	"math/rand"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

type Client struct {
	endpoints []string
	token     string
}

func New(endpoints []string, token string) (*Client, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("endpoints is empty")
	}

	return &Client{
		endpoints: endpoints,
		token:     token,
	}, nil
}

func (c *Client) GetPackage(ctx context.Context, digest string, repository string) (int64, io.ReadCloser, error) {
	endpoint := c.endpoints[rand.Intn(len(c.endpoints))]

	var scheme string

	if c.token == "" {
		scheme = "http"
	} else {
		scheme = "https"
	}

	url := url.URL{
		Scheme: scheme,
		Host:   endpoint,
		Path:   "/package",
	}

	query := url.Query()

	query.Set("digest", digest)
	query.Set("repository", repository)

	url.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return 0, nil, errors.Wrap(err, "failed to create request")
	}

	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, nil, errors.Wrap(err, "failed to do request")
	}

	return response.ContentLength, response.Body, nil
}
