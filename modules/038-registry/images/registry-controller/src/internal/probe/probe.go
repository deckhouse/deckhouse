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

// Package probe checks an upstream registry before the cluster is switched over
// to it.
//
// Changing the primary upstream — its address, its certificate authority, or its
// credentials, which is what changing a Deckhouse license amounts to — is applied
// only after the new upstream answers. Without this, a mistyped license key or a
// wrong path is indistinguishable from a healthy switch until the first pod
// cannot pull, and by then every node has already been reconfigured.
//
// The probe answers three separate questions, because they fail differently and
// an operator needs to know which one broke:
//
//  1. Reachability — does /v2/ answer at all.
//  2. Authorization — are these credentials accepted.
//  3. Content — is this the registry we think it is, or merely a reachable one.
package probe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// SentinelRepository is the repository whose presence tells a Deckhouse registry
// apart from any other reachable one.
//
// Deliberately probed through its tag list rather than a specific tag: the
// release channel a cluster follows is not known here, and hardcoding one would
// reject a perfectly good registry serving a different channel.
const SentinelRepository = "release-channel"

// FailureKind distinguishes the three questions the probe asks, so the reported
// reason points at what an operator has to fix.
type FailureKind string

const (
	// FailureUnreachable means /v2/ did not answer.
	FailureUnreachable FailureKind = "Unreachable"

	// FailureAuth means the credentials were rejected. This is the license case.
	FailureAuth FailureKind = "AuthFailed"

	// FailureSentinel means the registry answered and accepted the credentials,
	// but does not hold Deckhouse content at this path. A wrong address or a typo
	// in the path looks exactly like this.
	FailureSentinel FailureKind = "SentinelMissing"
)

// Failure is a probe verdict with the question that failed.
type Failure struct {
	Kind    FailureKind
	Message string
	Err     error
}

func (f *Failure) Error() string {
	if f.Err != nil {
		return fmt.Sprintf("%s: %s: %v", f.Kind, f.Message, f.Err)
	}
	return fmt.Sprintf("%s: %s", f.Kind, f.Message)
}

func (f *Failure) Unwrap() error { return f.Err }

// Reason maps a failure onto the condition reason recorded on RegistryConfig.
func (f *Failure) Reason() string {
	switch f.Kind {
	case FailureUnreachable:
		return registryv1alpha1.ReasonUnreachable
	case FailureAuth:
		return registryv1alpha1.ReasonAuthFailed
	case FailureSentinel:
		return registryv1alpha1.ReasonSentinelMissing
	default:
		return registryv1alpha1.ReasonProbeFailed
	}
}

// AsFailure extracts the verdict from an error returned by Probe.
func AsFailure(err error) (*Failure, bool) {
	var failure *Failure
	ok := errors.As(err, &failure)
	return failure, ok
}

// Prober checks whether an upstream can be switched to.
type Prober interface {
	Probe(ctx context.Context, upstream *registryv1alpha1.Upstream) error
}

// Registry probes a registry over its HTTP API.
type Registry struct {
	// Timeout bounds a single probe. A probe that hangs must not hold the whole
	// reconciliation, since everything else the controller does still needs to
	// happen.
	Timeout time.Duration
}

var _ Prober = (*Registry)(nil)

// Probe checks the upstream, and any of its mirrors that are configured.
//
// Only the primary endpoint has to pass. Mirrors serve the same content and exist
// for failover, so a mirror being down is not a reason to refuse a switch — the
// cluster would still work. Refusing would turn a redundancy feature into a
// single point of failure.
func (r *Registry) Probe(ctx context.Context, upstream *registryv1alpha1.Upstream) error {
	if upstream == nil {
		// Nothing configured means nothing to verify: air-gap is a valid target and
		// is gated on cache completeness instead.
		return nil
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return r.probeEndpoint(ctx, &upstream.Endpoint)
}

func (r *Registry) probeEndpoint(ctx context.Context, endpoint *registryv1alpha1.Endpoint) error {
	client, err := httpClient(endpoint)
	if err != nil {
		return &Failure{
			Kind:    FailureUnreachable,
			Message: fmt.Sprintf("cannot build a client for %s", endpoint.Host),
			Err:     err,
		}
	}

	base := baseURL(endpoint)

	// 1. Reachability, and the authentication challenge if there is one.
	challenge, err := r.probeAPIVersion(ctx, client, base)
	if err != nil {
		return err
	}

	// 2. Authorization. An anonymous registry issues no challenge, and there is
	// nothing to verify.
	token := ""
	if challenge != nil {
		token, err = r.authorize(ctx, client, endpoint, challenge)
		if err != nil {
			return err
		}
	}

	// 3. Content.
	return r.probeSentinel(ctx, client, base, endpoint, token)
}

func (r *Registry) probeAPIVersion(
	ctx context.Context, client *http.Client, base string,
) (*challenge, error) {
	response, err := get(ctx, client, base+"/v2/")
	if err != nil {
		return nil, &Failure{
			Kind:    FailureUnreachable,
			Message: fmt.Sprintf("%s/v2/ did not answer", base),
			Err:     err,
		}
	}
	defer drain(response)

	switch response.StatusCode {
	case http.StatusOK:
		return nil, nil
	case http.StatusUnauthorized:
		parsed := parseChallenge(response.Header.Get("WWW-Authenticate"))
		if parsed == nil {
			return nil, &Failure{
				Kind:    FailureAuth,
				Message: fmt.Sprintf("%s/v2/ requires authentication but offered no usable scheme", base),
			}
		}
		return parsed, nil
	default:
		// A 5xx or a redirect to a login page: reachable in the TCP sense, but not
		// a registry API.
		return nil, &Failure{
			Kind:    FailureUnreachable,
			Message: fmt.Sprintf("%s/v2/ answered %s", base, response.Status),
		}
	}
}

// authorize satisfies the challenge and returns a bearer token when the registry
// uses one.
func (r *Registry) authorize(
	ctx context.Context, client *http.Client, endpoint *registryv1alpha1.Endpoint, ch *challenge,
) (string, error) {
	if endpoint.Auth.IsEmpty() {
		return "", &Failure{
			Kind:    FailureAuth,
			Message: fmt.Sprintf("%s requires authentication but no credentials are configured", endpoint.Host),
		}
	}

	if ch.scheme == schemeBasic {
		// Basic auth is verified by the sentinel request itself; there is no
		// separate exchange to perform.
		return "", nil
	}

	tokenURL, err := ch.tokenURL(scopeFor(endpoint))
	if err != nil {
		return "", &Failure{
			Kind:    FailureAuth,
			Message: fmt.Sprintf("%s offered an unusable authentication realm", endpoint.Host),
			Err:     err,
		}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", &Failure{Kind: FailureAuth, Message: "cannot build the token request", Err: err}
	}
	setBasicAuth(request, endpoint.Auth)

	response, err := client.Do(request)
	if err != nil {
		return "", &Failure{
			Kind:    FailureAuth,
			Message: fmt.Sprintf("the authentication service of %s did not answer", endpoint.Host),
			Err:     err,
		}
	}
	defer drain(response)

	if response.StatusCode != http.StatusOK {
		// This is the license case: the address is right, the service is up, and the
		// credentials are not accepted.
		return "", &Failure{
			Kind: FailureAuth,
			Message: fmt.Sprintf("the authentication service of %s rejected the credentials with %s",
				endpoint.Host, response.Status),
		}
	}

	var body struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return "", &Failure{
			Kind:    FailureAuth,
			Message: fmt.Sprintf("the authentication service of %s returned an unreadable token", endpoint.Host),
			Err:     err,
		}
	}

	token := body.Token
	if token == "" {
		token = body.AccessToken
	}
	if token == "" {
		return "", &Failure{
			Kind:    FailureAuth,
			Message: fmt.Sprintf("the authentication service of %s returned an empty token", endpoint.Host),
		}
	}
	return token, nil
}

// probeSentinel checks that this registry actually holds Deckhouse content at the
// configured path, which is what tells "wrong registry" apart from "registry
// temporarily unavailable".
func (r *Registry) probeSentinel(
	ctx context.Context, client *http.Client, base string, endpoint *registryv1alpha1.Endpoint, token string,
) error {
	sentinel := fmt.Sprintf("%s/v2/%s/tags/list", base, sentinelRepository(endpoint))

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sentinel, nil)
	if err != nil {
		return &Failure{Kind: FailureSentinel, Message: "cannot build the sentinel request", Err: err}
	}
	switch {
	case token != "":
		request.Header.Set("Authorization", "Bearer "+token)
	case !endpoint.Auth.IsEmpty():
		setBasicAuth(request, endpoint.Auth)
	}

	response, err := client.Do(request)
	if err != nil {
		return &Failure{
			Kind:    FailureUnreachable,
			Message: fmt.Sprintf("%s did not answer", sentinel),
			Err:     err,
		}
	}
	defer drain(response)

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		// Reached only with basic auth, where the credentials are first exercised
		// here rather than at a token endpoint.
		return &Failure{
			Kind: FailureAuth,
			Message: fmt.Sprintf("%s rejected the credentials with %s",
				endpoint.Host, response.Status),
		}
	case http.StatusNotFound:
		return &Failure{
			Kind: FailureSentinel,
			Message: fmt.Sprintf("%s has no %q repository, so it does not look like a Deckhouse registry at path %q",
				endpoint.Host, SentinelRepository, endpoint.Path),
		}
	default:
		return &Failure{
			Kind:    FailureUnreachable,
			Message: fmt.Sprintf("%s answered %s", sentinel, response.Status),
		}
	}

	var body struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return &Failure{
			Kind:    FailureSentinel,
			Message: fmt.Sprintf("%s returned an unreadable tag list", sentinel),
			Err:     err,
		}
	}
	if len(body.Tags) == 0 {
		// An empty repository is worse than a missing one: it would pass a existence
		// check and then serve nothing.
		return &Failure{
			Kind: FailureSentinel,
			Message: fmt.Sprintf("the %q repository of %s is empty, so no release channel is served",
				SentinelRepository, endpoint.Host),
		}
	}

	return nil
}

func sentinelRepository(endpoint *registryv1alpha1.Endpoint) string {
	path := strings.Trim(endpoint.Path, "/")
	if path == "" {
		return SentinelRepository
	}
	return path + "/" + SentinelRepository
}

func scopeFor(endpoint *registryv1alpha1.Endpoint) string {
	return fmt.Sprintf("repository:%s:pull", sentinelRepository(endpoint))
}

func baseURL(endpoint *registryv1alpha1.Endpoint) string {
	scheme := endpoint.Scheme
	if scheme == "" {
		scheme = registryv1alpha1.SchemeHTTPS
	}
	return fmt.Sprintf("%s://%s", scheme.Lower(), endpoint.Host)
}

func httpClient(endpoint *registryv1alpha1.Endpoint) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if endpoint.CA != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(endpoint.CA)) {
			return nil, errors.New("the configured certificate authority is not valid PEM")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}

	return &http.Client{Transport: transport}, nil
}

func get(ctx context.Context, client *http.Client, target string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(request)
}

func setBasicAuth(request *http.Request, auth *registryv1alpha1.Auth) {
	if auth.IsEmpty() {
		return
	}
	if auth.Username != "" || auth.Password != "" {
		request.SetBasicAuth(auth.Username, auth.Password)
		return
	}
	// Only the pre-encoded form is available, which is exactly what the header
	// carries.
	request.Header.Set("Authorization", "Basic "+auth.Auth)
}

func drain(response *http.Response) {
	if response == nil || response.Body == nil {
		return
	}
	_ = response.Body.Close()
}

const (
	schemeBearer = "bearer"
	schemeBasic  = "basic"
)

// challenge is a parsed WWW-Authenticate header.
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
	scheme = strings.ToLower(scheme)

	switch scheme {
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
		return "", fmt.Errorf("realm %q is not an absolute URL", c.realm)
	}

	query := parsed.Query()
	query.Set("scope", scope)
	if c.service != "" {
		query.Set("service", c.service)
	}
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}
