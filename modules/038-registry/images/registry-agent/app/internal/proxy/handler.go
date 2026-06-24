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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// Authenticator validates local client credentials. A bcrypt-backed
// implementation is wired in a later step; this interface keeps the handler
// testable in isolation.
type Authenticator interface {
	Authenticate(user, password string) bool
}

// Handler routes registry requests by ns and forwards them to the cache
// (transparent pass-through) or directly to the upstream (local auth + upstream
// credential injection).
type Handler struct {
	routerFn func() *Router
	auth     Authenticator
}

// NewHandlerFunc builds a Handler that resolves the active Router per request
// by calling routerFn. This allows the controller to hot-swap routes at runtime
// without restarting the server. Returns an error if routerFn or auth is nil.
func NewHandlerFunc(routerFn func() *Router, auth Authenticator) (*Handler, error) {
	if routerFn == nil {
		return nil, fmt.Errorf("routerFn is nil")
	}
	if auth == nil {
		return nil, fmt.Errorf("authenticator is nil")
	}
	return &Handler{routerFn: routerFn, auth: auth}, nil
}

// NewHandler builds a Handler from a fixed router and a local authenticator.
func NewHandler(router *Router, auth Authenticator) (*Handler, error) {
	if router == nil {
		return nil, fmt.Errorf("router is nil")
	}
	return NewHandlerFunc(func() *Router { return router }, auth)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router := h.routerFn()
	if router == nil {
		http.Error(w, "no routes configured", http.StatusServiceUnavailable)
		return
	}
	// Token endpoint: a /auth* request carries no ns and arrives only because the
	// agent rewrote the cache's Bearer realm to point back at itself (see
	// proxyCache). Forward it to the cache's auth so a node's containerd can fetch
	// a token via the agent instead of the cache's unresolvable Service DNS.
	if isAuthPath(r.URL.Path) {
		h.proxyCacheAuth(w, r, router)
		return
	}
	ns := r.URL.Query().Get("ns")
	route, ok := router.Match(ns, r.Host, r.URL.Path)
	if !ok {
		http.Error(w, "no route for registry", http.StatusNotFound)
		return
	}
	switch route.Mode {
	case ModeCache:
		h.proxyCache(w, r, route)
	case ModeDirect:
		h.proxyDirect(w, r, route)
	default:
		http.Error(w, "unknown route mode", http.StatusInternalServerError)
	}
}

// proxyCache forwards transparently to the on-master cache, preserving the
// client Authorization and the ns param (the cache validates auth and reads ns).
func (h *Handler) proxyCache(w http.ResponseWriter, r *http.Request, route Route) {
	target, err := url.Parse(route.CacheURL)
	if err != nil {
		http.Error(w, "bad cache url", http.StatusInternalServerError)
		return
	}
	// The cache serves HTTPS with a cert signed by the module CA, which is not in
	// the system roots — trust route.CacheCA explicitly (else: x509 unknown
	// authority). Empty CacheCA falls back to the default transport.
	base, err := transportWithCA(route.CacheCA)
	if err != nil {
		http.Error(w, "bad cache ca", http.StatusInternalServerError)
		return
	}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
		},
		Transport:      base,
		ModifyResponse: rewriteAuthRealm(r.Host),
	}
	rp.ServeHTTP(w, r)
}

// proxyCacheAuth forwards a token (/auth*) request to the cache's auth endpoint.
// Token requests carry no ns, so they cannot be matched by the ns router; route
// them to any cache. The realm the client followed here was rewritten by
// proxyCache to the agent's own host, so the client reaches us (node-reachable)
// instead of the cache's in-cluster Service DNS.
func (h *Handler) proxyCacheAuth(w http.ResponseWriter, r *http.Request, router *Router) {
	route, ok := router.firstCacheRoute()
	if !ok {
		http.Error(w, "no cache route for auth", http.StatusNotFound)
		return
	}
	target, err := url.Parse(route.CacheURL)
	if err != nil {
		http.Error(w, "bad cache url", http.StatusInternalServerError)
		return
	}
	base, err := transportWithCA(route.CacheCA)
	if err != nil {
		http.Error(w, "bad cache ca", http.StatusInternalServerError)
		return
	}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
		},
		Transport: base,
	}
	rp.ServeHTTP(w, r)
}

// proxyDirect terminates local auth, then reverse-proxies to the real upstream,
// stripping the client Authorization and ns param and injecting upstream creds.
func (h *Handler) proxyDirect(w http.ResponseWriter, r *http.Request, route Route) {
	if route.Upstream == nil {
		http.Error(w, "direct route missing upstream", http.StatusInternalServerError)
		return
	}

	user, pass, ok := r.BasicAuth()
	if !ok || !h.auth.Authenticate(user, pass) {
		w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	target, err := url.Parse(route.Upstream.URL)
	if err != nil {
		http.Error(w, "bad upstream url", http.StatusInternalServerError)
		return
	}
	base, err := transportWithCA(route.Upstream.CA)
	if err != nil {
		http.Error(w, "bad upstream ca", http.StatusInternalServerError)
		return
	}

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Header.Del("Authorization")
			q := pr.Out.URL.Query()
			q.Del("ns")
			pr.Out.URL.RawQuery = q.Encode()
			pr.Out.URL.Path = rewriteRepoPath(pr.Out.URL.Path, route.Upstream.LocalPathAlias, route.Upstream.RemotePath)
			pr.Out.URL.RawPath = "" // let net/http re-derive the escaped form
		},
		Transport: newUpstreamAuthTransport(base, route.Upstream.Creds),
	}
	rp.ServeHTTP(w, r)
}

// transportWithCA returns an http transport trusting the given PEM CA bundle, or
// the default transport when caPEM is empty.
func transportWithCA(caPEM string) (http.RoundTripper, error) {
	def, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("default transport is not *http.Transport")
	}
	tr := def.Clone()
	if caPEM == "" {
		return tr, nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, fmt.Errorf("no valid certificates in CA PEM")
	}
	tr.TLSClientConfig = &tls.Config{RootCAs: pool}
	return tr, nil
}

// isAuthPath reports whether p is the registry token endpoint the agent proxies
// to the cache's auth (the path component of the cache's Bearer realm, /auth).
func isAuthPath(p string) bool {
	return p == "/auth" || strings.HasPrefix(p, "/auth/")
}

// rewriteAuthRealm returns a ReverseProxy ModifyResponse that rewrites the host
// of a Bearer `realm="…"` in a Www-Authenticate header to host (the address the
// client used to reach the agent). The cache advertises its in-cluster Service
// DNS as the realm, which a node's containerd (node DNS) cannot resolve; pointing
// the realm back at the agent lets the token fetch come through the agent. A
// no-op when there is no Www-Authenticate header (cache hits never 401).
func rewriteAuthRealm(host string) func(*http.Response) error {
	return func(resp *http.Response) error {
		const hdr = "Www-Authenticate"
		v := resp.Header.Get(hdr)
		if v == "" {
			return nil
		}
		if nv, changed := rewriteRealmHost(v, host); changed {
			resp.Header.Set(hdr, nv)
		}
		return nil
	}
}

// rewriteRealmHost replaces the host in the realm="<url>" token of a
// Www-Authenticate header value with newHost, preserving scheme + path. Returns
// the original value and false when there is no parseable realm URL.
func rewriteRealmHost(headerValue, newHost string) (string, bool) {
	const key = `realm="`
	i := strings.Index(headerValue, key)
	if i < 0 {
		return headerValue, false
	}
	start := i + len(key)
	rel := strings.IndexByte(headerValue[start:], '"')
	if rel < 0 {
		return headerValue, false
	}
	end := start + rel
	u, err := url.Parse(headerValue[start:end])
	if err != nil || u.Host == "" {
		return headerValue, false
	}
	u.Host = newHost
	return headerValue[:start] + u.String() + headerValue[end:], true
}
