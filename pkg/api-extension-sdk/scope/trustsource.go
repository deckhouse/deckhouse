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

package scope

import (
	"crypto/x509"
	"net/http"
	"slices"
	"strings"
)

// TrustConfig is what a plain net/http server needs to recognise kube-apiserver's
// front proxy. kube-apiserver publishes it in the ConfigMap
// kube-system/extension-apiserver-authentication.
type TrustConfig struct {
	// Roots is the requestheader client CA (key requestheader-client-ca-file).
	Roots *x509.CertPool
	// AllowedNames are the common names the front-proxy certificate may carry
	// (key requestheader-allowed-names). Empty trusts nothing.
	AllowedNames []string
	// UsernameHeaders are the headers carrying the asserted username (key
	// requestheader-username-headers). Empty trusts nothing.
	UsernameHeaders []string
}

// TrustSource returns the current TrustConfig. ok is false when none is
// available (not synced yet, ConfigMap missing or malformed); nothing is
// trusted then. It is called on every request carrying scope headers and must
// be cheap and safe for concurrent use.
type TrustSource interface {
	TrustConfig() (cfg TrustConfig, ok bool)
}

// StaticTrustSource is a TrustSource with a fixed TrustConfig, for tests and
// servers that load the CA from a file themselves.
type StaticTrustSource TrustConfig

func (s StaticTrustSource) TrustConfig() (TrustConfig, bool) { return TrustConfig(s), s.Roots != nil }

// NewTrustedHandler is WithTrusted for a plain net/http server. The server must
// terminate TLS itself and request client certificates (tls.Config.ClientAuth
// tls.RequestClientCert or stricter): a TLS-terminating sidecar breaks the
// check. The handler does not authenticate the caller for the application; it
// only decides whether the scope headers are honoured.
//
// A request is trusted when its leaf client certificate verifies against
// cfg.Roots for client auth (the other peer certificates are intermediates),
// its common name is in cfg.AllowedNames, and one of cfg.UsernameHeaders is
// not empty.
func NewTrustedHandler(h http.Handler, src TrustSource) http.Handler {
	return newTrustedHandler(h, sourceTrust(src))
}

func sourceTrust(src TrustSource) trustFunc {
	return func(r *http.Request) (string, bool, string) {
		if src == nil {
			return "", false, "no trust source"
		}
		cfg, ok := src.TrustConfig()
		if !ok || cfg.Roots == nil {
			return "", false, "no requestheader client CA"
		}
		if !anyNonEmpty(cfg.AllowedNames) {
			return "", false, "requestheader allowed names are empty"
		}
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			return "", false, "no client certificate"
		}
		leaf := r.TLS.PeerCertificates[0]
		opts := x509.VerifyOptions{
			Roots:     cfg.Roots,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		if len(r.TLS.PeerCertificates) > 1 {
			opts.Intermediates = x509.NewCertPool()
			for _, c := range r.TLS.PeerCertificates[1:] {
				opts.Intermediates.AddCert(c)
			}
		}
		if _, err := leaf.Verify(opts); err != nil {
			return "", false, err.Error()
		}
		if leaf.Subject.CommonName == "" || !slices.Contains(cfg.AllowedNames, leaf.Subject.CommonName) {
			return "", false, "common name " + leaf.Subject.CommonName + " is not allowed"
		}
		for _, name := range cfg.UsernameHeaders {
			if name = strings.TrimSpace(name); name == "" {
				continue
			}
			if user := r.Header.Get(name); user != "" {
				return user, true, ""
			}
		}
		return "", false, "empty username header"
	}
}
