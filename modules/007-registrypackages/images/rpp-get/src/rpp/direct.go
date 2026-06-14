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
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	registrySchemeHTTPS = "https"
	registrySchemeHTTP  = "http"

	manifestAccept = "application/vnd.oci.image.manifest.v1+json," +
		"application/vnd.docker.distribution.manifest.v2+json"
)

func splitRepository(repository string) (string, string) {
	repository = strings.Trim(strings.TrimSpace(repository), "/")
	host, repoPath, ok := strings.Cut(repository, "/")
	if !ok || host == "" || repoPath == "" {
		return "", ""
	}
	return host, repoPath
}

func registrySchemeOrDefault(scheme string) string {
	if strings.ToLower(strings.TrimSpace(scheme)) == registrySchemeHTTP {
		return registrySchemeHTTP
	}
	return registrySchemeHTTPS
}

func buildManifestURL(scheme, host, repoPath, digest string) string {
	return fmt.Sprintf("%s://%s/v2/%s/manifests/%s", scheme, host, repoPath, digest)
}

func buildBlobURL(scheme, host, repoPath, digest string) string {
	return fmt.Sprintf("%s://%s/v2/%s/blobs/%s", scheme, host, repoPath, digest)
}

func parseWWWAuthenticate(header string) (realm, service, scope string, ok bool) {
	header = strings.TrimSpace(header)
	const prefix = "Bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", "", "", false
	}

	params := parseAuthParams(header[len(prefix):])
	realm = params["realm"]
	if realm == "" {
		return "", "", "", false
	}
	return realm, params["service"], params["scope"], true
}

func parseAuthParams(s string) map[string]string {
	params := make(map[string]string)
	for _, part := range strings.Split(s, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)
		if key != "" {
			params[key] = value
		}
	}
	return params
}

func selectLastLayerDigest(manifest []byte) (string, error) {
	var parsed struct {
		MediaType string            `json:"mediaType"`
		Manifests []json.RawMessage `json:"manifests"`
		Layers    []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(manifest, &parsed); err != nil {
		return "", fmt.Errorf("parse manifest: %w", err)
	}
	if len(parsed.Manifests) > 0 {
		return "", fmt.Errorf("manifest is an index/list, not a single image manifest")
	}
	if len(parsed.Layers) == 0 {
		return "", fmt.Errorf("manifest has no layers")
	}
	digest := parsed.Layers[len(parsed.Layers)-1].Digest
	if digest == "" {
		return "", fmt.Errorf("last layer has empty digest")
	}
	return digest, nil
}

func buildTokenURL(realm, service, scope string) (string, error) {
	u, err := url.Parse(realm)
	if err != nil {
		return "", fmt.Errorf("parse token realm %q: %w", realm, err)
	}
	q := u.Query()
	if service != "" {
		q.Set("service", service)
	}
	if scope != "" {
		q.Set("scope", scope)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func readErrorBody(body io.Reader) string {
	data, _ := io.ReadAll(io.LimitReader(body, 255))
	return strings.TrimSpace(string(data))
}

type directClient struct {
	client     *http.Client
	repository string
	auth       string
	scheme     string

	mu    sync.Mutex
	token string // cached Bearer token, shared across manifest/blob requests
}

func newDirectClient(cfg Config) *directClient {
	return &directClient{
		client:     newDirectHTTPClient(cfg.RegistryCA),
		repository: cfg.RegistryRepo,
		auth:       cfg.RegistryAuth,
		scheme:     registrySchemeOrDefault(cfg.RegistryScheme),
	}
}

func newDirectHTTPClient(ca string) *http.Client {
	tlsConfig := &tls.Config{}
	if ca != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		pool.AppendCertsFromPEM([]byte(ca))
		tlsConfig.RootCAs = pool
	}

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
		TLSClientConfig:       tlsConfig,
	}

	return &http.Client{Transport: transport}
}

func (c *directClient) Get(ctx context.Context, digest string) (io.ReadCloser, string, error) {
	if err := validateDigest(digest); err != nil {
		return nil, "", err
	}
	digest = strings.TrimSpace(digest)

	host, repoPath := splitRepository(c.repository)
	if host == "" || repoPath == "" {
		return nil, "", fmt.Errorf("invalid registry repository %q, expected host/path", c.repository)
	}

	manifest, err := c.fetchManifest(ctx, host, repoPath, digest)
	if err != nil {
		return nil, "", err
	}

	layerDigest, err := selectLastLayerDigest(manifest)
	if err != nil {
		return nil, "", fmt.Errorf("select layer for %s: %w", digest, err)
	}

	body, err := c.fetchBlob(ctx, host, repoPath, layerDigest)
	if err != nil {
		return nil, "", err
	}
	return body, host, nil
}

func (c *directClient) fetchManifest(ctx context.Context, host, repoPath, digest string) ([]byte, error) {
	manifestURL := buildManifestURL(c.scheme, host, repoPath, digest)

	resp, err := c.doRegistryGet(ctx, manifestURL, manifestAccept)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read manifest body: %w", err)
	}
	return body, nil
}

func (c *directClient) fetchBlob(ctx context.Context, host, repoPath, digest string) (io.ReadCloser, error) {
	blobURL := buildBlobURL(c.scheme, host, repoPath, digest)
	resp, err := c.doRegistryGet(ctx, blobURL, "")
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (c *directClient) doRegistryGet(ctx context.Context, requestURL, accept string) (*http.Response, error) {
	usedToken := c.cachedToken()

	resp, err := c.sendGet(ctx, requestURL, accept, c.authorization(usedToken))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		token, tokenErr := c.refreshToken(ctx, resp, usedToken)
		resp.Body.Close()
		if tokenErr != nil {
			return nil, tokenErr
		}

		resp, err = c.sendGet(ctx, requestURL, accept, "Bearer "+token)
		if err != nil {
			return nil, err
		}
	}

	if resp.StatusCode != http.StatusOK {
		bodyText := readErrorBody(resp.Body)
		resp.Body.Close()
		return nil, &httpStatusError{packageURL: requestURL, statusCode: resp.StatusCode, body: bodyText}
	}

	return resp, nil
}

func (c *directClient) cachedToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

// authorization returns the header for an outgoing request: a cached Bearer
// token if we have one, otherwise the Basic credentials used to bootstrap the
// first token.
func (c *directClient) authorization(token string) string {
	if token != "" {
		return "Bearer " + token
	}
	return c.basicAuthHeader()
}

// refreshToken exchanges the Basic credentials for a Bearer token using the 401
// challenge, caches it, and returns it. usedToken is the token that just got
// rejected; if another goroutine already refreshed past it, that token is
// reused instead of fetching again. Holding the lock across the fetch serializes
// concurrent refreshers so only one token request is made per challenge.
func (c *directClient) refreshToken(ctx context.Context, challenge *http.Response, usedToken string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && c.token != usedToken {
		return c.token, nil
	}

	token, err := c.fetchBearerToken(ctx, challenge)
	if err != nil {
		return "", err
	}

	c.token = token
	return token, nil
}

func (c *directClient) sendGet(ctx context.Context, requestURL, accept, authorization string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if accept != "" {
		request.Header.Set("Accept", accept)
	}
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}

	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", requestURL, err)
	}
	return response, nil
}

func (c *directClient) basicAuthHeader() string {
	if c.auth == "" {
		return ""
	}
	return "Basic " + c.auth
}

func (c *directClient) fetchBearerToken(ctx context.Context, challenge *http.Response) (string, error) {
	realm, service, scope, ok := parseWWWAuthenticate(challenge.Header.Get("WWW-Authenticate"))
	if !ok {
		return "", fmt.Errorf("registry returned 401 without a usable Bearer challenge")
	}

	tokenURL, err := buildTokenURL(realm, service, scope)
	if err != nil {
		return "", err
	}

	resp, err := c.sendGet(ctx, tokenURL, "", c.basicAuthHeader())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &httpStatusError{packageURL: tokenURL, statusCode: resp.StatusCode, body: readErrorBody(resp.Body)}
	}

	var payload struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if payload.Token != "" {
		return payload.Token, nil
	}
	if payload.AccessToken != "" {
		return payload.AccessToken, nil
	}
	return "", fmt.Errorf("token response contained no token")
}
