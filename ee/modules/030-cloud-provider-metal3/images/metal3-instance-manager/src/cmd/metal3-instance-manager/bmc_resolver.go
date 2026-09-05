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

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	ipmiclient "github.com/bougou/go-ipmi/pkg/client"
	ipmitypes "github.com/bougou/go-ipmi/pkg/types"
)

type BMCConfig struct {
	IPAddress  string
	Port       int
	SystemUUID string
	Insecure   bool
}

type ResolvedBMC struct {
	Protocol   string
	Address    string
	SystemUUID string
}

type BMCResolver interface {
	Resolve(ctx context.Context, config BMCConfig, username, password string) (ResolvedBMC, error)
}

type networkBMCResolver struct {
	timeout time.Duration
}

func newNetworkBMCResolver(timeout time.Duration) BMCResolver {
	return &networkBMCResolver{timeout: timeout}
}

func (r *networkBMCResolver) Resolve(ctx context.Context, config BMCConfig, username, password string) (ResolvedBMC, error) {
	probeTimeout := r.timeout / 3
	if probeTimeout <= 0 {
		probeTimeout = time.Second
	}

	redfish, redfishErr := r.resolveRedfish(ctx, config, username, password, probeTimeout)
	if redfishErr == nil {
		return redfish, nil
	}
	ipmiCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	ipmi, ipmiErr := r.resolveIPMI(ipmiCtx, config, username, password)
	if ipmiErr == nil {
		return ipmi, nil
	}
	return ResolvedBMC{}, fmt.Errorf("no supported BMC protocol resolved: Redfish: %v; IPMI: %v", redfishErr, ipmiErr)
}

func (r *networkBMCResolver) resolveRedfish(ctx context.Context, config BMCConfig, username, password string, probeTimeout time.Duration) (ResolvedBMC, error) {
	var errs []error
	for _, endpoint := range redfishEndpoints(config) {
		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		resolved, err := resolveRedfishEndpoint(probeCtx, endpoint, config, username, password)
		cancel()
		if err == nil {
			return resolved, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", endpoint, err))
	}
	return ResolvedBMC{}, errors.Join(errs...)
}

func redfishEndpoints(config BMCConfig) []string {
	if config.Port != 0 {
		host := net.JoinHostPort(config.IPAddress, fmt.Sprintf("%d", config.Port))
		return []string{"https://" + host, "http://" + host}
	}
	return []string{
		"https://" + net.JoinHostPort(config.IPAddress, "443"),
		"http://" + net.JoinHostPort(config.IPAddress, "80"),
	}
}

type redfishClient struct {
	baseURL  string
	username string
	password string
	token    string
	session  string
	client   *http.Client
}

type odataReference struct {
	ODataID string `json:"@odata.id"`
}

type redfishRoot struct {
	Systems        odataReference `json:"Systems"`
	SessionService odataReference `json:"SessionService"`
	Links          struct {
		Sessions odataReference `json:"Sessions"`
	} `json:"Links"`
}

type redfishCollection struct {
	Members []odataReference `json:"Members"`
}

type redfishSystem struct {
	ODataID string `json:"@odata.id"`
	UUID    string `json:"UUID"`
}

func resolveRedfishEndpoint(ctx context.Context, endpoint string, config BMCConfig, username, password string) (ResolvedBMC, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: config.Insecure} //nolint:gosec
	c := &redfishClient{
		baseURL:  strings.TrimSuffix(endpoint, "/"),
		username: username,
		password: password,
		client:   &http.Client{Transport: transport},
	}
	defer c.closeSession(ctx)

	var root redfishRoot
	if err := c.getJSON(ctx, "/redfish/v1/", &root); err != nil {
		return ResolvedBMC{}, err
	}
	systemsPath := root.Systems.ODataID
	if systemsPath == "" {
		systemsPath = "/redfish/v1/Systems"
	}
	var collection redfishCollection
	if err := c.getJSON(ctx, systemsPath, &collection); err != nil {
		return ResolvedBMC{}, err
	}

	matches := make([]redfishSystem, 0, 1)
	for _, member := range collection.Members {
		if member.ODataID == "" {
			continue
		}
		var system redfishSystem
		if err := c.getJSON(ctx, member.ODataID, &system); err != nil {
			return ResolvedBMC{}, fmt.Errorf("read ComputerSystem %q: %w", member.ODataID, err)
		}
		if equalUUID(system.UUID, config.SystemUUID) {
			if system.ODataID == "" {
				system.ODataID = member.ODataID
			}
			matches = append(matches, system)
		}
	}
	if len(matches) == 0 {
		return ResolvedBMC{}, fmt.Errorf("ComputerSystem UUID %q not found", config.SystemUUID)
	}
	if len(matches) > 1 {
		return ResolvedBMC{}, fmt.Errorf("ComputerSystem UUID %q matched %d resources", config.SystemUUID, len(matches))
	}
	return ResolvedBMC{
		Protocol:   "Redfish",
		Address:    "redfish+" + c.baseURL + ensureLeadingSlash(matches[0].ODataID),
		SystemUUID: matches[0].UUID,
	}, nil
}

func (c *redfishClient) getJSON(ctx context.Context, path string, target interface{}) error {
	response, err := c.do(ctx, http.MethodGet, path, nil, true)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode Redfish response from %q: %w", path, err)
	}
	return nil
}

func (c *redfishClient) do(ctx context.Context, method, path string, body []byte, allowSession bool) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+ensureLeadingSlash(path), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		request.Header.Set("X-Auth-Token", c.token)
	} else {
		request.SetBasicAuth(c.username, c.password)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusUnauthorized && allowSession && c.token == "" {
		response.Body.Close()
		if err := c.createSession(ctx); err != nil {
			return nil, err
		}
		return c.do(ctx, method, path, body, false)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		message, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		return nil, fmt.Errorf("Redfish returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	return response, nil
}

func (c *redfishClient) createSession(ctx context.Context) error {
	payload, _ := json.Marshal(map[string]string{"UserName": c.username, "Password": c.password})
	response, err := c.do(ctx, http.MethodPost, "/redfish/v1/SessionService/Sessions", payload, false)
	if err != nil {
		return fmt.Errorf("create Redfish session: %w", err)
	}
	response.Body.Close()
	c.token = response.Header.Get("X-Auth-Token")
	c.session = response.Header.Get("Location")
	if c.token == "" {
		return fmt.Errorf("create Redfish session: response has no X-Auth-Token")
	}
	return nil
}

func (c *redfishClient) closeSession(ctx context.Context) {
	if c.token == "" || c.session == "" {
		return
	}
	response, err := c.do(ctx, http.MethodDelete, c.session, nil, false)
	if err == nil {
		response.Body.Close()
	}
}

func (r *networkBMCResolver) resolveIPMI(ctx context.Context, config BMCConfig, username, password string) (ResolvedBMC, error) {
	port := config.Port
	if port == 0 {
		port = 623
	}
	c, err := ipmiclient.NewClient(config.IPAddress, port, username, password)
	if err != nil {
		return ResolvedBMC{}, err
	}
	if err := c.Connect(ctx); err != nil {
		return ResolvedBMC{}, err
	}
	defer c.Close(ctx)
	response, err := c.GetSystemGUID(ctx)
	if err != nil {
		return ResolvedBMC{}, fmt.Errorf("get IPMI System GUID: %w", err)
	}
	uuid, err := ipmitypes.ParseGUID(response.GUID[:], ipmitypes.GUIDModeSMBIOS)
	if err != nil {
		return ResolvedBMC{}, fmt.Errorf("parse IPMI System GUID: %w", err)
	}
	if !equalUUID(uuid.String(), config.SystemUUID) {
		return ResolvedBMC{}, fmt.Errorf("IPMI System GUID %q does not match requested UUID %q", uuid.String(), config.SystemUUID)
	}
	return ResolvedBMC{
		Protocol:   "IPMI",
		Address:    "ipmi://" + net.JoinHostPort(config.IPAddress, fmt.Sprintf("%d", port)),
		SystemUUID: uuid.String(),
	}, nil
}

func equalUUID(left, right string) bool {
	normalize := func(value string) string {
		return strings.ToLower(strings.Trim(strings.TrimSpace(value), "{}"))
	}
	return normalize(left) != "" && normalize(left) == normalize(right)
}

func ensureLeadingSlash(path string) string {
	if parsed, err := url.Parse(path); err == nil && parsed.IsAbs() {
		return parsed.RequestURI()
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}
