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

func TestRenderPassThroughCache(t *testing.T) {
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

	config := renderToMap(t, spec)

	proxy, ok := config["proxy"].(map[string]any)
	require.True(t, ok, "a configured upstream must produce a proxy section")
	assert.Equal(t, "https://registry.deckhouse.io", proxy["remoteurl"])
	assert.Equal(t, "/deckhouse/ee", proxy["remotepathonly"])
	// The cluster refers to images under a fixed prefix, so the upstream address can
	// change without every image reference changing with it.
	assert.Equal(t, LocalPathAlias, proxy["localpathalias"])
	assert.Equal(t, "license-token", proxy["username"])
	assert.Equal(t, "key", proxy["password"])
	assert.Equal(t, UpstreamCAFile, proxy["ca"])

	http, _ := config["http"].(map[string]any)
	assert.Equal(t, "10.0.0.1:5001", http["addr"])
	assert.Equal(t, "shared-secret", http["secret"])
}

// TestRenderAirGap covers the authoritative cache: no proxy section at all, so the
// registry serves only what it holds.
func TestRenderAirGap(t *testing.T) {
	config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{})

	assert.NotContains(t, config, "proxy",
		"without an upstream the cache must not be able to fetch anything")

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
	config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ce"},
		},
	})

	proxy, _ := config["proxy"].(map[string]any)
	assert.NotContains(t, proxy, "username")
	assert.NotContains(t, proxy, "password")
	assert.NotContains(t, proxy, "ca", "no certificate authority means no file to point at")
}

func TestRenderUpstreamWithPreEncodedCredentials(t *testing.T) {
	config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{
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

	// The registry configuration only understands a pair, so the pre-encoded form
	// has to be decoded here rather than passed through.
	proxy, _ := config["proxy"].(map[string]any)
	assert.Equal(t, "license-token", proxy["username"])
	assert.Equal(t, "key", proxy["password"])
}

func TestRenderInsecureUpstream(t *testing.T) {
	config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Scheme: registryv1alpha1.SchemeHTTP, Host: "registry.local:5000"},
		},
	})

	proxy, _ := config["proxy"].(map[string]any)
	assert.Equal(t, "http://registry.local:5000", proxy["remoteurl"])
}

func TestRenderDefaultsTheSchemeToHTTPS(t *testing.T) {
	config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io"},
		},
	})

	proxy, _ := config["proxy"].(map[string]any)
	assert.Equal(t, "https://registry.deckhouse.io", proxy["remoteurl"],
		"an unset scheme must not silently downgrade to plain HTTP")
}

func TestRenderTrimsTheUpstreamPath(t *testing.T) {
	config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee/"},
		},
	})

	proxy, _ := config["proxy"].(map[string]any)
	assert.Equal(t, "/deckhouse/ee", proxy["remotepathonly"])
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

// TestRenderPublicationRequiresAClientCertificate covers the write path. It is the
// one path that can replace an image, and it is reachable from outside the cluster,
// so credentials alone must not be enough to use it.
func TestRenderPublicationRequiresAClientCertificate(t *testing.T) {
	config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{Publish: true})

	http, _ := config["http"].(map[string]any)
	realip, ok := http["realip"].(map[string]any)
	require.True(t, ok, "the write path must be gated on a client certificate")
	assert.Equal(t, true, realip["enabled"])

	clientcert, _ := realip["clientcert"].(map[string]any)
	assert.Equal(t, IngressClientCAFile, clientcert["ca"])
}

// TestRenderWithoutPublicationHasNoWritePath keeps the client-certificate trust off
// where there is no endpoint to protect: trusting an authority that is not used is
// surface for nothing.
func TestRenderWithoutPublicationHasNoWritePath(t *testing.T) {
	config := renderToMap(t, &registryv1alpha1.RegistryStorageSpec{Publish: false})

	http, _ := config["http"].(map[string]any)
	assert.NotContains(t, http, "realip")
}
