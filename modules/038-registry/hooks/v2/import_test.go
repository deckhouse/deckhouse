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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	registry_helpers "github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouseregistry"
)

func dockerConfigFor(t *testing.T, host, username, password string) []byte {
	t.Helper()

	raw, err := registry_helpers.DockerCfgFromCreds(username, password, host)
	require.NoError(t, err)
	return raw
}

// TestUpstreamFromLegacyCarriesEverythingNeededToPull is the point of the import: these
// values are spread across a secret, and an operator retyping them by hand is where a
// wrong certificate authority or a truncated path comes from.
func TestUpstreamFromLegacyCarriesEverythingNeededToPull(t *testing.T) {
	upstream, err := upstreamFromLegacy(deckhouse_registry.Config{
		Address:      "registry.deckhouse.io",
		Path:         "/deckhouse/ee",
		Scheme:       "https",
		CA:           "-----BEGIN CERTIFICATE-----upstream",
		DockerConfig: dockerConfigFor(t, "registry.deckhouse.io", "license-token", "the-license-key"),
	})
	require.NoError(t, err)
	require.NotNil(t, upstream)

	assert.Equal(t, "HTTPS", upstream["scheme"], "the schemes of the two models differ in case")
	assert.Equal(t, "registry.deckhouse.io", upstream["host"])
	assert.Equal(t, "/deckhouse/ee", upstream["path"])
	assert.Equal(t, "-----BEGIN CERTIFICATE-----upstream", upstream["ca"])

	auth, ok := upstream["auth"].(map[string]any)
	require.True(t, ok, "the credentials were not carried across")
	assert.Equal(t, "license-token", auth["username"])
	assert.Equal(t, "the-license-key", auth["password"])
}

// TestUpstreamFromLegacyRefusesTheInClusterAddress is the one case where importing would
// be actively wrong: a cluster in the legacy Proxy or Local mode pulls from the address
// the new implementation itself serves, and copying that across would tell the cluster to
// pull its images from itself.
func TestUpstreamFromLegacyRefusesTheInClusterAddress(t *testing.T) {
	upstream, err := upstreamFromLegacy(deckhouse_registry.Config{
		Address: "registry.d8-system.svc:5001",
		Path:    "/system/deckhouse",
		Scheme:  "https",
	})
	require.NoError(t, err)
	assert.Nil(t, upstream)
}

func TestUpstreamFromLegacyWithNothingConfigured(t *testing.T) {
	upstream, err := upstreamFromLegacy(deckhouse_registry.Config{})
	require.NoError(t, err)
	assert.Nil(t, upstream)
}

// TestUpstreamFromLegacyWithAnAnonymousRegistry: a public registry has no credentials,
// and an empty auth block is not the same as none — the new schema would carry an empty
// username into the resource.
func TestUpstreamFromLegacyWithAnAnonymousRegistry(t *testing.T) {
	upstream, err := upstreamFromLegacy(deckhouse_registry.Config{
		Address: "registry.deckhouse.io",
		Path:    "/deckhouse/ce",
		Scheme:  "HTTPS",
	})
	require.NoError(t, err)
	require.NotNil(t, upstream)

	_, present := upstream["auth"]
	assert.False(t, present)
	_, hasCA := upstream["ca"]
	assert.False(t, hasCA)
}

// TestUpstreamFromLegacyDefaultsTheScheme keeps a resource from being rendered with an
// empty scheme, which is not a value the CRD accepts.
func TestUpstreamFromLegacyDefaultsTheScheme(t *testing.T) {
	upstream, err := upstreamFromLegacy(deckhouse_registry.Config{
		Address: "registry.example.com",
	})
	require.NoError(t, err)
	require.NotNil(t, upstream)
	assert.Equal(t, "HTTPS", upstream["scheme"])
}

// TestUpstreamFromLegacyWithAnInsecureRegistry covers the one setting where getting the
// case wrong silently changes behaviour rather than failing.
func TestUpstreamFromLegacyWithAnInsecureRegistry(t *testing.T) {
	upstream, err := upstreamFromLegacy(deckhouse_registry.Config{
		Address: "registry.internal:5000",
		Scheme:  "http",
	})
	require.NoError(t, err)
	require.NotNil(t, upstream)
	assert.Equal(t, "HTTP", upstream["scheme"])
}

// TestSuggestedModuleConfigIsApplicable is the whole point of publishing a document
// rather than a fragment: an operator has to be able to read it out and apply it without
// editing it into shape.
func TestSuggestedModuleConfigIsApplicable(t *testing.T) {
	upstream, err := upstreamFromLegacy(deckhouse_registry.Config{
		Address:      "registry.deckhouse.io",
		Path:         "/deckhouse/ee",
		Scheme:       "https",
		DockerConfig: dockerConfigFor(t, "registry.deckhouse.io", "license-token", "the-license-key"),
	})
	require.NoError(t, err)

	document, err := suggestedModuleConfig(upstream)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(document), &parsed))
	assert.Equal(t, "ModuleConfig", parsed["kind"])
	assert.Equal(t, "deckhouse.io/v1alpha1", parsed["apiVersion"])

	settings := parsed["spec"].(map[string]any)["settings"].(map[string]any)
	// Carries the mode explicitly, because the schema refuses settings without it: a
	// suggestion that omitted it would be a document an operator cannot apply. There is
	// no implementation field to set — there is no choice of implementation.
	assert.Equal(t, "Managed", settings["mode"])
	_, hasImplementation := settings["implementation"]
	assert.False(t, hasImplementation, "the removed discriminator is back in the suggestion")

	primary := settings["primary"].(map[string]any)["upstream"].(map[string]any)
	assert.Equal(t, "registry.deckhouse.io", primary["host"])
	assert.Equal(t, "the-license-key", primary["auth"].(map[string]any)["password"])
}
