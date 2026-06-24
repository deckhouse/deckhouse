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
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubAuth struct{ ok bool }

func (s stubAuth) Authenticate(user, pass string) bool { return s.ok }

// TestHandler_CacheRewritesAuthRealm asserts proxyCache rewrites the host of the
// Bearer realm in a cache 401 to the agent's own host, so a node's containerd
// (node DNS) fetches the token via the agent instead of the cache's unresolvable
// Service DNS.
func TestHandler_CacheRewritesAuthRealm(t *testing.T) {
	cache := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Www-Authenticate",
			`Bearer realm="https://registry-cache.d8-system.svc:5001/auth",service="Deckhouse registry",scope="repository:system/deckhouse:pull"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer cache.Close()

	router := NewRouter([]Route{{NS: "registry.d8-system.svc:5001", Mode: ModeCache, CacheURL: cache.URL}})
	h, err := NewHandler(router, stubAuth{ok: false})
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v2/system/deckhouse/manifests/x?ns=registry.d8-system.svc:5001", nil)
	req.Host = "127.0.0.1:5001"
	h.ServeHTTP(rr, req)

	got := rr.Result().Header.Get("Www-Authenticate")
	if !strings.Contains(got, `realm="https://127.0.0.1:5001/auth"`) {
		t.Fatalf("realm not rewritten to agent host: %q", got)
	}
	if strings.Contains(got, "registry-cache.d8-system.svc") {
		t.Fatalf("cache Service DNS still present in realm: %q", got)
	}
	// service/scope must survive the rewrite.
	if !strings.Contains(got, `service="Deckhouse registry"`) || !strings.Contains(got, "scope=") {
		t.Fatalf("service/scope lost in rewrite: %q", got)
	}
}

// TestHandler_RoutesAuthToCache asserts a /auth token request (no ns) is routed
// to the cache's auth endpoint with its query preserved.
func TestHandler_RoutesAuthToCache(t *testing.T) {
	var gotPath, gotQuery string
	cache := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"token":"abc"}`)
	}))
	defer cache.Close()

	router := NewRouter([]Route{{NS: "registry.d8-system.svc:5001", Mode: ModeCache, CacheURL: cache.URL}})
	h, err := NewHandler(router, stubAuth{ok: false})
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth?service=Deckhouse+registry&scope=repository:system/deckhouse:pull", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if gotPath != "/auth" {
		t.Fatalf("cache received path %q, want /auth", gotPath)
	}
	if !strings.Contains(gotQuery, "scope=") {
		t.Fatalf("scope not forwarded to cache: %q", gotQuery)
	}
	if b, _ := io.ReadAll(rr.Result().Body); string(b) != `{"token":"abc"}` {
		t.Fatalf("token body not returned: %s", b)
	}
}

func TestHandler_CachePassThrough(t *testing.T) {
	var gotAuth, gotNS string
	cache := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotNS = r.URL.Query().Get("ns")
		_, _ = io.WriteString(w, "FROM-CACHE")
	}))
	defer cache.Close()

	router := NewRouter([]Route{{NS: "registry.d8-system.svc:5001", Mode: ModeCache, CacheURL: cache.URL}})
	h, err := NewHandler(router, stubAuth{ok: false}) // auth must be irrelevant for cache mode
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v2/x/manifests/latest?ns=registry.d8-system.svc:5001", nil)
	req.Header.Set("Authorization", "Basic Y2xpZW50OmNyZWRz")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if body, _ := io.ReadAll(rr.Result().Body); string(body) != "FROM-CACHE" {
		t.Fatalf("body = %q, want FROM-CACHE", body)
	}
	if gotAuth != "Basic Y2xpZW50OmNyZWRz" {
		t.Fatalf("cache did not receive client Authorization (pass-through): %q", gotAuth)
	}
	if gotNS != "registry.d8-system.svc:5001" {
		t.Fatalf("cache did not receive ns: %q", gotNS)
	}
}

// TestHandler_CacheTLSVerifiesCA asserts the cache route forwards over HTTPS
// trusting route.CacheCA: the cache serves a cert signed by its own (non-system)
// CA, so with CacheCA set the proxy succeeds, and without it the TLS handshake
// fails ("x509: certificate signed by unknown authority") → 502.
func TestHandler_CacheTLSVerifiesCA(t *testing.T) {
	cache := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "FROM-TLS-CACHE")
	}))
	defer cache.Close()

	caPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cache.Certificate().Raw}))

	t.Run("with CacheCA -> 200", func(t *testing.T) {
		router := NewRouter([]Route{{NS: "registry.d8-system.svc:5001", Mode: ModeCache, CacheURL: cache.URL, CacheCA: caPEM}})
		h, err := NewHandler(router, stubAuth{ok: false})
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v2/x/manifests/latest?ns=registry.d8-system.svc:5001", nil)
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		if body, _ := io.ReadAll(rr.Result().Body); string(body) != "FROM-TLS-CACHE" {
			t.Fatalf("body = %q, want FROM-TLS-CACHE", body)
		}
	})

	t.Run("without CacheCA -> 502 (untrusted cert)", func(t *testing.T) {
		router := NewRouter([]Route{{NS: "registry.d8-system.svc:5001", Mode: ModeCache, CacheURL: cache.URL}})
		h, err := NewHandler(router, stubAuth{ok: false})
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v2/x/manifests/latest?ns=registry.d8-system.svc:5001", nil)
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502 (untrusted cache cert must fail)", rr.Code)
		}
	})
}

func TestHandler_DirectRequiresLocalAuth(t *testing.T) {
	router := NewRouter([]Route{{NS: "docker.io", Mode: ModeDirect, Upstream: &Upstream{URL: "https://example.invalid"}}})
	h, _ := NewHandler(router, stubAuth{ok: false})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v2/?ns=docker.io", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if rr.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("missing WWW-Authenticate challenge on direct 401")
	}
}

func TestHandler_DirectInjectsUpstreamCredsAndStripsClientAuthAndNS(t *testing.T) {
	var sawClientAuth bool
	var sawNS string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawNS = r.URL.Query().Get("ns")
		// Upstream requires Basic user:pass; client creds (client:creds) must NOT appear.
		u, p, _ := r.BasicAuth()
		if u == "client" {
			sawClientAuth = true
		}
		if u != "user" || p != "pass" {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, "FROM-UPSTREAM")
	}))
	defer upstream.Close()

	router := NewRouter([]Route{{
		NS:       "docker.io",
		Mode:     ModeDirect,
		Upstream: &Upstream{URL: upstream.URL, Creds: &Credentials{Username: "user", Password: "pass"}},
	}})
	h, _ := NewHandler(router, stubAuth{ok: true})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v2/?ns=docker.io", nil)
	req.Header.Set("Authorization", "Basic "+basicAuth("client", "creds"))
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if body, _ := io.ReadAll(rr.Result().Body); string(body) != "FROM-UPSTREAM" {
		t.Fatalf("body = %q, want FROM-UPSTREAM", body)
	}
	if sawClientAuth {
		t.Fatal("upstream saw client credentials; they must be stripped")
	}
	if sawNS != "" {
		t.Fatalf("upstream saw ns param %q; it must be stripped", sawNS)
	}
}

func TestHandler_NoRoute(t *testing.T) {
	h, _ := NewHandler(NewRouter(nil), stubAuth{ok: true})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v2/?ns=unknown.io", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestHandler_DirectRewritesRepoPath(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		u, p, _ := r.BasicAuth()
		if u != "user" || p != "pass" {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, "OK")
	}))
	defer upstream.Close()

	router := NewRouter([]Route{{
		NS:   "registry.d8-system.svc:5001",
		Mode: ModeDirect,
		Upstream: &Upstream{
			URL:            upstream.URL,
			Creds:          &Credentials{Username: "user", Password: "pass"},
			LocalPathAlias: "system/deckhouse",
			RemotePath:     "deckhouse/ee",
		},
	}})
	h, _ := NewHandler(router, stubAuth{ok: true})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v2/system/deckhouse/module/manifests/v1?ns=registry.d8-system.svc:5001", nil)
	req.Header.Set("Authorization", "Basic "+basicAuth("client", "creds"))
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if gotPath != "/v2/deckhouse/ee/module/manifests/v1" {
		t.Fatalf("upstream path = %q, want /v2/deckhouse/ee/module/manifests/v1", gotPath)
	}
}

func TestHandler_DirectNilUpstreamReturns500(t *testing.T) {
	// Route with ModeDirect but Upstream == nil must return 500 without panicking.
	router := NewRouter([]Route{{NS: "nil.example.io", Mode: ModeDirect, Upstream: nil}})
	h, err := NewHandler(router, stubAuth{ok: true})
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v2/?ns=nil.example.io", nil)
	req.SetBasicAuth("user", "pass")
	// Must not panic.
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_RouterFuncResolvesPerRequest(t *testing.T) {
	// Build two different cache backends so we can tell which router is active.
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "BACKEND-1")
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "BACKEND-2")
	}))
	defer backend2.Close()

	router1 := NewRouter([]Route{{NS: "example.io", Mode: ModeCache, CacheURL: backend1.URL}})
	router2 := NewRouter([]Route{{NS: "example.io", Mode: ModeCache, CacheURL: backend2.URL}})

	// routerFn returns router1 on the first call, then router2, then nil.
	calls := 0
	routerFn := func() *Router {
		calls++
		switch calls {
		case 1:
			return router1
		case 2:
			return router2
		default:
			return nil
		}
	}

	h, err := NewHandlerFunc(routerFn, stubAuth{ok: true})
	if err != nil {
		t.Fatal(err)
	}

	// First request → router1 → BACKEND-1
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v2/?ns=example.io", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("call 1: status = %d, want 200", rr.Code)
	}
	if body, _ := io.ReadAll(rr.Result().Body); string(body) != "BACKEND-1" {
		t.Fatalf("call 1: body = %q, want BACKEND-1", body)
	}

	// Second request → router2 → BACKEND-2
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v2/?ns=example.io", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("call 2: status = %d, want 200", rr.Code)
	}
	if body, _ := io.ReadAll(rr.Result().Body); string(body) != "BACKEND-2" {
		t.Fatalf("call 2: body = %q, want BACKEND-2", body)
	}

	// Third request → nil router → 503
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v2/?ns=example.io", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("call 3 (nil router): status = %d, want 503", rr.Code)
	}

	// NewHandlerFunc must reject nil routerFn.
	_, err = NewHandlerFunc(nil, stubAuth{ok: true})
	if err == nil {
		t.Fatal("NewHandlerFunc(nil routerFn) must return error")
	}

	// NewHandlerFunc must reject nil auth.
	_, err = NewHandlerFunc(func() *Router { return nil }, nil)
	if err == nil {
		t.Fatal("NewHandlerFunc(nil auth) must return error")
	}
}
