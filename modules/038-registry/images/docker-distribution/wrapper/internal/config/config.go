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

// Package config is this wrapper's own configuration: what the registry needs that the registry's
// own configuration has no place for.
//
// Kept in a second file rather than as extra keys in the first, and that is the point of it. The
// first file is the upstream project's configuration, rendered in the shape upstream documents, so
// a reader can compare it against upstream and a version bump changes nothing here. Everything this
// module adds — where the token service listens, whose client certificates may claim a real client
// address, the prefix the cluster pulls by and the upstream it maps onto — lives here, in one place,
// under names this repository owns.
//
// Both files are written by the syncer in the same pass, so they cannot disagree about the moment
// they describe.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Wrapper is the whole of this module's own configuration.
type Wrapper struct {
	// Scope is the repository prefix the cluster pulls by — the local half of the mapping. Every
	// request under it is answered from the store, and on a miss fetched from Upstream.
	//
	// A constant of the design rather than a preference: image references in the cluster name this
	// prefix, so it must not follow the upstream's own path. See the Upstream field.
	Scope string `yaml:"scope"`

	// Upstream is where a cache miss goes. Absent means the store is authoritative: it serves what
	// it holds and nothing else, which is what an air-gapped cluster runs on.
	Upstream *Upstream `yaml:"upstream,omitempty"`

	// WriteEndpoint is the second listener, the one that is not a cache.
	WriteEndpoint WriteEndpoint `yaml:"writeEndpoint"`

	// AuthProxy is the token service this registry fetches tokens from on its clients' behalf.
	AuthProxy *AuthProxy `yaml:"authProxy,omitempty"`
}

// Upstream is the registry a cache miss is fetched from, and the path it serves the image set
// under.
//
// Path is why this configuration exists at all. The cluster refers to images by Scope, the upstream
// serves them under its own prefix, and the two must be free to differ: an operator changing the
// upstream — a different edition, a different mirror, a different company — must not change what
// every image reference in the cluster says, and must not re-lay the store on disk. So the mapping
// is done in front of the cache, by a loopback rewriter this wrapper runs, and the cache itself
// only ever sees names that are already the cluster's.
type Upstream struct {
	// Address is the upstream's host, with an optional port.
	Address string `yaml:"address"`

	// Scheme is http or https. Empty means https.
	Scheme string `yaml:"scheme,omitempty"`

	// Path is the prefix the upstream serves the image set under, without a trailing slash.
	Path string `yaml:"path,omitempty"`

	// CA is a file holding the authority that signs the upstream's certificate, when it is not one
	// the system trusts.
	CA string `yaml:"ca,omitempty"`

	// Username and Password authenticate to the upstream. Empty is not a misconfiguration: this is
	// how the community edition is pulled.
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// WriteEndpoint is the listener that accepts a push.
//
// Its own listener because the serving one is a pull-through cache, and a cache refuses every write
// — measured, and it is upstream's deliberate behaviour: the proxy stores answer a write with
// UNSUPPORTED. A store that is filled BY writes while it still has an upstream to serve from
// therefore needs a listener with no cache in front of it, over the same directory.
type WriteEndpoint struct {
	// Address is where it listens, in the same form as the registry's own `http.addr`. Empty means
	// this process serves reads only.
	Address string `yaml:"address,omitempty"`

	// ClientCertCA is the authority whose client certificates are trusted to carry the real client
	// address on this listener. It is the one endpoint reached through an ingress, so without it
	// every push would appear to come from the ingress controller rather than from the operator.
	ClientCertCA string `yaml:"clientCertCA,omitempty"`
}

// AuthProxy is the token service, and the reason it needs proxying.
//
// The token service listens on the loopback, so it is reachable from exactly one place: this
// process. Every client of this registry — a node agent on every node, `d8 mirror push` from
// outside the cluster — would otherwise need to reach it directly, which would mean a second
// address to publish, a second certificate to cover it, and a second thing to get wrong. So the
// registry forwards the token request instead.
type AuthProxy struct {
	// URL is the token service, including the path it serves.
	URL string `yaml:"url"`

	// CA is the authority that signs its certificate.
	CA string `yaml:"ca,omitempty"`
}

// Load reads the wrapper's configuration.
func Load(path string) (*Wrapper, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	wrapper := &Wrapper{}
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	// Strict: an option this build does not know is a configuration written for another one, and
	// carrying on with it silently ignored is how a cluster ends up serving something nobody asked
	// for. The syncer renders this file, so a mismatch means the two halves of one image disagree.
	decoder.KnownFields(true)
	if err := decoder.Decode(wrapper); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}

	if err := wrapper.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return wrapper, nil
}

// Validate refuses what cannot work, rather than starting and failing per request.
func (w *Wrapper) Validate() error {
	if strings.Trim(w.Scope, "/") == "" {
		return fmt.Errorf("scope is required: it is the prefix the cluster pulls by")
	}

	if w.Upstream != nil {
		if w.Upstream.Address == "" {
			return fmt.Errorf("upstream.address is required when an upstream is configured")
		}
		switch strings.ToLower(w.Upstream.Scheme) {
		case "", "http", "https":
		default:
			return fmt.Errorf("upstream.scheme %q is neither http nor https", w.Upstream.Scheme)
		}
	}

	if w.AuthProxy != nil && w.AuthProxy.URL == "" {
		return fmt.Errorf("authProxy.url is required when a token service is configured")
	}

	if w.WriteEndpoint.ClientCertCA != "" && w.WriteEndpoint.Address == "" {
		return fmt.Errorf("writeEndpoint.clientCertCA is set without writeEndpoint.address, " +
			"so nothing would present it")
	}

	return nil
}

// LocalPrefix is the request path prefix the cluster's own names live under.
func (w *Wrapper) LocalPrefix() string {
	return "/v2/" + strings.Trim(w.Scope, "/")
}

// RemotePrefix is what LocalPrefix is rewritten to on the way out.
func (w *Wrapper) RemotePrefix() string {
	if w.Upstream == nil {
		return ""
	}
	path := strings.Trim(w.Upstream.Path, "/")
	if path == "" {
		return "/v2"
	}
	return "/v2/" + path
}
