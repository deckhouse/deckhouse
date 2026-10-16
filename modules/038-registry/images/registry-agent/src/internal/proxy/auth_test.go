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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func jwtExpiringAt(exp time.Time) string {
	encode := base64.RawURLEncoding.EncodeToString
	return encode([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." +
		encode([]byte(fmt.Sprintf(`{"iss":"Registry server","exp":%d}`, exp.Unix()))) + ".signature"
}

func jwtIssued(iat, exp time.Time) string {
	encode := base64.RawURLEncoding.EncodeToString
	return encode([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." +
		encode([]byte(fmt.Sprintf(`{"iss":"Registry server","iat":%d,"exp":%d}`, iat.Unix(), exp.Unix()))) + ".signature"
}

// TestARefusedTokenIsForgotten is the review's finding on the token cache: a token kept for as long as
// it claims to be valid was offered again after the target refused it — a store's signing key rotated,
// a node clock off — until it would have expired, and in an air-gapped cluster there is nothing else
// to try meanwhile. Forgotten, the next request exchanges a new one.
func TestARefusedTokenIsForgotten(t *testing.T) {
	a := &authenticator{}
	target := &Target{Host: "store:5001", Path: "/v2/system/deckhouse/manifests/v1"}
	key := keyFor(t, target)

	a.store(key, "refused", time.Now().Add(time.Hour))
	_, ok := a.cached(key)
	require.True(t, ok)

	a.forget(target, "Bearer refused")
	_, ok = a.cached(key)
	assert.False(t, ok, "a refused token is not offered again")
}

// TestForgetLeavesANewerToken is the review's finding on forget: requests sent with one token are
// refused together, and by the time the later ones come back the first has already obtained a new
// token. Forgetting whatever was cached threw that one away too, and every refused request exchanged
// its own.
func TestForgetLeavesANewerToken(t *testing.T) {
	a := &authenticator{}
	target := &Target{Host: "store:5001", Path: "/v2/system/deckhouse/manifests/v1"}
	key := keyFor(t, target)

	a.store(key, "fresh", time.Now().Add(time.Hour))

	a.forget(target, "Bearer stale")
	token, ok := a.cached(key)
	require.True(t, ok, "a refusal of an older token leaves the newer one")
	assert.Equal(t, "fresh", token)

	a.forget(target, "Basic dXNlcjpwYXNz")
	a.forget(target, "")
	_, ok = a.cached(key)
	assert.True(t, ok, "a request that carried no bearer token names nothing to forget")

	a.forget(target, "Bearer fresh")
	_, ok = a.cached(key)
	assert.False(t, ok)
}

// TestConcurrentExchangesCollapse is the review's finding on exchanges: a token missing for everyone
// at once — expired, or forgotten after a key rotation — sent every request running at the time into
// an exchange of its own, each a bcrypt comparison on the store.
func TestConcurrentExchangesCollapse(t *testing.T) {
	release := make(chan struct{})
	service := startTokenService(t, func() string { return `{"token":"shared","expires_in":300}` }, release)
	a := &authenticator{}

	const requests = 16
	var started, done sync.WaitGroup
	headers := make([]string, requests)
	errs := make([]error, requests)
	started.Add(requests)
	done.Add(requests)
	for i := range requests {
		go func() {
			defer done.Done()
			request, err := http.NewRequest(http.MethodGet, "http://unused/", nil)
			if err != nil {
				errs[i] = err
				started.Done()
				return
			}
			started.Done()
			errs[i] = a.authorize(context.Background(), service.client, request, service.target)
			headers[i] = request.Header.Get("Authorization")
		}()
	}

	// Every request is under way before the token service answers, so they all find the
	// token missing; the exchange is held until the stragglers have had time to join it.
	started.Wait()
	require.Eventually(t, func() bool { return service.exchanges.Load() > 0 }, 5*time.Second, time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	close(release)
	done.Wait()

	for i := range requests {
		require.NoError(t, errs[i])
		assert.Equal(t, "Bearer shared", headers[i])
	}
	assert.EqualValues(t, 1, service.exchanges.Load(), "one exchange for every request waiting on the key")
	assert.EqualValues(t, 1, service.probes.Load(), "and one probe")
}

// TestAShortLivedTokenIsNotCached is the review's finding on the leeway: a token living no longer than
// the leeway was raised to the conventional minute and kept for thirty seconds, past its own `exp`.
func TestAShortLivedTokenIsNotCached(t *testing.T) {
	now := time.Now()
	for name, body := range map[string]string{
		"a JWT":      fmt.Sprintf(`{"token":%q}`, jwtIssued(now, now.Add(tokenLeeway))),
		"expires_in": `{"token":"short","expires_in":10}`,
	} {
		t.Run(name, func(t *testing.T) {
			service := startTokenService(t, func() string { return body }, nil)
			a := &authenticator{}

			for range 2 {
				request, err := http.NewRequest(http.MethodGet, "http://unused/", nil)
				require.NoError(t, err)
				require.NoError(t, a.authorize(context.Background(), service.client, request, service.target))
				assert.NotEmpty(t, request.Header.Get("Authorization"), "the token still serves the request it was obtained for")
			}

			_, ok := a.cached(keyFor(t, service.target))
			assert.False(t, ok, "but is not kept")
			assert.EqualValues(t, 2, service.exchanges.Load(), "so every request exchanges its own")
		})
	}

	// And one that lives long enough is kept, which is what the two above are measured against.
	service := startTokenService(t, func() string { return `{"token":"long","expires_in":300}` }, nil)
	a := &authenticator{}
	for range 2 {
		request, err := http.NewRequest(http.MethodGet, "http://unused/", nil)
		require.NoError(t, err)
		require.NoError(t, a.authorize(context.Background(), service.client, request, service.target))
	}
	assert.EqualValues(t, 1, service.exchanges.Load())
}

func keyFor(t *testing.T, target *Target) tokenKey {
	t.Helper()

	scope, err := tokenScope(target)
	require.NoError(t, err)
	return tokenKey{host: target.Host, scope: scope}
}

// tokenService is a registry that demands a bearer token and the token service that issues it,
// counting what it is asked.
type tokenService struct {
	target    *Target
	client    *http.Client
	probes    atomic.Int32
	exchanges atomic.Int32
}

// startTokenService answers every exchange with body, after release is closed if it is given.
func startTokenService(t *testing.T, body func() string, release <-chan struct{}) *tokenService {
	t.Helper()

	service := &tokenService{}
	var server *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(writer http.ResponseWriter, _ *http.Request) {
		service.probes.Add(1)
		writer.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s/token",service="registry"`, server.URL))
		writer.WriteHeader(http.StatusUnauthorized)
	})
	mux.HandleFunc("/token", func(writer http.ResponseWriter, _ *http.Request) {
		service.exchanges.Add(1)
		if release != nil {
			<-release
		}
		_, _ = writer.Write([]byte(body()))
	})
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	service.client = server.Client()
	service.target = &Target{
		Scheme: registryv1alpha1.SchemeHTTP,
		Host:   parsed.Host,
		Path:   "/v2/system/deckhouse/manifests/v1",
		Auth:   &registryv1alpha1.Auth{Username: "user", Password: "pass"},
	}
	return service
}

func TestTokenLifetime(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)

	assert.Equal(t, 300*time.Second, tokenLifetime(300, jwtExpiringAt(now.Add(time.Hour)), now),
		"what the response says comes first")

	// A token service like the store's own: no lifetime in the response, only in the token.
	assert.Equal(t, 5*time.Minute, tokenLifetime(0, jwtExpiringAt(now.Add(5*time.Minute)), now),
		"a response without a lifetime keeps the token for as long as the token itself says")
	assert.Equal(t, maxTokenLifetime, tokenLifetime(0, jwtExpiringAt(now.Add(15*time.Minute)), now),
		"but no longer than the bound: a token the agent cannot tell is stale costs at most that")

	// Issued fifteen minutes ago by this node's clock and valid for five: the issuer's arithmetic, not
	// a clock that is off, decides how long it is kept.
	issued := jwtIssued(now.Add(-15*time.Minute), now.Add(-10*time.Minute))
	assert.Equal(t, 5*time.Minute, tokenLifetime(0, issued, now),
		"exp − iat holds whatever this node's clock says")

	assert.Equal(t, time.Minute, tokenLifetime(0, "opaque-token", now),
		"a token that says nothing either is kept for the conventional minute")
	assert.Equal(t, 10*time.Second, tokenLifetime(0, jwtExpiringAt(now.Add(10*time.Second)), now),
		"one about to expire is not raised to the minute: it is not to be kept past its own exp")
	assert.Equal(t, 10*time.Second, tokenLifetime(10, "opaque-token", now),
		"nor is a short lifetime the response states")
	assert.Equal(t, time.Minute, tokenLifetime(0, "a.not-base64!.c", now))
}
