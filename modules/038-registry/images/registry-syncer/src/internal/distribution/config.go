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

// Package distribution renders and applies the configuration of the registry
// process that actually serves images.
//
// This is the path that makes a credential or certificate change take effect
// while the Deckhouse operator is down: the configuration comes from a custom
// resource and is applied by this sidecar, never through a helm re-render. It is
// the mechanism behind the "expired upstream credentials on a cluster whose
// deckhouse pod cannot pull its own image" story.
package distribution

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// Paths inside the container. They are constants rather than settings because
// they are a contract with the pod spec, not something an operator tunes:
// `RegistryStorage.spec.store.path` names the location on the HOST, and the
// volume mount maps it here.
const (
	// DataDir is where the blobs live inside the container.
	DataDir = "/data"

	// AuthAddress is where the token service listens, on the loopback address of the
	// node the replica runs on.
	//
	// Loopback rather than the node address: nothing outside this pod talks to it
	// directly, because the registry proxies token requests on the client's behalf.
	AuthAddress = "127.0.0.1:5051"

	// PKIDir holds the serving certificate, the token bundle and the certificate
	// authorities.
	PKIDir = "/pki"

	// UpstreamCAFile is where the upstream certificate authority is written when
	// the configuration carries one. Written by this package alongside the config,
	// since the two have to change together.
	UpstreamCAFile = PKIDir + "/upstream-registry-ca.crt"

	// IngressClientCAFile is the authority the write path trusts. Mounted from the
	// storage PKI rather than written here: it is cluster material, not something
	// derived from the desired state.
	IngressClientCAFile = PKIDir + "/ingress-client-ca.crt"

	// LocalPathAlias is the repository prefix this registry serves under, and the
	// prefix every image reference in the cluster uses.
	LocalPathAlias = constant.Path
)

// Options are the parts of the rendering that come from the pod rather than from
// the custom resource.
type Options struct {
	// ListenAddress is the address the registry serves on.
	ListenAddress string

	// HTTPSecret signs the state carried in upload URLs. It has to be identical
	// across replicas, otherwise an upload started against one replica cannot be
	// finished against another.
	HTTPSecret string

	// AuthRealm is the token service the registry sends clients to.
	//
	// This replica's own address, because the token service runs beside it in the same
	// pod. Not the Service name: the clients are node agents in the host network, where
	// a cluster DNS name does not resolve.
	AuthRealm string

	// TokenIssuer names the issuer expected in tokens.
	TokenIssuer string

	// ReadOnly refuses writes while keeping reads working.
	//
	// Set for the duration of a garbage collection. Distribution's collector computes the
	// set of reachable blobs and then deletes the rest, so a blob uploaded between those
	// two steps is deleted — which is why the documented way to run it is against a store
	// nothing is writing to.
	//
	// Reads are unaffected, so a replica in this state still serves every image it holds.
	// What it cannot do is accept a `d8 mirror push`, or store the result of a cache miss
	// — and a miss simply becomes a pull from the upstream, which the agent already falls
	// back to.
	ReadOnly bool
}

// Render turns the desired storage state into a registry configuration.
//
// Authentication is always configured, including in an air-gapped cluster where
// the cache only serves reads. There is no code path that renders an open
// registry, so no configuration mistake can produce one.
func Render(spec *registryv1alpha1.RegistryStorageSpec, opts Options) ([]byte, error) {
	if spec == nil {
		return nil, fmt.Errorf("no storage spec to render")
	}
	if opts.ListenAddress == "" {
		return nil, fmt.Errorf("no listen address")
	}

	config := map[string]any{
		"version": "0.1",
		"log": map[string]any{
			"level": "info",
		},
		"storage": map[string]any{
			"filesystem": map[string]any{
				"rootdirectory": DataDir,
			},
			"delete":   map[string]any{"enabled": true},
			"redirect": map[string]any{"disable": true},
			"maintenance": map[string]any{
				// The background upload purge stays off. It would delete in-flight uploads
				// on its own schedule, which is a second thing removing data from the store
				// for reasons nobody asked about.
				"uploadpurging": map[string]any{"enabled": false},
				"readonly":      map[string]any{"enabled": opts.ReadOnly},
			},
		},
		"http": map[string]any{
			"addr":   fmt.Sprintf("%s:%d", opts.ListenAddress, constant.Port),
			"prefix": "/",
			"secret": opts.HTTPSecret,
			"debug": map[string]any{
				"addr": "127.0.0.1:5002",
				"prometheus": map[string]any{
					"enabled": true,
					"path":    "/metrics",
				},
			},
			"tls": map[string]any{
				"certificate": PKIDir + "/distribution.crt",
				"key":         PKIDir + "/distribution.key",
			},
		},
		"auth": map[string]any{
			"token": map[string]any{
				"realm":          opts.AuthRealm,
				"service":        "Deckhouse registry",
				"issuer":         opts.TokenIssuer,
				"rootcertbundle": PKIDir + "/token.crt",
				"autoredirect":   true,
				// The registry fetches the token itself, on the client's behalf, over
				// the loopback address.
				//
				// Without this the token service would have to be reachable by every
				// client that the registry is: node agents on every node, and whatever
				// pushes through the publication endpoint from outside the cluster. It
				// would mean a second address to publish, a second certificate to cover
				// it, and a second thing to get wrong. With it the token service listens
				// on loopback and is reachable from exactly one place — the process that
				// needs it.
				"proxy": map[string]any{
					"url": fmt.Sprintf("https://%s/auth", AuthAddress),
					"ca":  PKIDir + "/ca.crt",
				},
			},
		},
	}

	// The pass-through half. Absent means the cache is authoritative: it serves
	// only what it already holds, which is exactly what an air-gapped cluster
	// needs, and is also why completeness has to be decided before the upstream is
	// removed.
	if proxy := renderProxy(spec.Upstream); proxy != nil {
		config["proxy"] = proxy
	}

	if spec.Publish {
		// The publication endpoint is reachable from outside the cluster, so the
		// registry requires a client certificate from the authority the ingress
		// presents. Credentials alone would not be enough: this is the one path that
		// can REPLACE an image, and a leaked password must not be sufficient to use it.
		//
		// It doubles as the real client address, without which every write would look
		// as if it came from the ingress controller.
		http, _ := config["http"].(map[string]any)
		http["realip"] = map[string]any{
			"enabled": true,
			"clientcert": map[string]any{
				"ca": IngressClientCAFile,
			},
		}
	}

	rendered, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshalling the configuration: %w", err)
	}
	return rendered, nil
}

func renderProxy(upstream *registryv1alpha1.Upstream) map[string]any {
	if upstream == nil {
		return nil
	}

	scheme := upstream.Scheme
	if scheme == "" {
		scheme = registryv1alpha1.SchemeHTTPS
	}

	proxy := map[string]any{
		"remoteurl": fmt.Sprintf("%s://%s", scheme.Lower(), upstream.Host),
		// The upstream serves Deckhouse under its own prefix, while the cluster
		// refers to images under a fixed one. Rewriting here is what lets the
		// upstream address change without every image reference changing with it.
		"remotepathonly": strings.TrimRight(upstream.Path, "/"),
		"localpathalias": LocalPathAlias,
	}

	if !upstream.Auth.IsEmpty() {
		username, password := basicCredentials(upstream.Auth)
		proxy["username"] = username
		proxy["password"] = password
	}
	if upstream.CA != "" {
		proxy["ca"] = UpstreamCAFile
	}

	return proxy
}

// basicCredentials reduces the credentials to the pair the registry
// configuration accepts, decoding the pre-encoded form when that is all there is.
func basicCredentials(auth *registryv1alpha1.Auth) (string, string) {
	if auth.Username != "" || auth.Password != "" {
		return auth.Username, auth.Password
	}

	username, password, ok := decodeBasic(auth.Auth)
	if !ok {
		return "", ""
	}
	return username, password
}
