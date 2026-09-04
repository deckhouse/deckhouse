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

	// ConfigDir is the writable directory the rendered configuration lives in. An
	// emptyDir shared with the registry: derived state, rewritten on every change.
	ConfigDir = "/config"

	// UpstreamCAFile is where the upstream certificate authority is written when the
	// configuration carries one, and removed when it stops carrying one.
	//
	// Under ConfigDir rather than PKIDir, which is where it used to be and could not work:
	// PKIDir is a Secret mount and therefore read-only, so every pass failed on removing a
	// stale copy — "read-only file system" — and with the pass never completing, the
	// configuration was never written and the registry beside it crash-looped on a file that
	// was never going to appear. Nothing in that error mentioned a certificate authority.
	//
	// This belongs beside the configuration in any case: the two are written by the same pass
	// and have to change together, and unlike the material in PKIDir it is not cluster
	// material — it is a copy of what the upstream configuration says.
	UpstreamCAFile = ConfigDir + "/upstream-registry-ca.crt"

	// IngressClientCAFile is the authority the write path trusts. Mounted from the
	// storage PKI rather than written here: it is cluster material, not something
	// derived from the desired state.
	IngressClientCAFile = PKIDir + "/ingress-client-ca.crt"

	// LocalPathAlias is the repository prefix this registry serves under, and the
	// prefix every image reference in the cluster uses.
	LocalPathAlias = constant.Path

	// WrapperConfigFile is this module's own half of the registry's configuration: the upstream a
	// cache miss goes to and the path it serves under, the write endpoint, the token service.
	//
	// A second file rather than more keys in the first, because the first is the upstream project's
	// configuration and is rendered in the shape upstream documents. Both are written by the same
	// pass, so they cannot describe two different moments.
	WrapperConfigFile = ConfigDir + "/deckhouse.yaml"

	// AuthTokenPath is where a client asks this registry for a token, and where the registry
	// forwards that request to the token service on the loopback.
	//
	// The same string the registry image mounts the forwarder on. Two spellings of one path would
	// answer a challenge with an address that serves nothing.
	AuthTokenPath = "/auth/token"
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

// WriteEndpointPort is the second address of the same registry: the one that accepts a push.
//
// A pull-through cache refuses every write (`POST /v2/.../blobs/uploads/` answered `UNSUPPORTED`,
// measured), and turning the cache off to accept one is not available either — the store is filled BY
// pushes while it still has an upstream to serve the cluster from. So the writes get their own
// listener, which the registry serves from the same process over the same storage with no proxy in
// front of it.
//
// A different port from the serving one, necessarily: the storage pod is host-networked, so these are
// ports on the NODE, and two listeners in one process cannot share one. It used to be a second
// container over the same data directory, with its own rendered configuration and a store-wide flag
// keeping each instance from deleting the other's data at startup.
const WriteEndpointPort = 5003

// debugAddress is the metrics and pprof listener, on loopback. Only the serving half has one: the
// write endpoint is the same process, and its metrics namespace is the same namespace.
const debugAddress = "127.0.0.1:5002"

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
				"addr": debugAddress,
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
				// The challenge names this registry's own address, and the request that follows it
				// is forwarded by the registry to the token service on the loopback. Upstream's own
				// options, both of them: the previous implementation carried a patch that hardcoded
				// this path.
				"autoredirect":     true,
				"autoredirectpath": AuthTokenPath,
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

	// No proxy section, and no write endpoint either. Both used to be rendered here, and both are
	// now decided by the registry image itself: the upstream a miss goes to, and the path it serves
	// the image set under, are in this module's own configuration file beside this one — see
	// RenderWrapper. What the registry reads from THIS file is upstream's own vocabulary, in the
	// shape upstream documents, so a version bump changes nothing on this side.
	//
	// Nothing was lost with the `skipmodecleanup` flag that used to be set here either: it turned off
	// a deletion the previous registry performed when it started in a mode other than the one that
	// last wrote the store — a feature of the fork, measured deleting twelve gigabytes twice in one
	// afternoon. Upstream has no such deletion, so there is nothing left to turn off.

	rendered, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshalling the configuration: %w", err)
	}
	return rendered, nil
}

// basicCredentials reduces the credentials to the pair the registry
// configuration accepts, decoding the pre-encoded form when that is all there is.
func basicCredentials(auth *registryv1alpha1.Auth) (string, string) {
	// Delegated: the reduction lives on the type, so that everything which authenticates with an
	// Auth reads it the same way. It did not always — this decoded the combined form and the fill
	// did not — and the result was a cache serving images while every fill went out anonymous.
	return auth.BasicCredentials()
}
