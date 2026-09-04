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

// Package realip decides whose word about the client's address is taken.
//
// The registry writes the client address into its own logs and, more importantly, into what the
// token service is told about a request. Behind an ingress the connection comes from the ingress
// controller, so the address that matters travels in `X-Forwarded-For` — and a header is exactly as
// trustworthy as whoever may set it. On a listener reachable from outside the cluster, that is
// anybody.
//
// So the header is honoured on one condition: the connection presented a client certificate signed
// by the authority this module issued to the ingress. Without a certificate, or with one from
// anywhere else, the header is ignored and the peer's own address stands. Nothing is refused —
// refusing here would turn a routing detail into an outage — the claim is simply not believed.
package realip

import (
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
)

// Handler wraps a handler with the trust decision.
//
// An empty authority disables the whole thing and the peer address is used as it comes. That is the
// serving listener, which nothing fronts: every request there arrives from a node on the cluster's
// own network.
func Handler(clientCertCA string, next http.Handler) (http.Handler, error) {
	// The port is filled in whatever the decision below: the registry parses RemoteAddr as
	// host:port, and a request that reached this process over a unix socket or a stripped address
	// would otherwise fail on the parse rather than on anything about it.
	withPort := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, _, err := net.SplitHostPort(request.RemoteAddr); err != nil {
			request.RemoteAddr = net.JoinHostPort(request.RemoteAddr, "0")
		}
		next.ServeHTTP(writer, request)
	})

	if clientCertCA == "" {
		return withPort, nil
	}

	pem, err := os.ReadFile(clientCertCA)
	if err != nil {
		return nil, fmt.Errorf("reading the client certificate authority %s: %w", clientCertCA, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%s holds no certificate this can use", clientCertCA)
	}

	trusted := handlers.ProxyHeaders(withPort)

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if presentedTrustedCertificate(request, pool) {
			trusted.ServeHTTP(writer, request)
			return
		}
		withPort.ServeHTTP(writer, request)
	}), nil
}

// presentedTrustedCertificate reports whether the peer proved it is the front this listener has.
//
// Verified against the configured authority and required to be a client certificate: a server
// certificate from the same authority is not a licence to speak for somebody else's address, and
// this module issues both.
func presentedTrustedCertificate(request *http.Request, pool *x509.CertPool) bool {
	if request.TLS == nil {
		return false
	}

	for _, certificate := range request.TLS.PeerCertificates {
		if certificate == nil {
			continue
		}
		if _, err := certificate.Verify(x509.VerifyOptions{
			Roots:     pool,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}); err != nil {
			continue
		}
		return true
	}

	return false
}
