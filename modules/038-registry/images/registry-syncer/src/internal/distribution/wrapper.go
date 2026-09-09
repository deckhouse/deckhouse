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

package distribution

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// wrapperConfig is the registry image's own half of the configuration.
//
// The other half — Render — is the upstream project's, in upstream's vocabulary. This one holds
// what upstream has no place for, and it is deliberately small: four things, each of which the
// previous implementation carried as a patch against the registry's source.
type wrapperConfig struct {
	// Scope is the repository prefix the cluster pulls by, and the local half of the mapping below.
	Scope string `json:"scope"`

	// Upstream is where a cache miss goes. Absent means the store is authoritative — it serves what
	// it holds and nothing else, which is what an air-gapped cluster runs on, and why completeness
	// has to be decided before the upstream is removed.
	Upstream *wrapperUpstream `json:"upstream,omitempty"`

	// WriteEndpoint is the listener that accepts a push.
	WriteEndpoint wrapperWriteEndpoint `json:"writeEndpoint"`

	// AuthProxy is the token service this registry forwards token requests to.
	AuthProxy *wrapperAuthProxy `json:"authProxy,omitempty"`
}

type wrapperUpstream struct {
	Address  string `json:"address"`
	Scheme   string `json:"scheme,omitempty"`
	Path     string `json:"path,omitempty"`
	CA       string `json:"ca,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type wrapperWriteEndpoint struct {
	Address      string `json:"address,omitempty"`
	ClientCertCA string `json:"clientCertCA,omitempty"`
}

type wrapperAuthProxy struct {
	URL string `json:"url"`
	CA  string `json:"ca,omitempty"`
}

// RenderWrapper turns the desired storage state into the registry image's own configuration.
//
// The upstream section is the one that used to be `proxy` in the registry's own file, with two
// options that existed only in this repository's fork of it: `localpathalias` and `remotepathonly`.
// They are here now because the mapping they describe is done in front of the cache rather than
// inside it, which is what let the fork go away — the cluster's prefix stays the cluster's, the
// store's layout does not follow the upstream's path, and the registry is upstream's own code.
func RenderWrapper(spec *registryv1alpha1.RegistryStorageSpec, opts Options) ([]byte, error) {
	if spec == nil {
		return nil, fmt.Errorf("no storage spec to render")
	}
	if opts.ListenAddress == "" {
		return nil, fmt.Errorf("no listen address")
	}

	config := wrapperConfig{
		Scope: strings.Trim(LocalPathAlias, "/"),
		// Always, not only where something is published: the syncer fills the store through this
		// endpoint as well. Filling through the serving address fills nothing and fails silently —
		// before uploading a layer the client asks whether the destination already holds it, and a
		// cache answers yes by fetching it from the upstream, so the upload is skipped and the store
		// is left with manifests naming blobs it does not have (measured: 400 layers "written", the
		// store unchanged at 333 MB).
		WriteEndpoint: wrapperWriteEndpoint{
			Address: fmt.Sprintf("%s:%d", opts.ListenAddress, WriteEndpointPort),
			// The ingress's authority: this is the half an ingress fronts, and without it every
			// push would appear to come from the ingress controller rather than from the operator
			// who sent it.
			ClientCertCA: IngressClientCAFile,
		},
		// The token service listens on the loopback, so the registry is the only thing that can
		// reach it, and every client asks the registry instead.
		AuthProxy: &wrapperAuthProxy{
			URL: fmt.Sprintf("https://%s/auth", AuthAddress),
			CA:  PKIDir + "/ca.crt",
		},
	}

	if upstream := spec.Upstream; upstream != nil {
		scheme := upstream.Scheme
		if scheme == "" {
			scheme = registryv1alpha1.SchemeHTTPS
		}

		config.Upstream = &wrapperUpstream{
			Address: upstream.Host,
			Scheme:  scheme.Lower(),
			// The upstream serves the image set under its own prefix, while the cluster refers to
			// images under a fixed one. Keeping the two apart is what lets the upstream change —
			// a different edition, a different mirror — without every image reference in the
			// cluster changing and without the store being re-laid on disk.
			Path: strings.TrimRight(upstream.Path, "/"),
		}

		if !upstream.Auth.IsEmpty() {
			username, password := basicCredentials(upstream.Auth)
			config.Upstream.Username = username
			config.Upstream.Password = password
		}
		if upstream.CA != "" {
			config.Upstream.CA = UpstreamCAFile
		}
	}

	rendered, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshalling the registry image configuration: %w", err)
	}

	return rendered, nil
}
