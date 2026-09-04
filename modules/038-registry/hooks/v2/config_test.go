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

package v2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// testAddresses are the storage replica addresses as the hook discovers them: node
// addresses without a port, since the port is a constant of the design.
var testAddresses = []string{"10.0.0.1", "10.0.0.2"}

// licensedSettings is the ordinary case: one licensed upstream, no cache.
func licensedSettings(t *testing.T) settings {
	t.Helper()

	return decode(t, `{
		"mode": "Managed",
		"primary": {"upstream": {
			"scheme": "HTTPS",
			"host": "registry.deckhouse.io",
			"path": "/deckhouse/ee",
			"auth": {"license": "the-license-key"}
		}}
	}`)
}

// decode is how the settings actually arrive: as the JSON of the module's values, not as
// a hand-built struct. Going through it keeps the field names in the struct honest.
func decode(t *testing.T, raw string) settings {
	t.Helper()

	var parsed settings
	require.NoError(t, json.Unmarshal([]byte(raw), &parsed))
	return parsed
}

// TestBuildRegistryConfigResolvesALicense is the one piece of shorthand in the
// configuration. Expanded here rather than in the template, so the resource never
// carries a field the CRD does not have.
func TestBuildRegistryConfigResolvesALicense(t *testing.T) {
	config := buildRegistryConfig(licensedSettings(t))

	require.NotNil(t, config.Primary.Upstream)
	require.NotNil(t, config.Primary.Upstream.Auth)
	assert.Equal(t, registry_const.LicenseUsername, config.Primary.Upstream.Auth.Username)
	assert.Equal(t, "the-license-key", config.Primary.Upstream.Auth.Password)
}

// TestBuildRegistryConfigPrefersExplicitCredentials keeps a private mirror of a licensed
// registry expressible: the username the operator gave wins over the license shorthand.
func TestBuildRegistryConfigPrefersExplicitCredentials(t *testing.T) {
	config := buildRegistryConfig(decode(t, `{
		"mode": "Managed",
		"primary": {"upstream": {
			"host": "mirror.example.com",
			"auth": {"license": "the-license-key", "username": "robot", "password": "the-secret"}
		}}
	}`))

	require.NotNil(t, config.Primary.Upstream.Auth)
	assert.Equal(t, "robot", config.Primary.Upstream.Auth.Username)
	assert.Equal(t, "the-secret", config.Primary.Upstream.Auth.Password)
}

func TestBuildRegistryConfigWithoutAnUpstreamIsAirGap(t *testing.T) {
	config := buildRegistryConfig(decode(t, `{
		"mode": "Managed",
		"storage": {"cache": true, "size": "20Gi", "source": {"bundleRef": "bundle", "expectedDigests": 459}}
	}`))

	assert.Nil(t, config.Primary.Upstream)
	assert.True(t, config.Storage.Cache)
	require.NotNil(t, config.Storage.Source)
	assert.EqualValues(t, 459, config.Storage.Source.ExpectedDigests)
}

// TestBuildBashibleConfigCarriesTheAgent is what every node's behaviour hinges on: the
// marker is what silences the step that writes per-registry drop-in directories. Without
// it a node keeps the legacy behaviour, and with it there are two writers.
func TestBuildBashibleConfigCarriesTheAgent(t *testing.T) {
	config := buildRegistryConfig(licensedSettings(t))

	built, err := buildBashibleConfig(config, registryv1alpha1.Auth{}, "", testAddresses)
	require.NoError(t, err)
	require.NotNil(t, built.Agent)
	assert.Equal(t, registry_const.ProxyHost, built.Agent.Endpoint)
	assert.NotEmpty(t, built.Version, "the node configuration has no version to compare against")
	assert.Empty(t, built.ProxyEndpoints, "the legacy node proxy is not removed")
	assert.Equal(t, registry_const.HostWithPath, built.ImagesBase)
}

// TestBuildBashibleConfigIsValidNodeContext guards the shared boundary: this structure is
// read by the same apiserver code as the legacy one, which validates it and refuses to
// serve a node context that does not pass.
func TestBuildBashibleConfigIsValidNodeContext(t *testing.T) {
	config := buildRegistryConfig(licensedSettings(t))

	built, err := buildBashibleConfig(config, registryv1alpha1.Auth{}, "", testAddresses)
	require.NoError(t, err)
	require.NoError(t, built.Validate())
	require.NoError(t, built.ToContext().Validate())
}

// TestBuildBootstrapLayoutSeedsTheUpstream is the routing a node uses before there is an
// API server to ask, which is the only thing that lets a first master pull the control
// plane at all.
func TestBuildBootstrapLayoutSeedsTheUpstream(t *testing.T) {
	config := buildRegistryConfig(licensedSettings(t))

	seed, err := buildBootstrapLayout(config, registryv1alpha1.Auth{}, "", testAddresses)
	require.NoError(t, err)

	var spec registryv1alpha1.RegistryNodeSpec
	require.NoError(t, json.Unmarshal([]byte(seed), &spec))

	upstream := spec.Backend(registryv1alpha1.BackendUpstream)
	require.NotNil(t, upstream)
	assert.Equal(t, "registry.deckhouse.io", upstream.Host)
	assert.Equal(t, "the-license-key", upstream.Auth.Password)
	assert.Nil(t, spec.Backend(registryv1alpha1.BackendStorage), "no cache was configured")
}

// TestBuildBootstrapLayoutSeedsTheCacheToo covers a node joining a cluster whose cache is
// already the way in. It needs the storage credentials and certificate authority, because
// there is nothing else on the node to dereference them from.
func TestBuildBootstrapLayoutSeedsTheCacheToo(t *testing.T) {
	config := buildRegistryConfig(decode(t, `{
		"mode": "Managed",
		"storage": {"cache": true, "source": {"bundleRef": "bundle", "expectedDigests": 459}}
	}`))

	seed, err := buildBootstrapLayout(config,
		registryv1alpha1.Auth{Username: "registry-ro", Password: "the-read-secret"},
		"-----BEGIN CERTIFICATE-----storage", testAddresses)
	require.NoError(t, err)

	var spec registryv1alpha1.RegistryNodeSpec
	require.NoError(t, json.Unmarshal([]byte(seed), &spec))

	storage := spec.Backend(registryv1alpha1.BackendStorage)
	require.NotNil(t, storage)
	// An address, not the Service name: a node reading this file is bootstrapping and
	// has neither cluster DNS nor a cluster network.
	assert.Equal(t, "10.0.0.1:5001", storage.Host)
	require.Len(t, storage.Mirrors, 1)
	assert.Equal(t, "10.0.0.2:5001", storage.Mirrors[0].Host)
	assert.Equal(t, "the-read-secret", storage.Auth.Password)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----storage", storage.CA)
	assert.Nil(t, spec.Backend(registryv1alpha1.BackendUpstream), "air-gap has no upstream")
}

// TestBuildBootstrapLayoutWithNothingToSeed: no cache and no upstream is a configuration
// the schema already refuses, and an empty seed is a better answer than a file the agent
// would reject.
func TestBuildBootstrapLayoutWithNothingToSeed(t *testing.T) {
	config := buildRegistryConfig(decode(t, `{"mode": "Managed"}`))

	seed, err := buildBootstrapLayout(config, registryv1alpha1.Auth{}, "", testAddresses)
	require.NoError(t, err)
	assert.Empty(t, seed)
}

// TestBuildBootstrapLayoutCarriesMirrors keeps the fallbacks available in the one window
// where an operator cannot intervene: bootstrap, with no API server to read them from.
func TestBuildBootstrapLayoutCarriesMirrors(t *testing.T) {
	config := buildRegistryConfig(decode(t, `{
		"mode": "Managed",
		"primary": {"upstream": {
			"host": "registry.deckhouse.io",
			"path": "/deckhouse/ee",
			"mirrors": [{"host": "mirror.example.com", "path": "/d8", "auth": {"username": "robot", "password": "s"}}]
		}}
	}`))

	seed, err := buildBootstrapLayout(config, registryv1alpha1.Auth{}, "", testAddresses)
	require.NoError(t, err)

	var spec registryv1alpha1.RegistryNodeSpec
	require.NoError(t, json.Unmarshal([]byte(seed), &spec))

	upstream := spec.Backend(registryv1alpha1.BackendUpstream)
	require.NotNil(t, upstream)
	require.Len(t, upstream.Mirrors, 1)
	assert.Equal(t, "mirror.example.com", upstream.Mirrors[0].Host)
	assert.Equal(t, registryv1alpha1.SchemeHTTPS, upstream.Mirrors[0].Scheme,
		"an unspecified scheme has to default, or the agent builds a URL with none")
}
