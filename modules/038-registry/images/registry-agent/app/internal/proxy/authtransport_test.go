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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// bearerRegistry is an httptest server that requires a Bearer token obtained
// from its /token endpoint using Basic credentials user:pass.
func bearerRegistry(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "user" || p != "pass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"token":"good-token"}`)
	})
	mux.HandleFunc("/v2/img/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s/token",service="reg",scope="repository:img:pull"`, srv.URL))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, "MANIFEST")
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestUpstreamAuthTransport_BearerFlow(t *testing.T) {
	srv := bearerRegistry(t)
	rt := newUpstreamAuthTransport(srv.Client().Transport, &Credentials{Username: "user", Password: "pass"})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v2/img/manifests/latest", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "MANIFEST" {
		t.Fatalf("body = %q, want MANIFEST", body)
	}
}

func TestUpstreamAuthTransport_BasicFlow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "user" || p != "pass" {
			w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, "OK")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	rt := newUpstreamAuthTransport(srv.Client().Transport, &Credentials{Username: "user", Password: "pass"})
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v2/", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestUpstreamAuthTransport_PassesThroughNon401(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "OK") })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	rt := newUpstreamAuthTransport(srv.Client().Transport, nil)
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v2/", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestUpstreamAuthTransport_BearerRetryPreservesBody(t *testing.T) {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "user" || p != "pass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"token":"good-token"}`)
	})
	mux.HandleFunc("/v2/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s/token",service="reg",scope="repository:img:push"`, srv.URL))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(body)
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	rt := newUpstreamAuthTransport(srv.Client().Transport, &Credentials{Username: "user", Password: "pass"})

	// http.NewRequest with a non-nil body Reader sets GetBody automatically for
	// strings.NewReader (via http.NewRequestWithContext internally).
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v2/upload", strings.NewReader("PAYLOAD"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if req.GetBody == nil {
		t.Fatal("expected GetBody to be set by http.NewRequest for strings.NewReader body")
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != "PAYLOAD" {
		t.Fatalf("upstream received body %q, want PAYLOAD", got)
	}
}
