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
	"encoding/base64"
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

	// mu guards flights: the exchanges under way, one per key, which every request
	// arriving for that key while one is running waits for instead of starting its own.
	//
	// A token is missing for everyone at once — the moment it expires, or the moment a
	// rotated signing key on the store makes it refused and forgotten — and a pull asks
	// for its layers in parallel, a node for its images in parallel. Each exchange costs
	// the store's token service a bcrypt comparison, so without this a single expiry is
	// a burst of them, one per request that happened to be in flight.
	//
	// A local equivalent of golang.org/x/sync/singleflight rather than the module, which
	// this one does not otherwise depend on.
	mu      sync.Mutex
	flights map[tokenKey]*flight
}

// flight is one probe-and-exchange for a key and what it came to, for every request
// waiting on it. The fields are written before done is closed and read only after.
type flight struct {
	done chan struct{}

	// basic is set when the target asked for basic credentials, bearer when it handed
	// out a token; neither when it wants nothing at all.
	basic  bool
	bearer string
	err    error
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
	ctx context.Context, client *http.Client, request *http.Request, target *Target,
) error {
	if target.Auth.IsEmpty() {
		return nil
	}

	// The scope names the repository this request is actually going to, which is not the one
	// the client asked for: the whole point of a backend is that it serves the same images
	// under its own prefix, and the path has already been rewritten for it.
	//
	// Taken from the client's repository, the token comes back issued for a repository the
	// target has never heard of, and the target answers `insufficient_scope` — with valid
	// credentials, against an image that is right there. Nothing noticed for as long as that
	// 401 was handed to the container runtime, which then authenticated itself with the pod's
	// pull secret and succeeded: the agent's own credentials were never what made a pull work.
	scope, err := tokenScope(target)
	if err != nil {
		return fmt.Errorf("working out what to ask a token for: %w", err)
	}

	key := tokenKey{host: target.Host, scope: scope}
	if token, ok := a.cached(key); ok {
		request.Header.Set("Authorization", "Bearer "+token)
		return nil
	}

	f := a.join(ctx, client, target, key)
	select {
	case <-f.done:
	case <-ctx.Done():
		return ctx.Err()
	}

	switch {
	case f.err != nil:
		return f.err

	case f.basic:
		setBasicAuth(request, target.Auth)

	case f.bearer != "":
		request.Header.Set("Authorization", "Bearer "+f.bearer)

	default:
		// The target accepts anonymous requests. Sending credentials anyway would hand
		// them to whatever is answering on that address.
	}
	return nil
}

// join returns the exchange under way for a key, starting one if there is none.
//
// The exchange runs on its own, detached from the request that happened to start it: the
// others waiting on it must not fail because that one client went away. It keeps the
// starter's deadline, which is what bounds it.
func (a *authenticator) join(
	ctx context.Context, client *http.Client, target *Target, key tokenKey,
) *flight {
	a.mu.Lock()
	defer a.mu.Unlock()

	if f, ok := a.flights[key]; ok {
		return f
	}

	// Looked at again under the lock: an exchange that finished between the caller's look
	// and this one has stored its token before leaving flights, and starting another would
	// be the very exchange this is here to save.
	if token, ok := a.cached(key); ok {
		f := &flight{done: make(chan struct{}), bearer: token}
		close(f.done)
		return f
	}

	if a.flights == nil {
		a.flights = map[tokenKey]*flight{}
	}
	f := &flight{done: make(chan struct{})}
	a.flights[key] = f

	detached := context.WithoutCancel(ctx)
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(DefaultResponseTimeout)
	}
	flightCtx, cancel := context.WithDeadline(detached, deadline)

	go func() {
		defer cancel()
		a.fly(flightCtx, client, target, key, f)
	}()
	return f
}

// fly probes the target and, if it wants a token, exchanges one, recording the outcome in
// the flight for everyone waiting on it.
func (a *authenticator) fly(
	ctx context.Context, client *http.Client, target *Target, key tokenKey, f *flight,
) {
	defer func() {
		a.mu.Lock()
		delete(a.flights, key)
		a.mu.Unlock()
		close(f.done)
	}()

	challenge, err := a.probe(ctx, client, target)
	if err != nil {
		f.err = err
		return
	}

	switch {
	case challenge == nil:
	case challenge.scheme == schemeBasic:
		f.basic = true
	default:
		token, expires, err := a.exchange(ctx, client, challenge, target.Auth, key.scope)
		if err != nil {
			f.err = err
			return
		}
		// A token that the leeway retires before it could ever be reused — tokenLifetime no
		// longer than tokenLeeway — serves the requests waiting on this exchange and is not
		// stored at all.
		if expires.After(time.Now()) {
			a.store(key, token, expires)
		}
		f.bearer = token
	}
}

// tokenScope is the scope a token for this target's request is asked for: the repository the request
// goes to on the target, which is not the one the client asked for — see authorize.
func tokenScope(target *Target) (string, error) {
	repository, _, err := splitAPIPath(target.Path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("repository:%s:pull", repository), nil
}

// forget drops the token a target has just refused, so that the next request exchanges a new one
// instead of offering the refused one again until it would have expired — up to maxTokenLifetime, with
// nothing else to fall back on in an air-gapped cluster. A signing key rotated on the store, or a node
// clock far enough off, is what refuses a token this agent still believes in.
//
// Only that token, named by the Authorization header the refused request carried. Requests sent
// with it are refused together, and the first to come back has already led a new exchange by the time
// the rest do; dropping whatever is cached for the key would throw away the fresh token too and send
// every one of them into an exchange of its own. A header that is not a bearer token — the request
// went out with basic credentials, or was redirected to another host, which drops the header — names
// no cached token, and nothing is dropped.
func (a *authenticator) forget(target *Target, authorization string) {
	refused, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok || refused == "" {
		return
	}
	scope, err := tokenScope(target)
	if err != nil {
		return
	}
	key := tokenKey{host: target.Host, scope: scope}
	value, ok := a.tokens.Load(key)
	if !ok {
		return
	}
	if token, ok := value.(cachedToken); ok && token.value == refused {
		a.tokens.CompareAndDelete(key, value)
	}
}

func (a *authenticator) cached(key tokenKey) (string, bool) {
	value, ok := a.tokens.Load(key)
	if !ok {
		return "", false
	}
	token, ok := value.(cachedToken)
	if !ok || time.Now().After(token.expires) {
		return "", false
	}
	return token.value, true
}

func (a *authenticator) store(key tokenKey, token string, expires time.Time) {
	a.tokens.Store(key, cachedToken{value: token, expires: expires})
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

	return token, time.Now().Add(tokenLifetime(body.ExpiresIn, token, time.Now()) - tokenLeeway), nil
}

// tokenLifetime is how long a token is good for: what the response says, or else what the token says
// about itself, or else a minute.
//
// The middle answer is the one this module's own store needs. Its token service states no lifetime in
// the response, only inside the token — a JWT valid for fifteen minutes — and taking the conventional
// minute for it meant a token kept for thirty seconds after the leeway. The platform checks its module
// sources every three minutes, so every check exchanged a fresh token for every module repository:
// measured on a leader, close to two exchanges a second, each a bcrypt comparison, about a tenth of a
// core for as long as the cluster ran.
//
// The claim is read, not verified: it decides only how long this agent keeps a token before asking
// again, and a registry refusing a token it no longer honours makes the agent forget it — see forget —
// so the next request exchanges a new one. Bounded by maxTokenLifetime all the same: a token the agent
// cannot tell is stale costs at most that long.
//
// A lifetime no longer than tokenLeeway is returned as it is, and the token is not cached at all — see
// fly. Raised to the conventional minute, as it once was, such a token was kept for thirty seconds
// whatever it said, past its own `exp`.
func tokenLifetime(expiresIn int, token string, now time.Time) time.Duration {
	if expiresIn > 0 {
		return time.Duration(expiresIn) * time.Second
	}
	if lifetime := jwtLifetime(token, now); lifetime != 0 {
		return min(lifetime, maxTokenLifetime)
	}
	// Registries that report no lifetime conventionally mean a minute.
	return time.Minute
}

// maxTokenLifetime bounds how long a token is kept on the strength of its own claims.
const maxTokenLifetime = 10 * time.Minute

// jwtLifetime is how long a JWT is valid for: `exp − iat` when it says when it was issued, which is the
// issuer's own arithmetic and holds whatever this node's clock says; the time left until `exp` by this
// node's clock otherwise. Zero when the token is not one.
func jwtLifetime(token string, now time.Time) time.Duration {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0
	}
	var claims struct {
		Expires  int64 `json:"exp"`
		IssuedAt int64 `json:"iat"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Expires == 0 {
		return 0
	}
	if claims.IssuedAt > 0 && claims.IssuedAt < claims.Expires {
		return time.Unix(claims.Expires, 0).Sub(time.Unix(claims.IssuedAt, 0))
	}
	return time.Unix(claims.Expires, 0).Sub(now)
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
