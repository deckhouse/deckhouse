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

package providercheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type defaultHTTP struct{}

var (
	_ HTTPFactory = defaultHTTP{}
	_ HTTPDoer    = (*http.Client)(nil)
)

// NewDefaultHTTP returns an HTTPFactory backed by net/http with httpTimeout.
func NewDefaultHTTP() HTTPFactory {
	return defaultHTTP{}
}

func (defaultHTTP) New(opts TLSOptions) (HTTPDoer, error) {
	tlsConfig, err := buildTLSConfig(opts.RootCAData, "", opts.InsecureSkipVerify)
	if err != nil {
		return nil, err
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("http default transport is not *http.Transport")
	}
	transport := base.Clone()
	transport.TLSClientConfig = tlsConfig
	// One request per client; keep-alives would pin FDs after the 16-worker probe burst.
	transport.DisableKeepAlives = true
	return &http.Client{
		Timeout:   httpTimeout,
		Transport: transport,
	}, nil
}

func httpGet(ctx context.Context, client HTTPDoer, rawURL, basicUser, basicPass string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	if basicUser != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response body: %w", err)
	}
	return resp.StatusCode, body, nil
}

func httpPostForm(
	ctx context.Context,
	client HTTPDoer,
	rawURL, basicUser, basicPass string,
	form url.Values,
) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if basicUser != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response body: %w", err)
	}
	return resp.StatusCode, body, nil
}

func unmarshalJSON(body []byte, dest any) error {
	return json.Unmarshal(body, dest)
}

// oauthErrorCode extracts the RFC 6749 "error" code from a token or
// introspection error response body, or "" when it is absent or unparseable.
func oauthErrorCode(body []byte) string {
	var parsed struct {
		Error string `json:"error"`
	}
	_ = unmarshalJSON(body, &parsed)
	return parsed.Error
}

type oauthClientSecretCheck struct {
	stepName     string
	providerName string
	tokenURL     string
	clientID     string
	clientSecret string
}

func reportOAuthClientSecret(
	ctx context.Context,
	client HTTPDoer,
	result *dexProviderCheckResult,
	check oauthClientSecretCheck,
) {
	if check.clientID == "" || check.clientSecret == "" {
		result.skip(check.stepName, "clientID or clientSecret is empty")
		return
	}

	accepted, detail, err := probeClientSecret(ctx, client, check.tokenURL, check.clientID, check.clientSecret)
	switch {
	case err != nil:
		result.failUnreachable(check.stepName, err, "cannot reach the %s token endpoint: %v", check.providerName, err)
	case !accepted:
		result.fail(check.stepName, "%s rejected the client credentials: %s", check.providerName, detail)
	default:
		result.succeed(check.stepName, "%s client credentials are valid (%s)", check.providerName, detail)
	}
}

func probeClientSecret(
	ctx context.Context,
	client HTTPDoer,
	tokenURL, clientID, clientSecret string,
) (bool, string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", "deckhouse-dex-provider-check")
	form.Set("redirect_uri", "https://deckhouse.local/dex-provider-check")

	statusCode, body, err := httpPostForm(ctx, client, tokenURL, clientID, clientSecret, form)
	if err != nil {
		return false, "", err
	}
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		return true, "the provider authenticated the client and issued a token", nil
	}

	errCode := oauthErrorCode(body)
	clientAuthFailed := statusCode == http.StatusUnauthorized ||
		errCode == "invalid_client" ||
		errCode == "unauthorized_client"
	if clientAuthFailed {
		if errCode != "" {
			return false, fmt.Sprintf("client authentication failed (%s)", errCode), nil
		}
		return false, "client authentication failed (HTTP 401)", nil
	}

	if errCode != "" {
		return true, fmt.Sprintf("the provider authenticated the client and rejected only the synthetic probe code, as expected (%s)", errCode), nil
	}
	return true, fmt.Sprintf("the provider authenticated the client (token endpoint returned HTTP %d)", statusCode), nil
}

func (r *Reconciler) checkHTTPReachability(
	ctx context.Context,
	result *dexProviderCheckResult,
	stepName string,
	rawURL string,
	rootCAData string,
) {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		result.fail(stepName, "URL %q is invalid: %v", rawURL, err)
		return
	}

	client, err := r.http.New(TLSOptions{RootCAData: rootCAData})
	if err != nil {
		result.fail(stepName, "cannot build HTTP client: %v", err)
		return
	}
	statusCode, _, err := httpGet(ctx, client, rawURL, "", "")
	if err != nil {
		result.failUnreachable(stepName, err, "URL %q is not reachable: %v", rawURL, err)
		return
	}
	switch {
	case statusCode >= http.StatusInternalServerError:
		result.fail(stepName, "URL %q returned HTTP %d", rawURL, statusCode)
	case statusCode >= http.StatusBadRequest:
		result.succeed(stepName, "URL %q is reachable; the endpoint is up (HTTP %d is expected for a connectivity probe)", rawURL, statusCode)
	default:
		result.succeed(stepName, "URL %q is reachable (HTTP %d)", rawURL, statusCode)
	}
}

func checkPublicBrowserURL(result *dexProviderCheckResult, stepName, rawURL string) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		result.fail(stepName, "URL %q is invalid: %v", rawURL, err)
		return
	}
	if reason := clusterInternalHostReason(parsed.Hostname()); reason != "" {
		result.fail(stepName,
			"URL %q is not browser-reachable (%s); the user's browser is redirected here during login, so it must be a publicly resolvable domain",
			rawURL, reason)
		return
	}
	result.succeed(stepName, "URL %q uses a publicly resolvable host", rawURL)
}

func clusterInternalHostReason(host string) string {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if ip := net.ParseIP(h); ip != nil {
		switch {
		case ip.IsLoopback():
			return "loopback IP address"
		case ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast():
			return "link-local IP address"
		case ip.IsPrivate():
			return "private IP address"
		case ip.IsUnspecified():
			return "unspecified IP address"
		}
		return ""
	}
	if h == "localhost" {
		return "localhost"
	}
	for _, suffix := range []string{".cluster.local", ".svc", ".local"} {
		if strings.HasSuffix(h, suffix) {
			return "cluster-internal domain (" + suffix + ")"
		}
	}
	if !strings.Contains(h, ".") {
		return "single-label hostname resolvable only inside the cluster"
	}
	return ""
}
