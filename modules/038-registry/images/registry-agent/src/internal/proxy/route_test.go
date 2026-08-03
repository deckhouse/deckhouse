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

package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

const self = "127.0.0.1:5001"

func layout() *registryv1alpha1.RegistryNodeSpec {
	return &registryv1alpha1.RegistryNodeSpec{
		Cache: true,
		Backends: []registryv1alpha1.Backend{
			{
				Name: registryv1alpha1.BackendStorage,
				Endpoint: registryv1alpha1.Endpoint{
					Scheme: registryv1alpha1.SchemeHTTPS,
					Host:   constant.Host,
					Path:   constant.Path,
					CA:     "storage-ca",
					Auth:   &registryv1alpha1.Auth{Username: "registry-ro", Password: "read-secret"},
				},
			},
			{
				Name: registryv1alpha1.BackendUpstream,
				Endpoint: registryv1alpha1.Endpoint{
					Scheme: registryv1alpha1.SchemeHTTPS,
					Host:   "registry.deckhouse.io",
					Path:   "/deckhouse/ee",
					Auth:   &registryv1alpha1.Auth{Username: "license-token", Password: "license-key"},
				},
			},
		},
		AdditionalRoutes: []registryv1alpha1.Route{{
			Match: "images.virtualization.example.com",
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTPS,
				Host:   "vendor.example.com",
				Path:   "/virt",
				CA:     "vendor-ca",
				Auth:   &registryv1alpha1.Auth{Username: "vendor", Password: "vendor-secret"},
			},
		}},
	}
}

// TestResolvePrimarySwapsThePrefixPerBackend is the mechanism that lets the upstream
// address change without every image reference in the cluster changing with it: the
// cluster names images under a fixed prefix, and each backend serves them under its
// own.
func TestResolvePrimarySwapsThePrefixPerBackend(t *testing.T) {
	decision, err := Resolve(
		constant.Host, "/v2/system/deckhouse/registry-controller/manifests/v1.76.6", layout(), self)
	require.NoError(t, err)

	assert.Equal(t, KindPrimary, decision.Kind)
	require.Len(t, decision.Targets, 2)

	// The cache first. Its own prefix is the one the cluster uses, so nothing changes.
	cache := decision.Targets[0]
	assert.Equal(t, string(registryv1alpha1.BackendStorage), cache.Name)
	assert.Equal(t, constant.Host, cache.Host)
	assert.Equal(t, "/v2/system/deckhouse/registry-controller/manifests/v1.76.6", cache.Path)
	assert.Equal(t, "read-secret", cache.Auth.Password)
	assert.Equal(t, "storage-ca", cache.CA)

	// The upstream behind it, where the same image lives under a different prefix.
	upstream := decision.Targets[1]
	assert.Equal(t, string(registryv1alpha1.BackendUpstream), upstream.Name)
	assert.Equal(t, "/v2/deckhouse/ee/registry-controller/manifests/v1.76.6", upstream.Path)
	assert.Equal(t, "license-key", upstream.Auth.Password)
	assert.Equal(t,
		"https://registry.deckhouse.io/v2/deckhouse/ee/registry-controller/manifests/v1.76.6", upstream.URL())
}

// TestResolvePrimaryAirGapHasNoFallback covers the cache being the only source: the
// upstream is gone from the layout, so there is nothing behind it.
func TestResolvePrimaryAirGapHasNoFallback(t *testing.T) {
	spec := layout()
	spec.Backends = spec.Backends[:1]

	decision, err := Resolve(constant.Host, "/v2/system/deckhouse/one/manifests/v1", spec, self)
	require.NoError(t, err)

	require.Len(t, decision.Targets, 1)
	assert.Equal(t, string(registryv1alpha1.BackendStorage), decision.Targets[0].Name)
}

// TestResolvePrimaryWithoutACache covers the cache being switched off: the upstream is
// the only backend, and the prefix is still swapped.
func TestResolvePrimaryWithoutACache(t *testing.T) {
	spec := layout()
	spec.Cache = false
	spec.Backends = spec.Backends[1:]

	decision, err := Resolve(constant.Host, "/v2/system/deckhouse/one/manifests/v1", spec, self)
	require.NoError(t, err)

	require.Len(t, decision.Targets, 1)
	assert.Equal(t, "/v2/deckhouse/ee/one/manifests/v1", decision.Targets[0].Path)
}

func TestResolveRoute(t *testing.T) {
	decision, err := Resolve("images.virtualization.example.com",
		"/v2/virtual-machine/manifests/v1.2.3", layout(), self)
	require.NoError(t, err)

	assert.Equal(t, KindRoute, decision.Kind)
	require.Len(t, decision.Targets, 1)

	target := decision.Targets[0]
	assert.Equal(t, "vendor.example.com", target.Host)
	// The image is named after the intercepted host, so the repository has to be
	// prefixed with where it lives in the upstream.
	assert.Equal(t, "/v2/virt/virtual-machine/manifests/v1.2.3", target.Path)
	assert.Equal(t, "vendor-secret", target.Auth.Password)
	assert.Equal(t, "vendor-ca", target.CA)
}

func TestResolveRouteMirrorsAreOrderedFallbacks(t *testing.T) {
	spec := layout()
	spec.AdditionalRoutes[0].Mirrors = []registryv1alpha1.Endpoint{{
		Scheme: registryv1alpha1.SchemeHTTPS, Host: "fallback.example.com", Path: "/virt-mirror",
	}}

	decision, err := Resolve("images.virtualization.example.com", "/v2/one/manifests/v1", spec, self)
	require.NoError(t, err)

	require.Len(t, decision.Targets, 2)
	assert.Equal(t, "vendor.example.com", decision.Targets[0].Host)
	assert.Equal(t, "fallback.example.com", decision.Targets[1].Host)
	assert.Equal(t, "/v2/virt-mirror/one/manifests/v1", decision.Targets[1].Path)
}

// TestResolvePassesThroughWhatNobodyConfigured is the consequence of the agent being
// on the path of every pull on the node. An unconfigured registry has to keep working,
// or workloads that never asked to be involved break.
func TestResolvePassesThroughWhatNobodyConfigured(t *testing.T) {
	decision, err := Resolve("docker.io", "/v2/library/nginx/manifests/latest", layout(), self)
	require.NoError(t, err)

	assert.Equal(t, KindPassThrough, decision.Kind)
	require.Len(t, decision.Targets, 1)

	target := decision.Targets[0]
	assert.Equal(t, "docker.io", target.Host)
	// Untouched: no prefix, and no credentials, because the agent has none for it and
	// inventing any would send this node's secrets to a registry nobody vetted.
	assert.Equal(t, "/v2/library/nginx/manifests/latest", target.Path)
	assert.Nil(t, target.Auth)
	assert.Empty(t, target.CA)
}

// TestResolveRefusesToTalkToItself guards the loop the `_default` fallback makes
// possible: the agent's own endpoint has no directory of its own either.
func TestResolveRefusesToTalkToItself(t *testing.T) {
	_, err := Resolve(self, "/v2/one/manifests/v1", layout(), self)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loop")

	// Case and whitespace must not get around it.
	_, err = Resolve(" 127.0.0.1:5001 ", "/v2/one/manifests/v1", layout(), self)
	assert.Error(t, err)
}

// TestResolveMatchesHostsCaseInsensitively matters because a name differing only in
// case is the same registry, and missing its route would silently send a pull
// somewhere else.
func TestResolveMatchesHostsCaseInsensitively(t *testing.T) {
	decision, err := Resolve("Images.Virtualization.Example.COM", "/v2/one/manifests/v1", layout(), self)
	require.NoError(t, err)
	assert.Equal(t, KindRoute, decision.Kind)

	decision, err = Resolve("Registry.D8-System.SVC:5001",
		"/v2/system/deckhouse/one/manifests/v1", layout(), self)
	require.NoError(t, err)
	assert.Equal(t, KindPrimary, decision.Kind)
}

func TestResolveEveryOperation(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		want        string
	}{
		{
			name:        "a manifest by tag",
			requestPath: "/v2/system/deckhouse/one/manifests/v1",
			want:        "/v2/deckhouse/ee/one/manifests/v1",
		},
		{
			name:        "a manifest by digest",
			requestPath: "/v2/system/deckhouse/one/manifests/sha256:abc",
			want:        "/v2/deckhouse/ee/one/manifests/sha256:abc",
		},
		{
			name:        "a blob",
			requestPath: "/v2/system/deckhouse/one/blobs/sha256:abc",
			want:        "/v2/deckhouse/ee/one/blobs/sha256:abc",
		},
		{
			name:        "a tag list",
			requestPath: "/v2/system/deckhouse/one/tags/list",
			want:        "/v2/deckhouse/ee/one/tags/list",
		},
		{
			name:        "referrers",
			requestPath: "/v2/system/deckhouse/one/referrers/sha256:abc",
			want:        "/v2/deckhouse/ee/one/referrers/sha256:abc",
		},
		{
			name: "a nested repository",
			// The repository itself contains slashes, which is why the split looks for the
			// operation rather than counting segments.
			requestPath: "/v2/system/deckhouse/group/sub/one/manifests/v1",
			want:        "/v2/deckhouse/ee/group/sub/one/manifests/v1",
		},
		{
			name: "a repository whose name contains the word blobs",
			// "blobs" appearing inside the repository must not be mistaken for the
			// operation, which is why the LAST occurrence wins.
			requestPath: "/v2/system/deckhouse/blobs/tool/manifests/v1",
			want:        "/v2/deckhouse/ee/blobs/tool/manifests/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := Resolve(constant.Host, tt.requestPath, layout(), self)
			require.NoError(t, err)
			require.Len(t, decision.Targets, 2)
			assert.Equal(t, tt.want, decision.Targets[1].Path)
		})
	}
}

func TestResolveRejectsWhatItCannotRoute(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		requestPath string
		spec        *registryv1alpha1.RegistryNodeSpec
	}{
		{
			name: "no namespace",
			// The runtime always names the original registry, since the agent is never the
			// registry that was asked for. Its absence means something else is talking.
			namespace:   "",
			requestPath: "/v2/one/manifests/v1",
			spec:        layout(),
		},
		{
			name:        "no layout",
			namespace:   constant.Host,
			requestPath: "/v2/one/manifests/v1",
			spec:        nil,
		},
		{
			name:        "not a registry path",
			namespace:   constant.Host,
			requestPath: "/healthz",
			spec:        layout(),
		},
		{
			name:        "no repository operation",
			namespace:   constant.Host,
			requestPath: "/v2/system/deckhouse/one",
			spec:        layout(),
		},
		{
			name:        "the primary image set with no backend",
			namespace:   constant.Host,
			requestPath: "/v2/system/deckhouse/one/manifests/v1",
			spec:        &registryv1alpha1.RegistryNodeSpec{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(tt.namespace, tt.requestPath, tt.spec, self)
			assert.Error(t, err)
		})
	}
}

func TestTargetURL(t *testing.T) {
	assert.Equal(t, "https://registry.example.com/v2/one/manifests/v1",
		(&Target{Host: "registry.example.com", Path: "/v2/one/manifests/v1"}).URL(),
		"an unset scheme must not silently downgrade to plain HTTP")

	assert.Equal(t, "http://registry.local:5000/v2/one/manifests/v1",
		(&Target{
			Scheme: registryv1alpha1.SchemeHTTP, Host: "registry.local:5000", Path: "/v2/one/manifests/v1",
		}).URL())
}

func TestJoinRepository(t *testing.T) {
	assert.Equal(t, "virt/one", joinRepository("/virt/", "one"))
	assert.Equal(t, "one", joinRepository("", "one"))
	assert.Equal(t, "virt", joinRepository("virt", ""))
	assert.Equal(t, "", joinRepository("", ""))
	assert.Equal(t, "deckhouse/ee/group/one", joinRepository("deckhouse/ee", "group/one"))
}

func TestSplitAPIPath(t *testing.T) {
	repository, remainder, err := splitAPIPath("/v2/group/one/manifests/v1")
	require.NoError(t, err)
	assert.Equal(t, "group/one", repository)
	assert.Equal(t, "/manifests/v1", remainder)

	_, _, err = splitAPIPath("/v2/")
	assert.Error(t, err, "the ping endpoint names no repository and is answered by the agent itself")
}
