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

package tokenexchangeprovider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// probeTimeout bounds each request of the discovery probe, as Dex bounds its own (patch 400).
	probeTimeout = 10 * time.Second
	// maxProbeBytes bounds a discovery document or key set read into memory.
	maxProbeBytes = 1 << 20
)

// probeDiscovery reads the discovery document of the issuer and its key set the way the
// connector will: Dex reads them only at the first exchange, so without the probe a provider whose
// issuer is unreachable, or names another issuer, would look ready and refuse every exchange.
func probeDiscovery(ctx context.Context, spec Spec) error {
	discovery := spec.Keys.Discovery
	if discovery == nil {
		return nil
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	if discovery.CAData != "" && !pool.AppendCertsFromPEM([]byte(discovery.CAData)) {
		return errors.New("spec.keys.discovery.caData holds no PEM certificate")
	}
	// A probe makes two requests and is done: its connections are not kept, and nothing follows a
	// redirect away from https.
	httpClient := &http.Client{
		Timeout: probeTimeout,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to %s: only https is followed", req.URL.Redacted())
			}
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	defer httpClient.CloseIdleConnections()

	url := discovery.URL
	if url == "" {
		url = strings.TrimSuffix(spec.Issuer, "/") + "/.well-known/openid-configuration"
	}
	var doc struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := getJSON(ctx, httpClient, url, &doc); err != nil {
		return fmt.Errorf("discovery document %s: %w", url, err)
	}
	if doc.Issuer != spec.Issuer {
		return fmt.Errorf("discovery document %s names the issuer %q, not %q", url, doc.Issuer, spec.Issuer)
	}
	if doc.JWKSURI == "" {
		return fmt.Errorf("discovery document %s has no jwks_uri", url)
	}
	// Dex refuses keys read over plain http (patch 420): they could be swapped on the way.
	if !strings.HasPrefix(doc.JWKSURI, "https://") {
		return fmt.Errorf("discovery document %s names the jwks_uri %s, which is not https", url, doc.JWKSURI)
	}

	var keys struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := getJSON(ctx, httpClient, doc.JWKSURI, &keys); err != nil {
		return fmt.Errorf("key set %s: %w", doc.JWKSURI, err)
	}
	if len(keys.Keys) == 0 {
		return fmt.Errorf("key set %s has no keys", doc.JWKSURI)
	}
	return nil
}

func getJSON(ctx context.Context, httpClient *http.Client, url string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get: status %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxProbeBytes)).Decode(into); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
