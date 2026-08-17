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

	// WriteEndpoint renders the configuration for the SECOND instance — the one behind the ingress
	// that `d8 mirror push` writes to — rather than for the one the cluster pulls through.
	//
	// Two instances over one data directory, because a single one cannot be both: docker distribution
	// refuses every write when it is configured as a pull-through cache (`POST /v2/.../blobs/uploads/`
	// answered `UNSUPPORTED`, measured). Publishing through the serving instance therefore meant
	// turning its cache off, which is why publication used to exist only in air-gap, and why the
	// bundle could never be pushed BEFORE the transition it was supposed to make safe.
	//
	// The difference is exactly two things: this one never proxies, and it demands a client
	// certificate. Everything else — storage path, authentication, the token service — is identical,
	// and identical on purpose: they are the same registry, seen from two sides.
	WriteEndpoint bool
}

// The ports of the write endpoint instance.
//
// Distinct from the serving instance's, and that is not a formality: the storage pod is host-networked,
// so these are ports on the NODE. Two instances asking for the same one means whichever starts second
// dies with `address already in use`, which is how the bundle host's own registry announced the same
// mistake earlier — `listen tcp 127.0.0.1:5511: bind: address already in use`.
const (
	// WriteEndpointPort is where the instance behind the ingress listens.
	WriteEndpointPort = 5003

	// writeEndpointDebugAddress is its debug and metrics listener, on loopback like the other one.
	writeEndpointDebugAddress = "127.0.0.1:5004"
)

func listenPort(opts Options) int {
	if opts.WriteEndpoint {
		return WriteEndpointPort
	}
	return constant.Port
}

func debugAddress(opts Options) string {
	if opts.WriteEndpoint {
		return writeEndpointDebugAddress
	}
	return "127.0.0.1:5002"
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
			"addr":   fmt.Sprintf("%s:%d", opts.ListenAddress, listenPort(opts)),
			"prefix": "/",
			"secret": opts.HTTPSecret,
			"debug": map[string]any{
				"addr": debugAddress(opts),
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
	//
	// And absent as soon as the cluster asks to become air-gapped, even while the
	// upstream is still held. A registry configured as a pull-through cache is
	// read-only by construction: docker distribution answers every write with
	// `UNSUPPORTED`. Since the publication endpoint appears at the moment air-gap is
	// requested, and the upstream is held until the cache is complete, keeping the
	// proxy on would close a circle with no way out of it — `d8 mirror push` is the
	// only way to fill the cache, the push is refused because the registry proxies,
	// and the proxy stays because the cache is not full. Measured on a cluster: the
	// push failed on `POST /v2/.../blobs/uploads/` with exactly that error.
	//
	// Nothing is lost by dropping it here. What a pass-through cache gives a node is
	// a miss served from the upstream; the node keeps that anyway, because its own
	// layout carries the upstream as a fallback backend until the transition
	// completes. And filling does not need it either: the syncer fills by writing.
	// Which instance this is, not whether anything is published: the write endpoint is its own
	// process now, so the serving instance goes on proxying whenever there is an upstream to proxy —
	// including inside the air-gap transition window, where the cache is exactly what keeps the
	// cluster working while the bundle arrives.
	if !opts.WriteEndpoint {
		if proxy := renderProxy(spec.Upstream); proxy != nil {
			config["proxy"] = proxy
		}
	}

	// Whatever the mode, this store is never wiped because the mode changed.
	//
	// The registry deletes everything under /docker when it starts in a mode other than the one that
	// last wrote the directory, deciding which mode that was by whether the proxy scheduler's state
	// file is present. Sound for a registry that owns its directory, and wrong here twice over: two
	// processes share this store — one proxying for reads, one accepting the writes that fill it — and
	// the expiry scheduler is patched out at build time, so the state file comes and goes for reasons
	// that have nothing to do with the data.
	//
	// Measured on `ly-mmc`: 3236 files and twelve gigabytes deleted two seconds after the write
	// instance started, twice in one afternoon, and once at the exact moment the upstream was dropped
	// for an air-gap — which left a three-master cluster with no images and nothing to pull them from.
	// The blobs a proxy wrote are the same blobs a local registry serves; what must not survive a mode
	// change is the scheduler's own state, and that is deleted either way.
	setSkipModeCleanup(config)

	if opts.WriteEndpoint {
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

// setSkipModeCleanup tells the registry to keep the store across a mode change.
//
// Written into the `proxy` section because that is where the registry reads it from, and it is read
// even where no proxying is configured — which is the case that mattered: the write instance never
// proxies, and it was the one deleting the store. An absent section is created holding nothing else,
// so the instance stays non-proxying exactly as before.
func setSkipModeCleanup(config map[string]any) {
	proxy, ok := config["proxy"].(map[string]any)
	if !ok {
		proxy = map[string]any{}
		config["proxy"] = proxy
	}
	proxy["skipmodecleanup"] = true
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
	// Delegated: the reduction lives on the type, so that everything which authenticates with an
	// Auth reads it the same way. It did not always — this decoded the combined form and the fill
	// did not — and the result was a cache serving images while every fill went out anonymous.
	return auth.BasicCredentials()
}
