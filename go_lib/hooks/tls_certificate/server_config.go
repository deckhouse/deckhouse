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

// Package tls_certificate also exposes server-side TLS configuration
// helpers that implement the project-wide "TLS profiles" standard
// (see go_lib/hooks/tls_certificate/README.md).
//
// Two profiles are supported:
//
//   - Category A — the client is exclusively another in-cluster
//     deckhouse-controlled component (kube-apiserver to admission/
//     conversion webhook or extension API server). The handshake floor is
//     TLS 1.3; CipherSuites are not configured because Go fixes the
//     suite list for TLS 1.3.
//
//   - Category B — heterogeneous clients (kube-rbac-proxy in front of
//     metrics endpoints, controller-manager metrics, custom HTTPS
//     surfaces). The handshake floor is TLS 1.2 and the cipher suite
//     allow-list contains only ECDHE + AEAD entries. Anything with RSA
//     key exchange, CBC, SHA1 or GOST is rejected by policy.
//
// kube-apiserver itself (Category C) and externally exposed services
// (Category D — ingress, dex via ingress, …) live outside this helper
// and follow their own standards.

package tls_certificate

import "crypto/tls"

// CategoryBCipherSuites is the deckhouse-wide allow-list of TLS 1.2
// cipher suites for servers whose clients we do not strictly control
// (Category B). Order matters only as a tie-breaker for the Go runtime;
// every entry in this list is ECDHE-keyed and uses an AEAD construction
// (GCM or ChaCha20-Poly1305). The list intentionally omits:
//
//   - any TLS_RSA_* (no Perfect Forward Secrecy);
//   - any *_CBC_* (Lucky13 family);
//   - any *_SHA suite without SHA256/SHA384 (SHA1 deprecated);
//   - any TLS_GOSTR341112_* / KUZNYECHIK / MAGMA (not implemented in
//     upstream Go; would silently drop out of the handshake).
var CategoryBCipherSuites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
}

// CategoryBCipherSuiteNames mirrors CategoryBCipherSuites in the
// symbolic form expected by k8s component-base / kube-rbac-proxy
// `--tls-cipher-suites` flag values.
var CategoryBCipherSuiteNames = []string{
	"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
	"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
}

// ApplyServerCategoryA mutates c in place to enforce the Category A
// profile (TLS 1.3-only). Existing `CipherSuites` (if any) are cleared:
// for TLS 1.3 the field is ignored by Go, but we drop it anyway so the
// validation test does not flag the configuration as ambiguous.
//
// All other fields of c (Certificates, GetCertificate, ClientAuth,
// NextProtos, …) are preserved.
func ApplyServerCategoryA(c *tls.Config) {
	c.MinVersion = tls.VersionTLS13
	c.CipherSuites = nil
}

// ApplyServerCategoryB mutates c in place to enforce the Category B
// profile (TLS 1.2 floor, ECDHE+AEAD cipher suites). A defensive copy of
// CategoryBCipherSuites is taken so callers cannot accidentally mutate
// the package-global allow-list.
//
// All other fields of c are preserved.
func ApplyServerCategoryB(c *tls.Config) {
	c.MinVersion = tls.VersionTLS12
	c.CipherSuites = append([]uint16(nil), CategoryBCipherSuites...)
}

// ServerOptionCategoryA returns ApplyServerCategoryA in the controller-
// runtime `func(*tls.Config)` form expected by
// `webhook.Options.TLSOpts` and `metrics/server.Options.TLSOpts`.
//
// Composing with other tlsOpts is left to the caller; ServerOptionCategoryA
// should run last so that MinVersion / CipherSuites cannot be downgraded
// by a previously appended option.
func ServerOptionCategoryA() func(*tls.Config) { return ApplyServerCategoryA }

// ServerOptionCategoryB returns ApplyServerCategoryB in the same shape.
func ServerOptionCategoryB() func(*tls.Config) { return ApplyServerCategoryB }
