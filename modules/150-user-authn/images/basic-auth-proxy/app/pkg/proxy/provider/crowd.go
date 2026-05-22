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

package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// CrowdConfig groups NewCrowd parameters.
type CrowdConfig struct {
	APIURL        string
	Login         string
	Password      string
	AllowedGroups []string
}

type Crowd struct {
	client *crowdClient
}

func NewCrowd(cfg CrowdConfig) (Provider, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	groups := make(map[string]struct{}, len(cfg.AllowedGroups))
	for _, group := range cfg.AllowedGroups {
		groups[group] = struct{}{}
	}

	return &Crowd{
		client: &crowdClient{
			apiURL:        strings.TrimSuffix(cfg.APIURL, "/"),
			login:         cfg.Login,
			password:      cfg.Password,
			allowedGroups: groups,
			httpClient:    client,
		},
	}, nil
}

func (p *Crowd) ValidateCredentials(ctx context.Context, login, password string) ([]string, error) {
	_, err := p.client.MakeRequest(ctx, "/session", http.MethodPost, struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: login, Password: password})
	if err != nil {
		return nil, err
	}

	body, err := p.client.MakeRequest(ctx, "/user/group/nested?username="+login, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	crowdGroups, err := p.client.GetGroups(body)
	if err != nil {
		return nil, err
	}

	return crowdGroups, nil
}

type crowdClient struct {
	apiURL   string
	login    string
	password string

	allowedGroups map[string]struct{}
	httpClient    *http.Client
}

func (c *crowdClient) MakeRequest(ctx context.Context, url, method string, jsonPayload any) (string, error) {
	var body io.Reader
	if jsonPayload != nil {
		jsonData, err := json.Marshal(jsonPayload)
		if err != nil {
			return "", fmt.Errorf("crowd request marshal: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/rest/usermanagement/1%s", c.apiURL, url), body)
	if err != nil {
		return "", fmt.Errorf("crowd request build: %w", err)
	}

	req.SetBasicAuth(c.login, c.password)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("crowd request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("crowd response read: %w", err)
	}

	if (resp.StatusCode != http.StatusOK) && (resp.StatusCode != http.StatusCreated) {
		return "", fmt.Errorf("crowd request was not successful: %d %s", resp.StatusCode, string(responseBody))
	}

	return string(responseBody), nil
}

func (c *crowdClient) GetGroups(body string) ([]string, error) {
	var crowdGroups struct {
		Groups []struct {
			Name string `json:"name"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(body), &crowdGroups); err != nil {
		return nil, err
	}

	groups := make([]string, 0, len(crowdGroups.Groups))
	for _, value := range crowdGroups.Groups {
		if len(c.allowedGroups) > 0 {
			if _, ok := c.allowedGroups[value.Name]; ok {
				groups = append(groups, value.Name)
			}
		} else {
			groups = append(groups, value.Name)
		}
	}
	return groups, nil
}
