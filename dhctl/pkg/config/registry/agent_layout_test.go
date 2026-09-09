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

package registry

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// The first master has to come out of the installation with the agent on it, because nothing else
// can put one there afterwards: the agent's package is fetched through registry-packages-proxy, and
// that proxy reaches the registry through the agent. Measured on a cache-less cluster before this
// existed — `[registry-agent] attempt 6 failed` on the master, thirty failed `rpp-get` attempts on
// the worker, and no node ever joining.
func TestTheFirstMasterIsToldAboutTheAgent(t *testing.T) {
	pki, err := GeneratePKI()
	require.NoError(t, err)

	config := ConfigBuilder(
		WithModeDirect(),
		WithImagesRepo("registry.example.com/deckhouse/ee"),
		WithCredentials("user", "password"),
		WithCA("CA CERTIFICATE"),
	)
	config.AgentOwnsRuntime = true

	ctx, err := config.Manifest().BashibleContext(func() (PKI, error) { return pki, nil })
	require.NoError(t, err)

	agent, ok := ctx.ToMap()["agent"].(map[string]any)
	require.True(t, ok, "the bashible steps cannot see an agent, so none of them would run")
	require.Equal(t, constant.ProxyHost, agent["endpoint"])
	require.Equal(t, constant.AgentDropInFile, agent["dropInFile"])

	// The layout is the only thing on the node that holds the upstream's credentials, and the only
	// way the agent can route before there is an API server to ask.
	var layout bootstrapLayout
	require.NoError(t, json.Unmarshal([]byte(agent["layout"].(string)), &layout))
	require.Equal(t, bootstrapLayout{
		Backends: []layoutBackend{{
			Name: backendUpstream,
			layoutEndpoint: layoutEndpoint{
				Scheme: "HTTPS",
				Host:   "registry.example.com",
				Path:   "/deckhouse/ee",
				CA:     "CA CERTIFICATE",
				Auth:   &layoutAuth{Username: "user", Password: "password"},
			},
		}},
	}, layout)

	// No storage backend. The store does not exist yet during an installation even when the cluster
	// is going to have one, and an address that answers nothing costs every pull a failed attempt.
	require.Len(t, layout.Backends, 1)
	require.False(t, layout.Cache)
}

// What the rest of the node configuration says, which is what the module itself writes once it is
// running. A node whose configuration changes the moment the module takes over is a node that rolls
// its container runtime for no reason.
func TestTheAgentConfigurationMatchesWhatTheModuleWrites(t *testing.T) {
	pki, err := GeneratePKI()
	require.NoError(t, err)

	config := ConfigBuilder(
		WithModeDirect(),
		WithImagesRepo("registry.example.com/deckhouse/ee"),
		WithCredentials("user", "password"),
	)
	config.AgentOwnsRuntime = true

	cfg, err := config.Manifest().modeModel.BashibleConfig(pki)
	require.NoError(t, err)

	require.Equal(t, constant.HostWithPath, cfg.ImagesBase)
	require.Empty(t, cfg.ProxyEndpoints, "the legacy node-side proxy would stay installed")
	require.NotEmpty(t, cfg.Version)

	// One host, the in-cluster one, and no credentials on it: the runtime reaches it through the
	// agent, and the agent is what holds what the upstream needs.
	require.Len(t, cfg.Hosts, 1)
	host := cfg.Hosts[constant.Host]
	require.Len(t, host.Mirrors, 1)
	require.Equal(t, constant.Host, host.Mirrors[0].Host)
	require.Equal(t, constant.Scheme, host.Mirrors[0].Scheme)
	require.Empty(t, host.Mirrors[0].Auth.Username)
	require.Empty(t, host.Mirrors[0].Auth.Password)
	require.Empty(t, host.Mirrors[0].Rewrites,
		"a rewrite belongs to a node talking to the upstream directly, which this one does not")
}

// And the other half: a configuration the module does not own must not grow an agent, because the
// legacy modes configure the container runtime themselves and two writers in one drop-in directory
// is the confusion the agent exists to remove.
func TestLegacyModesAreLeftAlone(t *testing.T) {
	pki, err := GeneratePKI()
	require.NoError(t, err)

	for _, config := range []Config{
		ConfigBuilder(WithModeUnmanaged(), WithLegacyMode()),
		ConfigBuilder(WithModeDirect()),
		ConfigBuilder(WithModeProxy()),
	} {
		cfg, err := config.Manifest().modeModel.BashibleConfig(pki)
		require.NoError(t, err)
		require.Nil(t, cfg.Agent, "mode %s grew an agent nobody asked for", config.Settings.Mode)
	}
}

// The installer decides this from the registry ModuleConfig, and it is the same decision that picks
// the mode — so it is checked where that decision is made rather than only where it is used.
func TestEveryManagedInstallationAsksForTheAgent(t *testing.T) {
	tests := []struct {
		name  string
		facts BundleBootstrapInputs
		want  bool
	}{
		{
			name: "an upstream and no cache: the cluster that could not install an agent at all",
			facts: BundleBootstrapInputs{
				UpstreamConfigured: true,
				Upstream:           &UpstreamFacts{Host: "registry.example.com"},
			},
			want: true,
		},
		{
			name: "an upstream and a cache",
			facts: BundleBootstrapInputs{
				CacheEnabled:       true,
				UpstreamConfigured: true,
				Upstream:           &UpstreamFacts{Host: "registry.example.com"},
			},
			want: true,
		},
		{
			// The bundle installation serves images from a registry on the node itself and is
			// already measured working end to end. Its first master is left as it is until the
			// same path is exercised there.
			name:  "a bundle",
			facts: BundleBootstrapInputs{CacheEnabled: true},
			want:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, opts := test.facts.Resolve(nil)

			provider := NewConfigProvider(nil, nil, opts...)
			require.Equal(t, test.want, provider.agentOwnsRuntime)
		})
	}
}
