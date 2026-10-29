// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	cca "github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/utils/check-cloud-api"
)

func dynamixMetaConfig(t *testing.T, providerConfig map[string]any) *config.MetaConfig {
	t.Helper()

	provider := map[string]json.RawMessage{}
	for key, value := range providerConfig {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		provider[key] = raw
	}

	return &config.MetaConfig{
		ProviderName:          "dynamix",
		ProviderClusterConfig: provider,
		Registry: registry_mocks.ConfigBuilder(
			registry_mocks.WithImagesRepo("registry.company.my/deckhouse/ee"),
			registry_mocks.WithSchemeHTTPS(),
		),
	}
}

// TestNodeResolverFix: a node dhctl has just created gets its resolver from the cloud, so the
// advice to edit /etc/resolv.conf on it is lost with the node. Where the provider configuration
// decides the resolver, the fix names that field.
func TestNodeResolverFix(t *testing.T) {
	const hostname = "registry.company.my"

	t.Run("a provider with nothing to say about it", func(t *testing.T) {
		fix := nodeResolverFix(&config.MetaConfig{ProviderName: "yandex"}, hostname)

		assert.Contains(t, fix, "/etc/resolv.conf")
		assert.Contains(t, fix, hostname)
	})

	t.Run("no configuration at all", func(t *testing.T) {
		assert.Contains(t, nodeResolverFix(nil, hostname), "/etc/resolv.conf")
	})

	// Bought on dxe460: the external network handed out no DNS server, nameservers was empty, and
	// the bootstrap stopped on "connection reset by peer" and "HTTP 000".
	t.Run("Dynamix Standard without nameservers", func(t *testing.T) {
		fix := nodeResolverFix(dynamixMetaConfig(t, map[string]any{"layout": "Standard", "nameservers": []string{}}), hostname)

		assert.Contains(t, fix, "DynamixClusterConfiguration.nameservers")
		assert.Contains(t, fix, "external network")
		assert.NotContains(t, fix, "/etc/resolv.conf")
	})

	t.Run("Dynamix Standard with nameservers that do not resolve the name", func(t *testing.T) {
		fix := nodeResolverFix(dynamixMetaConfig(t, map[string]any{"layout": "Standard", "nameservers": []string{"10.4.9.254"}}), hostname)

		assert.Contains(t, fix, "DynamixClusterConfiguration.nameservers")
		assert.Contains(t, fix, "10.4.9.254")
		assert.Contains(t, fix, hostname)
	})

	// Not "nameservers is empty": the field is set, this just cannot read it, and the
	// general advice at least sends the user to the node.
	t.Run("Dynamix Standard with nameservers in an unexpected shape", func(t *testing.T) {
		fix := nodeResolverFix(dynamixMetaConfig(t, map[string]any{"layout": "Standard", "nameservers": "10.4.9.254"}), hostname)

		assert.Contains(t, fix, "/etc/resolv.conf")
		assert.NotContains(t, fix, "DynamixClusterConfiguration.nameservers")
	})

	// The ViNS hands out the servers there, and the field is already what configures it.
	t.Run("Dynamix StandardWithInternalNetwork", func(t *testing.T) {
		fix := nodeResolverFix(dynamixMetaConfig(t, map[string]any{"layout": "StandardWithInternalNetwork"}), hostname)

		assert.Contains(t, fix, "/etc/resolv.conf")
	})
}

// TestRegistryFromMasterReadsCurlStatusZero: curl prints 000 for its -w status when no response
// came back at all, so the check read the output, found a status, and reported "the node got
// HTTP 000, which is not how a registry answers /v2/" — for a node that could not resolve the
// registry, which the exit status said plainly.
func TestRegistryFromMasterReadsCurlStatusZero(t *testing.T) {
	meta := dynamixMetaConfig(t, map[string]any{"layout": "Standard"})
	endpoint := registryV2URL(meta).String()
	node := newFakeNode().
		on("command -v curl").succeeds().
		on("curl -s -o /dev/null -w %{http_code} --max-time 20 "+endpoint).printsAndExits("000", 6)

	check := RegistryFromMasterCheck{MetaConfig: meta}
	_, err := check.requestFromNode(t.Context(), node, check.target())

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "cannot resolve registry.company.my")
	assert.Contains(t, failure.Fix, "DynamixClusterConfiguration.nameservers")
}

// TestCloudAPIAsksTheNodeToResolve: through the SSH tunnel a name the master cannot resolve does
// not come back as a DNS error. sshd fails to dial and closes the forwarded channel, and the
// request sees only "connection reset by peer".
func TestCloudAPIAsksTheNodeToResolve(t *testing.T) {
	meta := dynamixMetaConfig(t, map[string]any{"layout": "Standard"})
	apiConfig := &cca.CloudAPIConfig{
		URL:   &url.URL{Scheme: "https", Host: "dxe460.dpr.lcl"},
		Field: "DynamixClusterConfiguration.provider.controllerUrl",
	}
	check := CloudAPICheck{MetaConfig: meta}

	t.Run("the name does not resolve", func(t *testing.T) {
		node := newFakeNode().on("getent ahosts dxe460.dpr.lcl").exits(2)

		err := check.unresolvedFromNode(t.Context(), node, apiConfig)

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "cannot resolve dxe460.dpr.lcl")
		assert.Contains(t, failure.Fix, "DynamixClusterConfiguration.nameservers")
	})

	t.Run("the name resolves", func(t *testing.T) {
		node := newFakeNode().on("getent ahosts dxe460.dpr.lcl").prints("100.120.4.10 STREAM dxe460.dpr.lcl")

		assert.NoError(t, check.unresolvedFromNode(t.Context(), node, apiConfig))
	})

	// No getent, or a session that is gone, says nothing about DNS: the transport error stands.
	t.Run("the question cannot be asked", func(t *testing.T) {
		assert.NoError(t, check.unresolvedFromNode(t.Context(), newFakeNode(), apiConfig))

		node := newFakeNode().on("getent ahosts dxe460.dpr.lcl").fails(errors.New("session closed"))
		assert.NoError(t, check.unresolvedFromNode(t.Context(), node, apiConfig))
	})

	t.Run("an address needs no resolving", func(t *testing.T) {
		node := newFakeNode()
		byAddress := &cca.CloudAPIConfig{URL: &url.URL{Scheme: "https", Host: "100.120.4.10:443"}}

		assert.NoError(t, check.unresolvedFromNode(t.Context(), node, byAddress))
		assert.Empty(t, node.ran())
	})
}
