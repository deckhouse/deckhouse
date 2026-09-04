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
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

func testOptions() Options {
	return Options{
		ListenAddress: "10.0.0.1",
		HTTPSecret:    "shared-secret",
		AuthRealm:     "https://10.0.0.1:5051/auth",
		TokenIssuer:   "Registry server",
	}
}

func renderToMap(t *testing.T, spec *registryv1alpha1.RegistryStorageSpec) map[string]any {
	t.Helper()

	rendered, err := Render(spec, testOptions())
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(rendered, &parsed))
	return parsed
}

// renderWrapperToMap reads the registry image's own half of the configuration.
func renderWrapperToMap(t *testing.T, spec *registryv1alpha1.RegistryStorageSpec) map[string]any {
	t.Helper()

	rendered, err := RenderWrapper(spec, testOptions())
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(rendered, &parsed))
	return parsed
}

// TestRenderCarriesNoUpstreamAndNoWriteEndpoint is where the two halves of the configuration were
// split.
//
// Both sections used to be here, and both were the registry's own vocabulary only because this
// repository's fork of it had been taught them: `proxy.localpathalias`, `proxy.remotepathonly` and a
// whole `writeendpoint` section existed nowhere upstream. The registry image decides them now, from
// the file RenderWrapper produces, and what is left in this one is upstream's configuration in
// upstream's shape.
func TestRenderCarriesNoUpstreamAndNoWriteEndpoint(t *testing.T) {
	config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee"},
		},
	})

	assert.NotContains(t, config, "proxy",
		"which upstream a miss goes to is decided in this module's own file, and two answers "+
			"would be two opinions nobody reconciles")
	assert.NotContains(t, config, "writeendpoint")

	http, _ := config["http"].(map[string]any)
	assert.Equal(t, "10.0.0.1:5001", http["addr"])
	assert.Equal(t, "shared-secret", http["secret"])

	// The challenge points a client at this registry's own token path, which the registry then
	// forwards to the token service on the loopback. Upstream's own options, both of them.
	auth, _ := config["auth"].(map[string]any)
	token, _ := auth["token"].(map[string]any)
	assert.Equal(t, true, token["autoredirect"])
	assert.Equal(t, AuthTokenPath, token["autoredirectpath"])
}

func TestRenderWrapperCarriesTheUpstream(t *testing.T) {
	spec := &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTPS,
				Host:   "registry.deckhouse.io",
				Path:   "/deckhouse/ee",
				CA:     "-----BEGIN CERTIFICATE-----upstream",
				Auth:   &registryv1alpha1.Auth{Username: "license-token", Password: "key"},
			},
		},
	}

	config := renderWrapperToMap(t, spec)

	upstream, ok := config["upstream"].(map[string]any)
	require.True(t, ok, "a configured upstream must be handed to the registry image")
	assert.Equal(t, "registry.deckhouse.io", upstream["address"])
	assert.Equal(t, "https", upstream["scheme"])
	assert.Equal(t, "/deckhouse/ee", upstream["path"])
	assert.Equal(t, "license-token", upstream["username"])
	assert.Equal(t, "key", upstream["password"])
	assert.Equal(t, UpstreamCAFile, upstream["ca"])

	// The cluster refers to images under a fixed prefix, so the upstream address and path can
	// change without every image reference changing with them — and without the store being
	// re-laid on disk under a different name.
	assert.Equal(t, strings.Trim(LocalPathAlias, "/"), config["scope"])

	// The listener that accepts a push, and the authority whose client certificates may claim
	// where a push came from.
	write, ok := config["writeEndpoint"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, fmt.Sprintf("10.0.0.1:%d", WriteEndpointPort), write["address"])
	assert.Equal(t, IngressClientCAFile, write["clientCertCA"])

	// The token service on the loopback: reachable from this pod and nowhere else.
	proxy, ok := config["authProxy"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://"+AuthAddress+"/auth", proxy["url"])
}

// TestRenderAirGap covers the authoritative cache: nothing in the configuration can fetch, so the
// registry serves only what it holds.
//
// Absent, not empty: the registry image starts no loopback rewriter at all, and there is no address
// in any file that a miss could be sent to. That is what makes the store authoritative, and why
// completeness has to be established before the upstream is taken away.
func TestRenderAirGap(t *testing.T) {
	wrapper := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{})
	assert.NotContains(t, wrapper, "upstream",
		"without an upstream the cache must not be able to fetch anything")

	config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{})

	// Authentication is still on. An air-gapped cache serving reads to anyone would
	// be an open registry inside the cluster.
	auth, ok := config["auth"].(map[string]any)
	require.True(t, ok)
	token, ok := auth["token"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://10.0.0.1:5051/auth", token["realm"])
	assert.Equal(t, PKIDir+"/token.crt", token["rootcertbundle"])
}

// TestRenderAlwaysAuthenticates is the property stated as "authentication is never
// disabled": there must be no input at all that produces an open registry.
func TestRenderAlwaysAuthenticates(t *testing.T) {
	specs := map[string]*registryv1alpha1.RegistryStorageSpec{
		"air-gap":                {},
		"pass-through":           {Upstream: &registryv1alpha1.Upstream{Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io"}}},
		"pass-through with auth": {Upstream: &registryv1alpha1.Upstream{Endpoint: registryv1alpha1.Endpoint{Host: "r.example.com", Auth: &registryv1alpha1.Auth{Username: "u", Password: "p"}}}},
		"publishing":             {Publish: true},
		"syncing":                {NeedSync: true},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			config := renderToMap(t, spec)

			auth, ok := config["auth"].(map[string]any)
			require.True(t, ok, "no configuration may leave the registry unauthenticated")
			assert.Contains(t, auth, "token")

			http, _ := config["http"].(map[string]any)
			tls, ok := http["tls"].(map[string]any)
			require.True(t, ok, "the registry always serves over TLS")
			assert.Equal(t, PKIDir+"/distribution.crt", tls["certificate"])
		})
	}
}

func TestRenderUpstreamWithoutCredentials(t *testing.T) {
	config := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ce"},
		},
	})

	upstream, _ := config["upstream"].(map[string]any)
	assert.NotContains(t, upstream, "username",
		"empty credentials are not a misconfiguration: this is how the community edition is pulled")
	assert.NotContains(t, upstream, "password")
	assert.NotContains(t, upstream, "ca", "no certificate authority means no file to point at")
}

func TestRenderUpstreamWithPreEncodedCredentials(t *testing.T) {
	config := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Host: "registry.deckhouse.io",
				// Encoded here rather than written out as a literal: the pair this stands
				// for is what the assertions below name, and no line of this file then
				// reads as a real credential.
				Auth: &registryv1alpha1.Auth{
					Auth: base64.StdEncoding.EncodeToString([]byte("license-token:key")),
				},
			},
		},
	})

	// The registry image only understands a pair, so the pre-encoded form has to be decoded here
	// rather than passed through.
	upstream, _ := config["upstream"].(map[string]any)
	assert.Equal(t, "license-token", upstream["username"])
	assert.Equal(t, "key", upstream["password"])
}

func TestRenderInsecureUpstream(t *testing.T) {
	config := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: "registry.local:5000"},
		},
	})

	upstream, _ := config["upstream"].(map[string]any)
	assert.Equal(t, "registry.local:5000", upstream["address"])
	assert.Equal(t, "http", upstream["scheme"])
}

func TestRenderDefaultsTheSchemeToHTTPS(t *testing.T) {
	config := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io"},
		},
	})

	upstream, _ := config["upstream"].(map[string]any)
	assert.Equal(t, "https", upstream["scheme"],
		"an unset scheme must not silently downgrade to plain HTTP")
}

func TestRenderTrimsTheUpstreamPath(t *testing.T) {
	config := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee/"},
		},
	})

	upstream, _ := config["upstream"].(map[string]any)
	assert.Equal(t, "/deckhouse/ee", upstream["path"])
}

func TestRenderRejectsMissingInput(t *testing.T) {
	_, err := Render(nil, testOptions())
	assert.Error(t, err)

	_, err = Render(&registryv1alpha1.RegistryStorageSpec{}, Options{})
	assert.Error(t, err, "without a listen address the registry would bind nowhere")
}

func TestRenderIsDeterministic(t *testing.T) {
	spec := &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Host: "registry.deckhouse.io",
				Auth: &registryv1alpha1.Auth{Username: "u", Password: "p"},
			},
		},
	}

	first, err := Render(spec, testOptions())
	require.NoError(t, err)
	second, err := Render(spec, testOptions())
	require.NoError(t, err)

	// Change detection is by content, so an unstable rendering would restart the
	// registry on every pass and leave the storage permanently unavailable.
	assert.Equal(t, string(first), string(second))
}

func TestDecodeBasic(t *testing.T) {
	tests := []struct {
		name         string
		encoded      string
		wantUsername string
		wantPassword string
		wantOK       bool
	}{
		{name: "a pair", encoded: "dXNlcjpwYXNz", wantUsername: "user", wantPassword: "pass", wantOK: true},
		{name: "an empty password", encoded: "dXNlcjo=", wantUsername: "user", wantOK: true},
		{name: "a password containing a colon", encoded: "dXNlcjpwYTpzcw==", wantUsername: "user", wantPassword: "pa:ss", wantOK: true},
		{name: "empty", encoded: "", wantOK: false},
		{name: "not base64", encoded: "not base64!", wantOK: false},
		{name: "no separator", encoded: "dXNlcm5hbWU=", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			username, password, ok := decodeBasic(tt.encoded)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantUsername, username)
			assert.Equal(t, tt.wantPassword, password)
		})
	}
}

// TestTheTwoListenersDoNotShareAPort is a host-network constraint, not a formality.
//
// The storage pod is host-networked, so these are ports on the node, and the two listeners belong to
// one process: asking for the same one means the process fails to start with `address already in use`
// and the pod crash-loops with nothing in it about registries.
func TestTheTwoListenersDoNotShareAPort(t *testing.T) {
	config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{Publish: true})
	wrapper := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{Publish: true})

	http, _ := config["http"].(map[string]any)
	write, _ := wrapper["writeEndpoint"].(map[string]any)

	require.NotEmpty(t, http["addr"])
	require.NotEmpty(t, write["address"])
	assert.NotEqual(t, http["addr"], write["address"],
		"the serving listener and the write listener cannot share a node port")

	debug, _ := http["debug"].(map[string]any)
	assert.NotEqual(t, debug["addr"], write["address"],
		"nor can the write listener take the debug port, which is the same failure one port over")
}

// TestTheCacheKeepsProxyingWhileTheWriteEndpointExists is the circle this closes.
//
// A registry configured as a pull-through cache is read-only: docker distribution answers every write
// with UNSUPPORTED. Measured on a cluster: `d8 mirror push` failed on POST /v2/.../blobs/uploads/.
// Publication therefore used to turn the proxy off — which confined it to air-gap and made the cache
// stop caching exactly when the cluster still depended on it.
//
// Now the push arrives on a second listener of the same registry, which never proxies (the registry's
// own tests cover that), so the serving half keeps its cache whenever there is an upstream — air-gap
// transition window included.
func TestTheCacheKeepsProxyingWhileTheWriteEndpointExists(t *testing.T) {
	upstream := &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{
			Scheme: registryv1alpha1.SchemeHTTPS, Host: "registry.deckhouse.io", Path: "/deckhouse/ee",
		},
	}

	// While the cluster merely caches, the upstream is what makes a miss slower rather than fatal.
	caching := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{Upstream: upstream})
	assert.Contains(t, caching, "upstream", "a cache with an upstream must serve misses from it")

	// Publication no longer changes that: the push lands on the other listener.
	published := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: upstream, Publish: true, AirGapRequested: true,
	})
	assert.Contains(t, published, "upstream",
		"publishing must not stop the cache serving misses; the push has a listener of its own")
	assert.Contains(t, published, "writeEndpoint",
		"and that listener has to be configured for the push to arrive anywhere")

	// Nothing here turns off a store-wiping mode change any more: the flag that used to do it
	// belonged to this repository's fork of the registry, and upstream performs no such deletion.
	// What it once cost is on the record — twelve gigabytes, twice in one afternoon, two seconds
	// after a start.
	assert.NotContains(t, renderToMap(t, &registryv1alpha1.RegistryStorageSpec{Upstream: upstream}),
		"proxy")
}

// TestTheWriteListenerTrustsTheIngressAuthority covers the one path reachable from outside the
// cluster.
//
// What the client certificate decides is whose word about the real client address is taken: without it
// every push would appear to come from the ingress controller. It is not what admits the push — that
// is the token service and its ACL, the same for both listeners.
func TestTheWriteListenerTrustsTheIngressAuthority(t *testing.T) {
	config := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{Publish: true})

	write, ok := config["writeEndpoint"].(map[string]any)
	require.True(t, ok, "there is no write listener to reach through the ingress")
	assert.Equal(t, IngressClientCAFile, write["clientCertCA"])
}

// TestTheWriteEndpointIsAlwaysConfigured, because the syncer fills the store through it too.
//
// Filling through the serving address fills nothing, silently: before uploading a layer the client
// asks whether the destination already holds it, the cache answers yes by fetching it from the
// upstream, and the store is left with manifests naming blobs it does not have. Measured: 400 layers
// reported written, the store unchanged at 333 MB. So the listener exists on every replica, whether or
// not anything is published from outside.
func TestTheWriteEndpointIsAlwaysConfigured(t *testing.T) {
	for _, published := range []bool{false, true} {
		config := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{Publish: published})

		write, ok := config["writeEndpoint"].(map[string]any)
		require.True(t, ok, "no write listener, published=%v", published)
		assert.Contains(t, write["address"], ":5003", "published=%v", published)
	}
}

// TestTheServingListenerDoesNotTrustProxyHeaders keeps the ingress authority off the address the
// cluster pulls through.
//
// Only one listener is fronted by an ingress, and the registry gives that one its own trust in
// process. Trusting a forwarded client address on the other would be surface for nothing: nothing
// forwards to it, so anything claiming to would be lying.
func TestTheServingListenerDoesNotTrustProxyHeaders(t *testing.T) {
	for _, published := range []bool{false, true} {
		config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{Publish: published})

		http, _ := config["http"].(map[string]any)
		assert.NotContains(t, http, "realip",
			"the serving listener is not the one behind the ingress, published=%v", published)
	}
}

// TestTheUpstreamCALivesOnAWritableVolume pins a path whose only wrong value fails in a way
// that names nothing relevant.
//
// The syncer both writes this file and removes it — when a configuration stops carrying a
// certificate authority, the stale copy has to go. Placed under the PKI directory, which is a
// Secret mount and therefore read-only, the removal fails with "read-only file system", the
// pass never completes, the configuration is never written, and the registry beside it
// crash-loops on `open /config/config.yaml: no such file or directory`. Three symptoms, none
// of them mentioning a certificate authority or a volume.
func TestTheUpstreamCALivesOnAWritableVolume(t *testing.T) {
	assert.True(t, strings.HasPrefix(UpstreamCAFile, ConfigDir+"/"),
		"the syncer writes and removes this file, so it cannot live under a Secret mount")
	assert.False(t, strings.HasPrefix(UpstreamCAFile, PKIDir+"/"),
		"%s is mounted from a Secret and is read-only", PKIDir)

	// The rendered configuration has to point the registry at wherever it actually is.
	config := renderWrapperToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Host: "registry.deckhouse.io",
				CA:   "-----BEGIN CERTIFICATE-----upstream",
			},
		},
	})
	upstream, _ := config["upstream"].(map[string]any)
	assert.Equal(t, UpstreamCAFile, upstream["ca"])
}
