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
// The registry writes the client address into its own logs, and the auth proxy passes it on to the
// token service as `X-Forwarded-For`, for that service's logs. Behind an ingress the connection comes from the ingress
// controller, so the address that matters travels in `X-Forwarded-For` — and a header is exactly as
// trustworthy as whoever may set it. On a listener reachable from outside the cluster, that is
// anybody.
//
// So the header is honoured on one condition: the connection presented a client certificate signed
// by the authority this module issued to the ingress AND issued to the ingress — the leaf's common
// name is the ingress's. Without a certificate, with one from anywhere else, or with one the same
// authority issued to somebody else, the header is ignored and the peer's own address stands.
// Nothing is refused here — who may connect at all is the listener's decision, made in the TLS
// handshake before any of this runs — the claim is simply not believed.
//
// The name matters because the authority alone does not single out the ingress. Every leaf this
// module's authority signs carries client authentication among its usages — the signer uses the
// default profile, which lists both server and client auth — so the storage's own serving
// certificate, which the syncer beside it presents when it writes to its own store, verifies as a
// client certificate exactly as well as the ingress's does. The syncer is not in front of anybody,
// and its header would be a claim about an address it never saw.
//
// Believing the ingress is not the same as the address being the operator's. The certificate proves
// that the header was set by the ingress controller; what the controller put into it is the
// controller's own configuration, and this process cannot see it. As upstream ingress-nginx renders
// its template, a controller left at its defaults sends the address of the connection it accepted,
// replacing whatever the client wrote: the client's own address, or that of a balancer in front of
// the controller. One that processes forwarded headers itself — `use-forwarded-headers`, which
// Deckhouse's `behindL7Proxy` turns on, with or without `compute-full-forwarded-for` — passes on
// what arrived, and then the leftmost entry, the one taken here, is whatever the client or the
// balancer in front of the controller wrote. No other header does better in both cases: X-Real-IP
// is the same address the controller settled on, and the rightmost entry of a computed list is the
// balancer's. So the address in the access log, and in the `X-Forwarded-For` the token service is
// sent, is exactly as trustworthy as the inlet of the controller that serves the publication
// endpoint. Nothing admits or refuses by it: the token service's ACL has no address rules, and
// `whitelistSourceRanges` is applied by the controller to the connection it accepted.
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
//
// trustedName is the common name of the one leaf whose header is believed: the front's. Required
// with an authority, because without it every leaf the authority ever signed would speak for its
// clients — see the package comment for why that is all of them.
func Handler(clientCertCA, trustedName string, next http.Handler) (http.Handler, error) {
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
	if trustedName == "" {
		return nil, fmt.Errorf("the client certificate authority %s is given without the name of "+
			"the front it issued a certificate to, and every leaf it signs is a client certificate",
			clientCertCA)
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
		if presentedTrustedCertificate(request, pool, trustedName) {
			trusted.ServeHTTP(writer, request)
			return
		}
		withPort.ServeHTTP(writer, request)
	}), nil
}

// presentedTrustedCertificate reports whether the peer proved it is the front this listener has.
//
// Verified against the configured authority, required to be a client certificate, and required to
// name the front. The usage check alone used to be described here as what kept the storage's own
// certificate out, and it never did: this module's authority signs every leaf for both server and
// client authentication, so a certificate it issued to anything is a client certificate. What
// singles out the ingress is the name it was issued under, and only this authority can put a name
// under itself — a leaf with the right name from anywhere else fails the verification first.
//
// The decision is the leaf's alone, and that is the whole point. TLS proves possession of a
// private key for PeerCertificates[0] and for nothing else in the list: everything after it is a
// chain the peer chose to send. Accepting any member of that chain — what this did until a review
// asked why — believed anybody who appended a certificate this authority had ever issued to
// somebody, their own key underneath it, which on this listener means anybody who can reach it.
//
// The rest of the list is still read, as intermediates, because that is what it is for: an
// authority that signs through an intermediate leaves the leaf unverifiable against the root on
// its own, and refusing that case would quietly stop believing a legitimate front.
func presentedTrustedCertificate(request *http.Request, pool *x509.CertPool, trustedName string) bool {
	if request.TLS == nil || len(request.TLS.PeerCertificates) == 0 {
		return false
	}

	leaf := request.TLS.PeerCertificates[0]
	if leaf == nil || leaf.Subject.CommonName != trustedName {
		return false
	}

	intermediates := x509.NewCertPool()
	for _, certificate := range request.TLS.PeerCertificates[1:] {
		if certificate == nil {
			continue
		}
		intermediates.AddCert(certificate)
	}

	_, err := leaf.Verify(x509.VerifyOptions{
		Roots:         pool,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})

	return err == nil
}
