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

package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// upstreamAuthTransport injects upstream registry credentials by satisfying the
// registry's WWW-Authenticate challenge: for Bearer it fetches a token from the
// challenge realm using Basic credentials; for Basic it sets the credentials
// directly. It retries the original request once with the obtained credentials.
type upstreamAuthTransport struct {
	base  http.RoundTripper
	creds *Credentials
}

// newUpstreamAuthTransport wraps base (http.DefaultTransport if nil) so requests
// carry upstream credentials. creds may be nil for anonymous upstreams.
func newUpstreamAuthTransport(base http.RoundTripper, creds *Credentials) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &upstreamAuthTransport{base: base, creds: creds}
}

func (t *upstreamAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	scheme, params := parseChallenge(resp.Header.Get("WWW-Authenticate"))
	if scheme == "" {
		return resp, nil
	}

	authz, err := t.authorization(req, scheme, params)
	if err != nil {
		return nil, err
	}
	if authz == "" {
		// Cannot satisfy the challenge (e.g. no creds); return the 401 as-is.
		return resp, nil
	}

	// Discard the 401 body before retrying.
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	retry := req.Clone(req.Context())
	retry.Header.Set("Authorization", authz)
	if req.Body != nil && req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("rewind request body for auth retry: %w", err)
		}
		retry.Body = body
	}
	return t.base.RoundTrip(retry)
}

func (t *upstreamAuthTransport) authorization(req *http.Request, scheme string, params map[string]string) (string, error) {
	switch strings.ToLower(scheme) {
	case "basic":
		if t.creds == nil {
			return "", nil
		}
		return "Basic " + basicAuth(t.creds.Username, t.creds.Password), nil
	case "bearer":
		token, err := t.fetchToken(req, params)
		if err != nil {
			return "", err
		}
		if token == "" {
			return "", nil
		}
		return "Bearer " + token, nil
	default:
		return "", nil
	}
}

func (t *upstreamAuthTransport) fetchToken(req *http.Request, params map[string]string) (string, error) {
	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("bearer challenge missing realm")
	}
	u, err := url.Parse(realm)
	if err != nil {
		return "", fmt.Errorf("parse realm %q: %w", realm, err)
	}
	q := u.Query()
	if s := params["service"]; s != "" {
		q.Set("service", s)
	}
	if s := params["scope"]; s != "" {
		q.Set("scope", s)
	}
	u.RawQuery = q.Encode()

	treq, err := http.NewRequestWithContext(req.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	if t.creds != nil {
		treq.SetBasicAuth(t.creds.Username, t.creds.Password)
	}
	resp, err := t.base.RoundTrip(treq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}
	var body struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if body.Token != "" {
		return body.Token, nil
	}
	return body.AccessToken, nil
}

// parseChallenge parses a WWW-Authenticate header into its scheme and params.
// Returns "" scheme when the header is empty.
func parseChallenge(header string) (string, map[string]string) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", nil
	}
	parts := strings.SplitN(header, " ", 2)
	scheme := parts[0]
	params := map[string]string{}
	if len(parts) == 2 {
		for _, kv := range splitParams(parts[1]) {
			eq := strings.IndexByte(kv, '=')
			if eq < 0 {
				continue
			}
			key := strings.TrimSpace(kv[:eq])
			val := strings.Trim(strings.TrimSpace(kv[eq+1:]), `"`)
			params[key] = val
		}
	}
	return scheme, params
}

// splitParams splits a challenge parameter list on commas that are not inside
// double quotes.
func splitParams(s string) []string {
	var out []string
	var b strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			b.WriteRune(r)
		case r == ',' && !inQuote:
			out = append(out, b.String())
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}
