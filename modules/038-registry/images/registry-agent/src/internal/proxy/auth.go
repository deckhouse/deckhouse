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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// authenticator satisfies a registry's authentication challenge on the client's
// behalf.
//
// The agent has to do this itself rather than pass the challenge along. The runtime
// asked the AGENT for the image, so a challenge forwarded to it would be answered
// against the agent's own address with credentials the runtime does not have. The
// credentials for the real target are only here.
type authenticator struct {
	// tokens caches bearer tokens per target and scope. A pull is a sequence of
	// requests — manifest, then config, then every layer — and paying for a token
	// exchange on each would multiply the latency of every image on the node.
	tokens sync.Map
}

type tokenKey struct {
	host  string
	scope string
}

type cachedToken struct {
	value   string
	expires time.Time
}

// tokenLeeway retires a token before it actually expires, so a long blob transfer
// does not start on a token that dies halfway.
const tokenLeeway = 30 * time.Second

// authorize adds credentials to a request for a target.
//
// Basic credentials are sent directly; a bearer challenge is exchanged for a token
// first. A target that wants neither is left alone: passing credentials to a registry
// that did not ask for them would leak them to anything that can answer on that
// address.
func (a *authenticator) authorize(
	ctx context.Context, client *http.Client, request *http.Request, target *Target, repository string,
) error {
	if target.Auth.IsEmpty() {
		return nil
	}

	scope := fmt.Sprintf("repository:%s:pull", repository)
	if token, ok := a.cached(target.Host, scope); ok {
		request.Header.Set("Authorization", "Bearer "+token)
		return nil
	}

	challenge, err := a.probe(ctx, client, target)
	if err != nil {
		return err
	}

	switch {
	case challenge == nil:
		// The target accepts anonymous requests. Sending credentials anyway would hand
		// them to whatever is answering on that address.
		return nil

	case challenge.scheme == schemeBasic:
		setBasicAuth(request, target.Auth)
		return nil

	default:
		token, expires, err := a.exchange(ctx, client, challenge, target.Auth, scope)
		if err != nil {
			return err
		}
		a.store(target.Host, scope, token, expires)
		request.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
}

func (a *authenticator) cached(host, scope string) (string, bool) {
	value, ok := a.tokens.Load(tokenKey{host: host, scope: scope})
	if !ok {
		return "", false
	}
	token, ok := value.(cachedToken)
	if !ok || time.Now().After(token.expires) {
		return "", false
	}
	return token.value, true
}

func (a *authenticator) store(host, scope, token string, expires time.Time) {
	a.tokens.Store(tokenKey{host: host, scope: scope}, cachedToken{value: token, expires: expires})
}

// probe asks the target what authentication it wants.
func (a *authenticator) probe(
	ctx context.Context, client *http.Client, target *Target,
) (*challenge, error) {
	scheme := target.Scheme
	if scheme == "" {
		scheme = registryv1alpha1.SchemeHTTPS
	}
	endpoint := fmt.Sprintf("%s://%s/v2/", scheme.Lower(), target.Host)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("asking %s for its authentication scheme: %w", target.Host, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusUnauthorized {
		return nil, nil
	}
	return parseChallenge(response.Header.Get("WWW-Authenticate")), nil
}

// exchange trades credentials for a bearer token.
func (a *authenticator) exchange(
	ctx context.Context, client *http.Client, ch *challenge, auth *registryv1alpha1.Auth, scope string,
) (string, time.Time, error) {
	endpoint, err := ch.tokenURL(scope)
	if err != nil {
		return "", time.Time{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	setBasicAuth(request, auth)

	response, err := client.Do(request)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("obtaining a token from %s: %w", ch.realm, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("%s rejected the credentials with %s", ch.realm, response.Status)
	}

	var body struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return "", time.Time{}, fmt.Errorf("reading the token from %s: %w", ch.realm, err)
	}

	token := body.Token
	if token == "" {
		token = body.AccessToken
	}
	if token == "" {
		return "", time.Time{}, fmt.Errorf("%s returned an empty token", ch.realm)
	}

	lifetime := time.Duration(body.ExpiresIn) * time.Second
	if lifetime <= tokenLeeway {
		// Registries that report no lifetime conventionally mean a minute; and anything
		// shorter than the leeway is not worth caching at all.
		lifetime = time.Minute
	}
	return token, time.Now().Add(lifetime - tokenLeeway), nil
}

const (
	schemeBearer = "bearer"
	schemeBasic  = "basic"
)

type challenge struct {
	scheme  string
	realm   string
	service string
}

func parseChallenge(header string) *challenge {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil
	}

	scheme, rest, _ := strings.Cut(header, " ")
	switch strings.ToLower(scheme) {
	case schemeBasic:
		return &challenge{scheme: schemeBasic}
	case schemeBearer:
	default:
		return nil
	}

	parsed := &challenge{scheme: schemeBearer}
	for _, part := range strings.Split(rest, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		value = strings.Trim(value, `"`)
		switch strings.ToLower(key) {
		case "realm":
			parsed.realm = value
		case "service":
			parsed.service = value
		}
	}

	if parsed.realm == "" {
		// Nothing to exchange against.
		return nil
	}
	return parsed
}

func (c *challenge) tokenURL(scope string) (string, error) {
	parsed, err := url.Parse(c.realm)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("the authentication realm %q is not an absolute URL", c.realm)
	}

	query := parsed.Query()
	query.Set("scope", scope)
	if c.service != "" {
		query.Set("service", c.service)
	}
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func setBasicAuth(request *http.Request, auth *registryv1alpha1.Auth) {
	if auth.IsEmpty() {
		return
	}
	if auth.Username != "" || auth.Password != "" {
		request.SetBasicAuth(auth.Username, auth.Password)
		return
	}
	request.Header.Set("Authorization", "Basic "+auth.Auth)
}
