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

// Package authproxy forwards a token request to the service that issues tokens.
//
// The token service listens on the loopback, which is the whole point: it is reachable from exactly
// one place, the process that needs it. Every client of this registry would otherwise have to reach
// it directly — a node agent on every node, `d8 mirror push` from outside the cluster — and that
// would mean a second address to publish, a second certificate to cover it and a second thing to
// get wrong.
//
// So the registry answers a challenge with its own address and forwards what comes back to it. The
// registry itself has no say in the answer: it does not read the request, does not mint anything and
// cannot widen a scope. What travels through here is the client's own credentials to the service that
// verifies them, and the token that service signed.
//
// Mounted in front of the registry rather than inside it, on one path. The registry's `autoredirect`
// with `autoredirectpath` — upstream options, not ours — is what makes clients ask for that path.
package authproxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/deckhouse/registry-distribution/internal/config"
)

// Path is where a client asks for a token, and what the registry's `autoredirectpath` must name.
//
// A constant shared with the configuration the syncer renders: the two have to agree, and a value
// spelled out in two places drifts.
const Path = "/auth/token"

// Handler mounts the token path in front of the registry.
//
// A nil AuthProxy leaves the registry exactly as it was: on a cluster whose registry needs no token
// service there is nothing to forward, and mounting a proxy to nowhere would answer a challenge
// with an address that refuses connections.
func Handler(settings *config.AuthProxy, registry http.Handler) (http.Handler, error) {
	if settings == nil || settings.URL == "" {
		return registry, nil
	}

	target, err := url.Parse(settings.URL)
	if err != nil {
		return nil, fmt.Errorf("the token service address %q cannot be parsed: %w", settings.URL, err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if settings.CA != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		pem, err := os.ReadFile(settings.CA)
		if err != nil {
			return nil, fmt.Errorf("reading the token service certificate authority %s: %w",
				settings.CA, err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s holds no certificate this can use", settings.CA)
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}

	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetXForwarded()

			// The scheme the CLIENT used, which is what the token service needs to know: it signs a
			// token for a service and an address, and behind an ingress the inbound hop is the one
			// that was https.
			if scheme := request.In.URL.Scheme; scheme == "http" || scheme == "https" {
				request.Out.Header.Set("X-Forwarded-Proto", scheme)
			}

			request.SetURL(target)
			// The service's own path, not the client's: the client asked this registry for a token
			// and the service serves it wherever it serves it.
			request.Out.URL.Path = target.Path

			// The Host the client used. The token service puts it in the token, and a token whose
			// audience is the loopback is a token no client can use.
			request.Out.Host = request.In.Host
		},
	}

	mounted := http.NewServeMux()
	mounted.Handle(Path, proxy)
	mounted.Handle("/", registry)

	return mounted, nil
}
