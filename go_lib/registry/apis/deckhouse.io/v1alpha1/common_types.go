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

package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// Scheme is the protocol used to reach a registry.
//
// Values match ModuleConfig/deckhouse `settings.registry.*.scheme` and
// constant.SchemeType so that a value can travel from ModuleConfig through the
// CRs down to the containerd drop-in without being rewritten.
//
// +kubebuilder:validation:Enum=HTTP;HTTPS
type Scheme string

const (
	SchemeHTTP  Scheme = "HTTP"
	SchemeHTTPS Scheme = "HTTPS"
)

// IsSecure reports whether TLS is used.
func (s Scheme) IsSecure() bool { return s != SchemeHTTP }

// Lower returns the scheme as it must appear in a URL and in a containerd
// hosts.toml host key ("https", "http").
func (s Scheme) Lower() string { return strings.ToLower(string(s)) }

// Auth holds registry credentials.
//
// Credentials are stored inline rather than behind a secretRef. This is a
// deliberate trade-off in favour of the agent staying self-sufficient: the
// agent must keep serving images while the API server is unreachable, working
// from its on-disk copy of a single RegistryNode, and there is nothing to
// dereference at that point.
//
// The consequence is that every object carrying an Auth must be free of
// per-node secrets, because RegistryNode is readable by the whole
// `system:nodes` group. See the module security documentation.
type Auth struct {
	// Username for basic authentication.
	// +optional
	Username string `json:"username,omitempty"`

	// Password for basic authentication.
	// +optional
	Password string `json:"password,omitempty"`

	// Auth is a pre-encoded base64("username:password"). Takes precedence over
	// Username/Password when set, and is the form containerd hosts.toml wants.
	// +optional
	Auth string `json:"auth,omitempty"`
}

// IsEmpty reports whether no credentials are set at all.
func (a *Auth) IsEmpty() bool {
	return a == nil || (a.Username == "" && a.Password == "" && a.Auth == "")
}

// Encoded returns the credentials in the base64("username:password") form used
// by containerd hosts.toml. It returns an empty string when no credentials are
// set.
func (a *Auth) Encoded() string {
	if a.IsEmpty() {
		return ""
	}
	if a.Auth != "" {
		return a.Auth
	}
	return base64.StdEncoding.EncodeToString([]byte(a.Username + ":" + a.Password))
}

// Endpoint is one reachable registry address with everything needed to talk to
// it. It is the shared shape behind Upstream, Mirror and RegistryNode backends,
// so a single rendering routine can turn any of them into a containerd
// hosts.toml entry.
type Endpoint struct {
	// Scheme to reach the registry with.
	// +kubebuilder:default=HTTPS
	// +optional
	Scheme Scheme `json:"scheme,omitempty"`

	// Host is the registry host, optionally with a port ("registry.example.com",
	// "registry.d8-system.svc:5001").
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`

	// Path is the repository prefix inside the registry ("/deckhouse/ee").
	// +optional
	Path string `json:"path,omitempty"`

	// CA is a PEM-encoded certificate authority bundle used to verify the
	// registry when Scheme is HTTPS. Empty means the system trust store.
	// +optional
	CA string `json:"ca,omitempty"`

	// Auth holds the credentials for this endpoint.
	// +optional
	Auth *Auth `json:"auth,omitempty"`
}

// Address returns "host/path" without a scheme, which is how an image
// reference names a repository.
func (e *Endpoint) Address() string {
	if e == nil {
		return ""
	}
	path := strings.Trim(e.Path, "/")
	if path == "" {
		return e.Host
	}
	return e.Host + "/" + path
}

// URL returns the scheme-qualified base URL of the endpoint.
func (e *Endpoint) URL() string {
	if e == nil {
		return ""
	}
	scheme := e.Scheme
	if scheme == "" {
		scheme = SchemeHTTPS
	}
	return fmt.Sprintf("%s://%s", scheme.Lower(), e.Address())
}

// UniqueKey identifies an endpoint for de-duplication. Credentials are
// deliberately excluded: two entries differing only by credentials still point
// at the same content.
func (e *Endpoint) UniqueKey() string {
	if e == nil {
		return ""
	}
	scheme := e.Scheme
	if scheme == "" {
		scheme = SchemeHTTPS
	}
	return string(scheme) + "|" + e.Address()
}

// Upstream is a registry Deckhouse pulls from, plus its HA mirrors.
type Upstream struct {
	Endpoint `json:",inline"`

	// Mirrors are additional addresses serving THE SAME content as the primary
	// endpoint, used for failover and load balancing. They are not separate
	// sources: the storage caches one de-duplicated set regardless of which
	// mirror it was pulled from.
	//
	// Meaningful only together with a primary endpoint.
	// +optional
	Mirrors []Endpoint `json:"mirrors,omitempty"`
}

// Endpoints returns the primary endpoint followed by its mirrors, in the order
// a client should try them.
func (u *Upstream) Endpoints() []Endpoint {
	if u == nil {
		return nil
	}
	out := make([]Endpoint, 0, len(u.Mirrors)+1)
	out = append(out, u.Endpoint)
	out = append(out, u.Mirrors...)
	return out
}

// StorageSource describes the image set the storage is expected to hold. It is
// what makes "the cache is full" a decidable question: without an expected
// count there is nothing to compare the write log against, and therefore no
// safe moment to drop the upstream.
type StorageSource struct {
	// BundleRef names the image set, e.g. the bundle pushed by `d8 mirror push`.
	// +optional
	BundleRef string `json:"bundleRef,omitempty"`

	// ExpectedDigests is how many distinct digests the set contains.
	// +kubebuilder:validation:Minimum=0
	// +optional
	ExpectedDigests int32 `json:"expectedDigests,omitempty"`
}
