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

// Package upstream is the registry the pull-through cache believes it is talking to.
//
// It exists to keep one thing out of the cache: the upstream's own repository path. The cluster
// refers to images by a fixed prefix — `system/deckhouse` — while an upstream serves them under
// whatever it likes, `deckhouse/ee` or a mirror's own layout, and those two must be free to differ.
// If the cache fetched by the upstream's name it would also STORE by it, and then changing the
// upstream — a different edition, a different mirror — would re-lay the whole store on disk and
// invalidate every image reference in the cluster.
//
// The previous implementation solved this by patching the registry: `localpathalias` and
// `remotepathonly`, two options carried against upstream for years, in the middle of the proxy's own
// name resolution. This does it from outside, in the one place where a path may be rewritten without
// anybody's storage layout depending on it: a listener on the loopback, which the cache is
// configured to treat as its upstream, and which rewrites the prefix on the way out and adds the
// credentials and the certificate authority the real upstream needs.
//
// What the cache sees is therefore an upstream that already speaks the cluster's names, and the
// registry stays the upstream project's, unmodified.
package upstream

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/deckhouse/registry-distribution/internal/config"
)

// Rewriter is the loopback upstream.
type Rewriter struct {
	// Address is where it listens, which is what the cache's `proxy.remoteurl` names. The loopback
	// only: this is an internal detail of one process, and anything else would publish a path to
	// the upstream that bypasses the cache's own authentication.
	Address string

	// Local and Remote are the request path prefixes being mapped. Remote may be shorter, longer or
	// equal; equal makes this a plain forwarder, which is the ordinary case for a mirror that
	// serves the same layout.
	Local  string
	Remote string

	// Target is the real upstream: scheme and host.
	Target url.URL

	// Username and Password are sent as basic credentials when set. A registry that answers with a
	// bearer challenge gets them from the cache's own challenge handling instead — this only
	// covers the upstreams that ask for basic, which is why it is not an error to leave them empty.
	Username string
	Password string

	// RootCAs verifies the upstream, nil meaning the system trust store.
	RootCAs *x509.CertPool

	Log *slog.Logger
}

// New builds the rewriter from the wrapper's configuration, or nil when no upstream is configured.
func New(wrapper *config.Wrapper, address string, log *slog.Logger) (*Rewriter, error) {
	if wrapper.Upstream == nil {
		return nil, nil
	}

	scheme := strings.ToLower(wrapper.Upstream.Scheme)
	if scheme == "" {
		scheme = "https"
	}

	rewriter := &Rewriter{
		Address:  address,
		Local:    wrapper.LocalPrefix(),
		Remote:   wrapper.RemotePrefix(),
		Target:   url.URL{Scheme: scheme, Host: wrapper.Upstream.Address},
		Username: wrapper.Upstream.Username,
		Password: wrapper.Upstream.Password,
		Log:      log,
	}

	if wrapper.Upstream.CA != "" {
		pem, err := os.ReadFile(wrapper.Upstream.CA)
		if err != nil {
			return nil, fmt.Errorf("reading the upstream certificate authority %s: %w",
				wrapper.Upstream.CA, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s holds no certificate this can use", wrapper.Upstream.CA)
		}
		rewriter.RootCAs = pool
	}

	return rewriter, nil
}

// Handler is the reverse proxy itself.
//
// Redirects are passed back untouched, which matters more than it looks: a registry answering a blob
// request with a redirect to object storage is the ordinary case for the large layers, the address it
// names is absolute, and the cache follows it directly. Rewriting it here would send those bytes
// through this process for no reason.
func (r *Rewriter) Handler() http.Handler {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if r.RootCAs != nil {
		transport.TLSClientConfig = &tls.Config{RootCAs: r.RootCAs, MinVersion: tls.VersionTLS12}
	}

	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Director: func(request *http.Request) {
			request.URL.Scheme = r.Target.Scheme
			request.URL.Host = r.Target.Host
			// The Host header follows the URL, or a registry serving several names answers for the
			// wrong one — and a token service signs a scope for it.
			request.Host = r.Target.Host

			request.URL.Path = r.rewrite(request.URL.Path)

			if r.Username != "" || r.Password != "" {
				request.SetBasicAuth(r.Username, r.Password)
			}
		},
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			// Reported as a gateway failure rather than as this registry's own error: what failed
			// is the hop to the upstream, and the cache decides what to do about it.
			if r.Log != nil {
				r.Log.Warn("the upstream did not answer",
					"upstream", r.Target.Host, "path", request.URL.Path, "error", err.Error())
			}
			writer.WriteHeader(http.StatusBadGateway)
		},
	}

	return proxy
}

// rewrite maps one request path from the cluster's names to the upstream's.
//
// Only the prefix, and only when it is the whole first segment run: `/v2/` itself — the endpoint
// every client checks first, and the one the cache pings to establish a challenge — is left alone,
// as is anything outside the scope. A request that does not match is forwarded unchanged rather
// than refused: it is the cache asking, and the cache asks about nothing this process does not
// serve.
func (r *Rewriter) rewrite(path string) string {
	if r.Remote == "" || r.Local == r.Remote {
		return path
	}

	if path == r.Local {
		return r.Remote
	}
	if after, found := strings.CutPrefix(path, r.Local+"/"); found {
		return r.Remote + "/" + after
	}

	return path
}

// Serve runs the rewriter until the listener is closed.
func (r *Rewriter) Serve(listener net.Listener) error {
	server := &http.Server{
		Handler: r.Handler(),
		// Generous: this is on the path of every cache miss, and a large layer from a slow upstream
		// is not a failure. The cache has its own timeout for a miss.
		ReadHeaderTimeout: 30 * time.Second,
	}
	return server.Serve(listener)
}
