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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

// AuthSecretRef names where credentials actually live.
//
// Used by every resource this module derives rather than receives: a RegistryNode, a
// RegistryStorage, and the recorded effective upstream carry this instead of the
// credentials themselves. The resources are cluster-scoped and the node agent's role is
// bound to the `system:nodes` group, so inline credentials there mean any node's kubelet
// can read every registry credential in the cluster through the API. A reference narrows
// that to one Secret, which is also the object kind that gets audited and encrypted at
// rest like a credential should be.
//
// Deliberately not used in RegistryConfig.spec or RegistryUpstream.spec: those are the
// inputs an operator writes, holding exactly what the ModuleConfig already holds in the
// clear, so a reference there would break the API without hiding anything.
type AuthSecretRef struct {
	// Name of the Secret in the module's namespace.
	Name string `json:"name"`

	// Key inside it. One Secret holds every credential the module resolved, keyed by
	// what it belongs to, so that a node reads one object rather than one per registry.
	Key string `json:"key"`
}

// IsEmpty reports whether the reference names nothing.
func (r *AuthSecretRef) IsEmpty() bool {
	return r == nil || r.Name == "" || r.Key == ""
}

// Auth holds registry credentials, or says where they are kept.
//
// Resolving a reference is the responsibility of whoever reads it, and so is persisting the
// result: the node agent must keep serving images while the API server is unreachable,
// working from its on-disk copy of the layout, and a reference is not dereferenceable at
// that point. It therefore resolves on receipt and caches what it resolved, never the
// reference.
type Auth struct {
	// SecretRef names where the credentials are kept, for the resources that must not
	// carry them. Mutually exclusive with the fields below: whoever resolves this
	// reference produces those.
	// +optional
	SecretRef *AuthSecretRef `json:"secretRef,omitempty"`

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

// IsEmpty reports whether no usable credentials are set.
//
// About the inline fields only, deliberately: an unresolved reference has nothing to write
// into a hosts.toml, and every caller of this asks in order to write one. Whether the
// reference names credentials elsewhere is a different question — NeedsResolution.
func (a *Auth) IsEmpty() bool {
	return a == nil || (a.Username == "" && a.Password == "" && a.Auth == "")
}

// AuthDigest identifies credentials without disclosing them.
//
// A digest rather than the credentials themselves, so that "have the credentials changed?"
// stays answerable from a record that is safe to keep: a rotated license under an unchanged
// address is exactly the change that has to be probed before a cluster is switched onto it.
// Salted with a fixed string so the value cannot be matched against a rainbow table of
// plausible tokens; it is compared only against another digest produced here.
func (a *Auth) AuthDigest() string {
	if a.IsEmpty() {
		return ""
	}
	sum := sha256.Sum256([]byte("deckhouse-registry-auth\x00" + a.Encoded()))
	return hex.EncodeToString(sum[:])
}

// NeedsResolution reports that the credentials live in a Secret and have not been read
// yet. Writing such an Auth anywhere it is expected to authenticate is a bug: the request
// would go out unauthenticated and fail with a 401 that says nothing about why.
func (a *Auth) NeedsResolution() bool {
	return a != nil && !a.SecretRef.IsEmpty() && a.IsEmpty()
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

// BasicCredentials returns the credentials as a username and a password, whichever form they
// arrived in.
//
// The canonical reader, and it is canonical for a reason measured on a cluster. Credentials reach
// this type in two shapes — as the pair, or as the base64("username:password") that a dockercfg
// carries — and for a while two consumers of the SAME upstream credential read them differently:
// the pass-through cache decoded the combined form, the fill did not. Nodes therefore pulled images
// through the cache perfectly well while every fill went out anonymous and came back
// `401 Unauthorized: Auth failed` on every repository, including the platform's own. Nothing in the
// status could say why, because from the fill's side the credential was simply absent.
//
// Anything that authenticates with an Auth must go through this. A second reader is how the
// asymmetry happened, and one function is the only thing that makes it impossible to happen again.
func (a *Auth) BasicCredentials() (string, string) {
	if a.IsEmpty() {
		return "", ""
	}
	if a.Username != "" || a.Password != "" {
		return a.Username, a.Password
	}

	decoded, err := base64.StdEncoding.DecodeString(a.Auth)
	if err != nil {
		return "", ""
	}
	username, password, found := strings.Cut(string(decoded), ":")
	if !found {
		return "", ""
	}
	return username, password
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
