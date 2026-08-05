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

package containerd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

func testOptions() Options {
	return Options{
		Root:          "/etc/containerd/registry.d",
		ProxyEndpoint: "127.0.0.1:5001",
		ProxyCA:       "-----BEGIN CERTIFICATE-----agent",
	}
}

func cachedLayout() *registryv1alpha1.RegistryNodeSpec {
	return &registryv1alpha1.RegistryNodeSpec{
		Cache: true,
		Backends: []registryv1alpha1.Backend{
			{
				Name:     registryv1alpha1.BackendStorage,
				Endpoint: registryv1alpha1.Endpoint{Host: constant.Host, Path: constant.Path},
			},
			{
				Name:     registryv1alpha1.BackendUpstream,
				Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee"},
			},
		},
	}
}

func fileOf(t *testing.T, desired Desired, path string) string {
	t.Helper()

	content, ok := desired.Files[path]
	require.Truef(t, ok, "no %s in %v", path, keysOf(desired))
	return string(content)
}

func keysOf(desired Desired) []string {
	keys := make([]string, 0, len(desired.Files))
	for key := range desired.Files {
		keys = append(keys, key)
	}
	return keys
}

// TestRenderPointsEverythingAtTheAgent is the shape of the whole design: one
// fallback directory, and the runtime asks the agent about every registry.
func TestRenderPointsEverythingAtTheAgent(t *testing.T) {
	desired, err := Render(cachedLayout(), testOptions())
	require.NoError(t, err)

	assert.Equal(t, []string{DefaultHost}, desired.Hosts)

	config := fileOf(t, desired, filepath.Join(DefaultHost, hostsFile))
	assert.Contains(t, config, `[host."https://127.0.0.1:5001"]`)
	// Pull and resolve only. An endpoint that could be pushed to would let anything on
	// the node write into the registry the whole cluster reads.
	assert.Contains(t, config, `capabilities = ["pull", "resolve"]`)
	assert.NotContains(t, config, "push")

	assert.Contains(t, config, filepath.Join(testOptions().Root, DefaultHost, ProxyCAFile))
	assert.Equal(t, "-----BEGIN CERTIFICATE-----agent",
		fileOf(t, desired, filepath.Join(DefaultHost, ProxyCAFile)))
}

// TestRenderIgnoresRoutesAndBackends is the property that keeps node configuration
// static. A module declaring an upstream, an upstream being removed, the cluster
// going air-gap — none of it may produce a write on any node, because the runtime
// already looks only at the agent.
func TestRenderIgnoresRoutesAndBackends(t *testing.T) {
	baseline, err := Render(cachedLayout(), testOptions())
	require.NoError(t, err)

	variants := map[string]*registryv1alpha1.RegistryNodeSpec{
		"an additional upstream appears": {
			Backends: cachedLayout().Backends,
			AdditionalRoutes: []registryv1alpha1.Route{{
				Match: "images.virtualization.example.com",
				Endpoint: registryv1alpha1.Endpoint{
					Host: "registry-vendor.example.com", Path: "/virt",
					CA:   "-----BEGIN CERTIFICATE-----vendor",
					Auth: &registryv1alpha1.Auth{Username: "user", Password: "pass"},
				},
			}},
		},
		"several additional upstreams": {
			Backends: cachedLayout().Backends,
			AdditionalRoutes: []registryv1alpha1.Route{
				{Match: "ghcr.io", Endpoint: registryv1alpha1.Endpoint{Host: "one.example.com"}},
				{Match: "docker.io", Endpoint: registryv1alpha1.Endpoint{Host: "two.example.com"}},
			},
		},
		"the upstream is dropped for air-gap": {
			Backends: cachedLayout().Backends[:1],
		},
		"the cache is turned off": {
			Backends: []registryv1alpha1.Backend{{
				Name:     registryv1alpha1.BackendUpstream,
				Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee"},
			}},
		},
	}

	for name, spec := range variants {
		t.Run(name, func(t *testing.T) {
			desired, err := Render(spec, testOptions())
			require.NoError(t, err)

			assert.Equal(t, baseline.Hosts, desired.Hosts)
			require.Equal(t, len(baseline.Files), len(desired.Files))
			for path, content := range baseline.Files {
				assert.Equal(t, string(content), string(desired.Files[path]), path)
			}
		})
	}
}

// TestRenderKeepsCredentialsOutOfTheRuntimeConfiguration narrows a claim that is
// easy to overstate. Credentials DO reach the node: the agent keeps a full copy of
// its layout on disk, credentials included, because that copy is the only reason it
// keeps working when the API server is unreachable. The two requirements are linked,
// not in tension.
//
// What this asserts is smaller and still worth having: they stay out of the
// runtime's own configuration files. Those have to be readable by containerd, which
// does not run as the agent; the agent's copy is its own and can be readable by
// nobody else.
func TestRenderKeepsCredentialsOutOfTheRuntimeConfiguration(t *testing.T) {
	spec := cachedLayout()
	spec.Backends[1].Auth = &registryv1alpha1.Auth{Username: "license-token", Password: "the-license-key"}
	spec.AdditionalRoutes = []registryv1alpha1.Route{{
		Match: "images.example.com",
		Endpoint: registryv1alpha1.Endpoint{
			Host: "vendor.example.com",
			Auth: &registryv1alpha1.Auth{Username: "vendor-user", Password: "the-vendor-secret"},
		},
	}}

	desired, err := Render(spec, testOptions())
	require.NoError(t, err)

	for path, content := range desired.Files {
		assert.NotContains(t, string(content), "the-license-key", path)
		assert.NotContains(t, string(content), "the-vendor-secret", path)
		assert.NotContains(t, string(content), "license-token", path)
	}
}

func TestRenderIsByteStable(t *testing.T) {
	first, err := Render(cachedLayout(), testOptions())
	require.NoError(t, err)
	second, err := Render(cachedLayout(), testOptions())
	require.NoError(t, err)

	// The writer only touches what changed, so an unstable rendering would rewrite
	// the directory on every pass.
	for path, content := range first.Files {
		assert.Equal(t, string(content), string(second.Files[path]), path)
	}
	assert.Equal(t, first.Hosts, second.Hosts)
}

func TestRenderFollowsTheProxyCertificate(t *testing.T) {
	// The one thing that does change the node configuration: the agent's own
	// certificate authority, after a rotation.
	rotated := testOptions()
	rotated.ProxyCA = "-----BEGIN CERTIFICATE-----rotated"

	before, err := Render(cachedLayout(), testOptions())
	require.NoError(t, err)
	after, err := Render(cachedLayout(), rotated)
	require.NoError(t, err)

	assert.NotEqual(t,
		string(before.Files[filepath.Join(DefaultHost, ProxyCAFile)]),
		string(after.Files[filepath.Join(DefaultHost, ProxyCAFile)]))
}

func TestRenderWithNothingToRoute(t *testing.T) {
	// An Unmanaged node, or a layout not compiled yet. Nothing is claimed, and in
	// particular no fallback that would send every pull on the node to an agent that
	// has no idea where to send it.
	desired, err := Render(&registryv1alpha1.RegistryNodeSpec{}, testOptions())
	require.NoError(t, err)

	assert.Empty(t, desired.Hosts)
	assert.Empty(t, desired.Files)
}

func TestRenderRejectsBadInput(t *testing.T) {
	_, err := Render(nil, testOptions())
	assert.Error(t, err)

	withoutEndpoint := testOptions()
	withoutEndpoint.ProxyEndpoint = ""
	_, err = Render(cachedLayout(), withoutEndpoint)
	assert.Error(t, err, "without an endpoint the runtime would be pointed nowhere")

	withoutCA := testOptions()
	withoutCA.ProxyCA = ""
	_, err = Render(cachedLayout(), withoutCA)
	assert.Error(t, err,
		"without the authority the runtime cannot verify the agent, and every pull fails at the handshake")
}

func TestRenderDefaultsTheRoot(t *testing.T) {
	options := testOptions()
	options.Root = ""

	desired, err := Render(cachedLayout(), options)
	require.NoError(t, err)

	assert.Contains(t, fileOf(t, desired, filepath.Join(DefaultHost, hostsFile)),
		filepath.Join(DefaultRoot, DefaultHost, ProxyCAFile))
}

// TestTheDropInPathIsTheSharedOne is a contract test rather than a behaviour one. A
// bashible step waits for exactly this file as the sign that pulls on the node can
// succeed, so the two must not be able to drift apart silently.
func TestTheDropInPathIsTheSharedOne(t *testing.T) {
	desired, err := Render(&registryv1alpha1.RegistryNodeSpec{
		Backends: []registryv1alpha1.Backend{{
			Name:     registryv1alpha1.BackendUpstream,
			Endpoint: registryv1alpha1.Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee"},
		}},
	}, Options{ProxyEndpoint: "127.0.0.1:5001", ProxyCA: "-----BEGIN CERTIFICATE-----agent"})
	require.NoError(t, err)

	relative := strings.TrimPrefix(constant.AgentDropInFile, DefaultRoot+"/")
	require.Contains(t, desired.Files, relative,
		"the agent does not write the file the bashible step waits for")
}
