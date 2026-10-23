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

package provider

import (
	"context"
	"net/http"
)

const (
	// RealIPHeader is the header Dex reads the address of the client from
	// (web.clientRemoteIP.header): its rate limit counts the failed sign-ins of each address.
	RealIPHeader = "X-Real-IP"
	// TrustedProxyHeader carries the secret Dex trusts RealIPHeader on
	// (web.clientRemoteIP.trustedProxySecret). Without it Dex counts the connection address.
	TrustedProxyHeader = "X-Deckhouse-Trusted-Proxy"
)

type clientAddressKey struct{}

// WithClientAddress returns a copy of ctx that carries the address of the end client whose
// credentials are checked. A provider that goes through Dex sends it in RealIPHeader, so that Dex
// counts the failures of that client and not those of the proxy, which every basic-auth client
// shares.
func WithClientAddress(ctx context.Context, address string) context.Context {
	return context.WithValue(ctx, clientAddressKey{}, address)
}

// ClientAddress returns the address WithClientAddress put in ctx, or "".
func ClientAddress(ctx context.Context) string {
	address, _ := ctx.Value(clientAddressKey{}).(string)
	return address
}

// dexTransport tells Dex who the end client is. To a request for host - the Dex Service inside the
// cluster - whose context carries a client address (WithClientAddress), it adds RealIPHeader and,
// when a secret is configured, TrustedProxyHeader with it. A request for any other host goes as it
// is, so the secret never leaves the cluster. The oauth2 and oidc packages send their requests with
// the context of the call.
type dexTransport struct {
	base               http.RoundTripper
	host               string
	trustedProxySecret string
}

// RoundTrip implements http.RoundTripper.
func (t dexTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	address := ClientAddress(r.Context())
	if t.host == "" || r.URL.Host != t.host || address == "" {
		return t.base.RoundTrip(r)
	}
	// A RoundTripper must not change the request it is given.
	r = r.Clone(r.Context())
	r.Header.Set(RealIPHeader, address)
	if t.trustedProxySecret != "" {
		r.Header.Set(TrustedProxyHeader, t.trustedProxySecret)
	}
	return t.base.RoundTrip(r)
}
