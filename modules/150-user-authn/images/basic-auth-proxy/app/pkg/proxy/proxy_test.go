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
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// fakeProvider records call count and the last ctx for cache assertions.
type fakeProvider struct {
	calls  atomic.Int64
	groups []string
	err    error

	lastCtx atomic.Pointer[context.Context]
}

func (p *fakeProvider) ValidateCredentials(ctx context.Context, _, _ string) ([]string, error) {
	p.calls.Add(1)
	p.lastCtx.Store(&ctx)
	return p.groups, p.err
}

// newTestHandler wires a Handler to a fakeProvider and closes the ttlcache
// janitor on test end so goroutines don't leak.
func newTestHandler(t *testing.T, p *fakeProvider, authTTL, groupsTTL time.Duration) *Handler {
	t.Helper()
	h := New()
	h.provider = p
	h.AuthCacheTTL = authTTL
	h.GroupsCacheTTL = groupsTTL
	t.Cleanup(func() {
		h.cache.Close()
	})
	return h
}

func TestHandler_ValidateCredentials(t *testing.T) {
	t.Parallel()

	authErr := errors.New("invalid_grant")

	tests := []struct {
		name      string
		newProv   func() *fakeProvider
		login     string
		password  string
		repeats   int
		wantErr   error
		wantGrps  []string
		wantCalls int64
	}{
		{
			name:      "negative result is cached and returned as error on every hit",
			newProv:   func() *fakeProvider { return &fakeProvider{err: authErr} },
			login:     "user",
			password:  "wrong",
			repeats:   3,
			wantErr:   authErr,
			wantGrps:  nil,
			wantCalls: 1,
		},
		{
			name:      "positive result with groups is cached",
			newProv:   func() *fakeProvider { return &fakeProvider{groups: []string{"admins", "devs"}} },
			login:     "alice",
			password:  "ok",
			repeats:   3,
			wantErr:   nil,
			wantGrps:  []string{"admins", "devs"},
			wantCalls: 1,
		},
		{
			name:      "successful auth with empty groups (LDAP getUserInfo=false) is cached as success",
			newProv:   func() *fakeProvider { return &fakeProvider{} },
			login:     "bob",
			password:  "ok",
			repeats:   3,
			wantErr:   nil,
			wantGrps:  nil,
			wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()

			p := tt.newProv()
			h := newTestHandler(t, p, 10*time.Second, 2*time.Minute)

			for i := range tt.repeats {
				groups, err := h.validateCredentials(ctx, tt.login, tt.password)
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("call %d: got err %v, want %v", i, err, tt.wantErr)
				}
				if !slices.Equal(groups, tt.wantGrps) {
					t.Fatalf("call %d: got groups %v, want %v", i, groups, tt.wantGrps)
				}
			}

			if got := p.calls.Load(); got != tt.wantCalls {
				t.Fatalf("provider call count: got %d, want %d (cache must serve repeats)", got, tt.wantCalls)
			}
		})
	}
}

// TestHandler_NegativeCacheExpiresAndProviderIsRetried checks that after
// AuthCacheTTL the negative entry is gone and the provider is hit again.
// Uses wall-clock sleep because ttlcache's janitor goroutine deadlocks
// testing/synctest; TTL is kept tiny so the test stays sub-second.
func TestHandler_NegativeCacheExpiresAndProviderIsRetried(t *testing.T) {
	t.Parallel()

	const (
		ttl  = 50 * time.Millisecond
		wait = 200 * time.Millisecond
	)

	p := &fakeProvider{err: errors.New("invalid_grant")}
	h := newTestHandler(t, p, ttl, 2*time.Minute)

	if _, err := h.validateCredentials(t.Context(), "u", "wrong"); err == nil {
		t.Fatal("first call: expected error, got nil")
	}
	if _, err := h.validateCredentials(t.Context(), "u", "wrong"); err == nil {
		t.Fatal("second call (cache hit): expected error, got nil")
	}
	if got := p.calls.Load(); got != 1 {
		t.Fatalf("provider calls before expiry: got %d, want 1", got)
	}

	time.Sleep(wait)

	if _, err := h.validateCredentials(t.Context(), "u", "wrong"); err == nil {
		t.Fatal("third call (after expiry): expected error, got nil")
	}
	if got := p.calls.Load(); got != 2 {
		t.Fatalf("provider calls after expiry: got %d, want 2 (cache must have expired)", got)
	}
}

// TestHandler_UnexpectedCacheTypeIsRefetched: an unexpected cache value
// type must not panic and must trigger a refetch from the provider.
func TestHandler_UnexpectedCacheTypeIsRefetched(t *testing.T) {
	t.Parallel()

	want := []string{"g1"}
	p := &fakeProvider{groups: want}
	h := newTestHandler(t, p, 10*time.Second, 2*time.Minute)

	key := h.cacheKey("u", "p")
	h.cache.SetWithTTL(key, "garbage-not-a-cacheEntry", 10*time.Second)

	got, err := h.validateCredentials(t.Context(), "u", "p")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if c := p.calls.Load(); c != 1 {
		t.Fatalf("provider calls: got %d, want 1 (unexpected cache type must trigger refetch)", c)
	}
}

// TestHandler_CacheKey_DoesNotLeakPassword: password must not appear in
// the cache key (HMAC, not concatenation).
func TestHandler_CacheKey_DoesNotLeakPassword(t *testing.T) {
	t.Parallel()

	const password = "s3cr3t-do-not-leak"
	h := newTestHandler(t, &fakeProvider{}, 10*time.Second, 2*time.Minute)

	key := h.cacheKey("alice", password)
	if len(key) == 0 {
		t.Fatal("expected non-empty cache key")
	}
	if bytes.Contains([]byte(key), []byte(password)) {
		t.Fatal("password leaked into cache key (must be hashed, not concatenated)")
	}

	// Determinism: same inputs → same key.
	if h.cacheKey("alice", password) != key {
		t.Fatal("cache key must be deterministic for the lifetime of the Handler")
	}

	// Different password → different key.
	if h.cacheKey("alice", password+"x") == key {
		t.Fatal("different password must yield different cache key")
	}

	// Per-process key: a second Handler must derive a different key for the
	// same inputs (no cross-process predictable key).
	h2 := newTestHandler(t, &fakeProvider{}, 10*time.Second, 2*time.Minute)
	if h2.cacheKey("alice", password) == key {
		t.Fatal("cache key must be derived with a per-process random HMAC key")
	}
}

// TestHandler_CacheKey_DomainSeparation: pairs with identical
// concatenations must produce different keys (length-prefix is in effect).
func TestHandler_CacheKey_DomainSeparation(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t, &fakeProvider{}, 10*time.Second, 2*time.Minute)

	tests := []struct {
		name             string
		loginA, passwdA  string
		loginB, passwdB  string
	}{
		{"plain concat collision", "ab", "cd", "a", "bcd"},
		{"nul-byte collision", "attacker\x00", "X", "attacker", "\x00X"},
		{"colon-byte collision", "user", ":secret", "user:", "secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if h.cacheKey(tt.loginA, tt.passwdA) == h.cacheKey(tt.loginB, tt.passwdB) {
				t.Fatalf("cache key collision between (%q,%q) and (%q,%q): length-prefixing missing?",
					tt.loginA, tt.passwdA, tt.loginB, tt.passwdB)
			}
		})
	}
}

// TestHandler_ServeHTTP is the e2e regression for the cache-bypass and
// the X-Remote-* header-injection privesc.
func TestHandler_ServeHTTP(t *testing.T) {
	t.Parallel()

	type upstreamObservation struct {
		headers http.Header
	}

	tests := []struct {
		name              string
		provider          *fakeProvider
		basicAuth         [2]string
		extraHeaders      http.Header
		repeats           int
		wantStatus        int
		wantUpstream      int64
		wantProvCalls     int64
		wantBasicAuth     bool
		assertLastRequest func(t *testing.T, obs upstreamObservation)
	}{
		{
			name:          "valid credentials are proxied upstream and cached",
			provider:      &fakeProvider{groups: []string{"devs"}},
			basicAuth:     [2]string{"alice", "ok"},
			repeats:       3,
			wantStatus:    http.StatusOK,
			wantUpstream:  3,
			wantProvCalls: 1,
			wantBasicAuth: true,
			assertLastRequest: func(t *testing.T, obs upstreamObservation) {
				t.Helper()
				if got := obs.headers.Get("X-Remote-User"); got != "alice" {
					t.Fatalf("X-Remote-User: got %q, want %q", got, "alice")
				}
				if got := obs.headers.Values("X-Remote-Group"); !slices.Equal(got, []string{"devs"}) {
					t.Fatalf("X-Remote-Group: got %v, want %v", got, []string{"devs"})
				}
			},
		},
		{
			name:          "invalid credentials yield 403 on every call within AuthCacheTTL (bypass regression)",
			provider:      &fakeProvider{err: errors.New("invalid_grant")},
			basicAuth:     [2]string{"user", "wrong"},
			repeats:       3,
			wantStatus:    http.StatusForbidden,
			wantUpstream:  0,
			wantProvCalls: 1,
			wantBasicAuth: true,
		},
		{
			name:          "request without basic auth yields 401",
			provider:      &fakeProvider{},
			repeats:       1,
			wantStatus:    http.StatusUnauthorized,
			wantUpstream:  0,
			wantProvCalls: 0,
			wantBasicAuth: false,
		},
		{
			name:      "client-supplied X-Remote-* headers MUST NOT leak to upstream (privilege escalation regression)",
			provider:  &fakeProvider{groups: []string{"devs"}},
			basicAuth: [2]string{"alice", "ok"},
			extraHeaders: http.Header{
				"X-Remote-User":           []string{"attacker"},
				"X-Remote-Group":          []string{"system:masters", "system:cluster-admins"},
				"X-Remote-Extra-Scopes":   []string{"cluster-admin"},
				"X-Remote-Extra-Whatever": []string{"evil"},
			},
			repeats:       1,
			wantStatus:    http.StatusOK,
			wantUpstream:  1,
			wantProvCalls: 1,
			wantBasicAuth: true,
			assertLastRequest: func(t *testing.T, obs upstreamObservation) {
				t.Helper()
				if got := obs.headers.Get("X-Remote-User"); got != "alice" {
					t.Fatalf("X-Remote-User: got %q, want %q (client-supplied value must be overwritten)", got, "alice")
				}
				if got := obs.headers.Values("X-Remote-Group"); !slices.Equal(got, []string{"devs"}) {
					t.Fatalf("X-Remote-Group: got %v, want %v (client-supplied groups must not leak)", got, []string{"devs"})
				}
				for k := range obs.headers {
					if len(k) >= len(extraHeaderPrefix) && k[:len(extraHeaderPrefix)] == extraHeaderPrefix {
						t.Fatalf("X-Remote-Extra-* header %q must not be forwarded, got %v", k, obs.headers.Values(k))
					}
				}
				if got := obs.headers.Get("Authorization"); got != "" {
					t.Fatalf("Authorization header must be stripped, got %q", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				upstreamHits atomic.Int64
				lastObs      atomic.Pointer[upstreamObservation]
			)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamHits.Add(1)
				obs := upstreamObservation{headers: r.Header.Clone()}
				lastObs.Store(&obs)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(upstream.Close)

			u, err := url.Parse(upstream.URL)
			if err != nil {
				t.Fatalf("parse upstream url: %v", err)
			}

			h := newTestHandler(t, tt.provider, 10*time.Second, 2*time.Minute)
			h.reverseProxy = httputil.NewSingleHostReverseProxy(u)

			for i := range tt.repeats {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
				if tt.wantBasicAuth {
					req.SetBasicAuth(tt.basicAuth[0], tt.basicAuth[1])
				}
				for k, vs := range tt.extraHeaders {
					for _, v := range vs {
						req.Header.Add(k, v)
					}
				}

				h.ServeHTTP(rec, req)

				if rec.Code != tt.wantStatus {
					t.Fatalf("call %d: status got %d, want %d", i, rec.Code, tt.wantStatus)
				}
			}

			if got := upstreamHits.Load(); got != tt.wantUpstream {
				t.Fatalf("upstream hits: got %d, want %d", got, tt.wantUpstream)
			}
			if got := tt.provider.calls.Load(); got != tt.wantProvCalls {
				t.Fatalf("provider calls: got %d, want %d", got, tt.wantProvCalls)
			}
			if tt.assertLastRequest != nil {
				obs := lastObs.Load()
				if obs == nil {
					t.Fatal("expected upstream to be called at least once")
				}
				tt.assertLastRequest(t, *obs)
			}
		})
	}
}

// TestStripIdentityHeaders is a focused unit test for the helper.
func TestStripIdentityHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   http.Header
		want http.Header
	}{
		{
			name: "removes X-Remote-User",
			in:   http.Header{"X-Remote-User": []string{"attacker"}, "X-Other": []string{"keep"}},
			want: http.Header{"X-Other": []string{"keep"}},
		},
		{
			name: "removes all X-Remote-Group values",
			in:   http.Header{"X-Remote-Group": []string{"system:masters", "evil"}},
			want: http.Header{},
		},
		{
			name: "removes any X-Remote-Extra-* header",
			in: http.Header{
				"X-Remote-Extra-Scopes":     []string{"cluster-admin"},
				"X-Remote-Extra-Department": []string{"infra"},
				"X-Real-Ip":                 []string{"keep"},
			},
			want: http.Header{"X-Real-Ip": []string{"keep"}},
		},
		{
			name: "removes all identity headers at once",
			in: http.Header{
				"X-Remote-User":         []string{"attacker"},
				"X-Remote-Group":        []string{"system:masters"},
				"X-Remote-Extra-Scopes": []string{"x"},
				"User-Agent":            []string{"curl/1"},
			},
			want: http.Header{"User-Agent": []string{"curl/1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stripIdentityHeaders(tt.in)
			if len(tt.in) != len(tt.want) {
				t.Fatalf("len: got %d, want %d (got %v)", len(tt.in), len(tt.want), tt.in)
			}
			for k, want := range tt.want {
				got := tt.in.Values(k)
				if !slices.Equal(got, want) {
					t.Fatalf("header %q: got %v, want %v", k, got, want)
				}
			}
		})
	}
}
