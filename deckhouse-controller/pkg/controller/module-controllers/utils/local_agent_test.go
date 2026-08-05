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

// An internal test, because what it is about is a path this process reads at runtime, and
// pointing that at a file a test is allowed to create is the only way to exercise it.
package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func withAgentCA(t *testing.T, content string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ca.crt")
	if content != "" {
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	previous := agentCAFile
	agentCAFile = path
	t.Cleanup(func() { agentCAFile = previous })
}

// TestForRepositoryLeavesOrdinaryRegistriesAlone is the case that must not regress: almost
// every cluster fetches straight from a registry outside it, and nothing here applies.
func TestForRepositoryLeavesOrdinaryRegistriesAlone(t *testing.T) {
	withAgentCA(t, "AGENT-CA")

	config := &RegistryConfig{Scheme: "http", CA: "UPSTREAM-CA", DockerConfig: "CFG"}
	got := config.ForRepository("registry.example.com/deckhouse/ee", log.NewNop())

	assert.Equal(t, "http", got.Scheme, "an upstream reached over plain HTTP is ordinary")
	assert.Equal(t, "UPSTREAM-CA", got.CA)
	assert.Equal(t, "CFG", got.DockerConfig)
}

// TestForRepositoryTrustsTheNodeWhenFetchingThroughIt covers the two things about the agent
// that no cluster object can describe.
func TestForRepositoryTrustsTheNodeWhenFetchingThroughIt(t *testing.T) {
	withAgentCA(t, "AGENT-CA")

	// As the secret describes it on a cluster whose upstream speaks plain HTTP — which is
	// exactly where taking the configured scheme would send a plaintext request to the
	// agent's TLS listener.
	config := &RegistryConfig{Scheme: "http", CA: "UPSTREAM-CA", DockerConfig: "CFG"}
	got := config.ForRepository(registry_const.ProxyHostWithPath, log.NewNop())

	assert.Equal(t, registry_const.Scheme, got.Scheme,
		"the agent serves HTTPS whatever the registry behind it speaks")
	assert.Equal(t, "AGENT-CA", got.CA,
		"the authority is generated on the node, so it cannot be the one the cluster knows")

	// Left alone rather than cleared: the agent authenticates to the registry behind it with
	// credentials of its own, and a docker config naming other hosts says nothing about the
	// loopback address. Clearing it would be one more thing to get wrong for no gain.
	assert.Equal(t, "CFG", got.DockerConfig)
}

// TestForRepositoryFallsBackWhenTheAuthorityIsMissing is about the shape of the failure.
//
// A cluster where the module manages the pull path but this pod cannot read the node's
// authority is broken either way — the fetch below cannot verify what it is given. What
// matters is that it fails as a fetch that could not verify a certificate, with a log line
// naming the file, rather than as a panic or a silent switch to no verification at all.
func TestForRepositoryFallsBackWhenTheAuthorityIsMissing(t *testing.T) {
	withAgentCA(t, "")

	config := &RegistryConfig{Scheme: "https", CA: "UPSTREAM-CA"}
	got := config.ForRepository(registry_const.ProxyHostWithPath, log.NewNop())

	assert.Equal(t, "UPSTREAM-CA", got.CA)
	assert.Equal(t, registry_const.Scheme, got.Scheme)
}

// TestRegistryConfigFromSecretFollowsTheAddressItWillDial ties the two together: which
// authority is used is decided by the address in the secret, not by anything else.
func TestRegistryConfigFromSecretFollowsTheAddressItWillDial(t *testing.T) {
	withAgentCA(t, "AGENT-CA")

	secret := &DeckhouseRegistrySecret{
		DockerConfig: "CFG",
		Address:      "registry.example.com",
		Path:         "/deckhouse/ee",
		Scheme:       "https",
		CA:           "UPSTREAM-CA",
	}

	secret.ImageRegistry = "registry.example.com/deckhouse/ee"
	assert.Equal(t, "UPSTREAM-CA", secret.RegistryConfig("uuid", log.NewNop()).CA)

	// The same secret, with only the address the controller fetches from moved onto the
	// agent — which is the whole of what changes when the registry module takes over.
	secret.ImageRegistry = registry_const.ProxyHostWithPath
	assert.Equal(t, "AGENT-CA", secret.RegistryConfig("uuid", log.NewNop()).CA)
}

// TestDialTranslatesOnlyTheInClusterAddress is the rule that keeps the loopback address out of
// everything the cluster records.
//
// A recorded address has more than one reader: it ends up in pod image references, in a
// ModuleSource, in a status. The loopback one belongs in none of those — it is node-local, and a
// cluster-wide object naming it is wrong even where it appears to work. What made that concrete
// was a ModuleSource whose repository named it: the pods under it kept running on content their
// nodes already held, while the agent answered 502 to every fresh pull, since a request naming
// the agent as its own registry is a loop and is refused. A node that joined later, or one that
// had collected its images, would not have started them at all.
func TestDialTranslatesOnlyTheInClusterAddress(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		want       string
	}{{
		name:       "the in-cluster registry, which is what everything records",
		repository: registry_const.HostWithPath,
		want:       registry_const.ProxyHostWithPath,
	}, {
		name:       "the in-cluster registry with a module under it",
		repository: registry_const.HostWithPath + "/modules/upmeter",
		want:       registry_const.ProxyHostWithPath + "/modules/upmeter",
	}, {
		name:       "the bare in-cluster host",
		repository: registry_const.Host,
		want:       registry_const.ProxyHost,
	}, {
		// Already dialable, and translating twice must not mangle it.
		name:       "the loopback address itself",
		repository: registry_const.ProxyHostWithPath,
		want:       registry_const.ProxyHostWithPath,
	}, {
		name:       "somebody else's registry",
		repository: "registry.deckhouse.io/deckhouse/ee",
		want:       "registry.deckhouse.io/deckhouse/ee",
	}, {
		// Merely starting with the same letters is a different host.
		name:       "a host whose name begins like the in-cluster one",
		repository: registry_const.Host + "-staging/deckhouse",
		want:       registry_const.Host + "-staging/deckhouse",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Dial(tt.repository))
		})
	}
}

// TestForRepositoryAcceptsEitherSpellingOfTheAgent: the configuration is adjusted by what the
// repository names, and the same agent is named two ways — as recorded and as dialled.
func TestForRepositoryAcceptsEitherSpellingOfTheAgent(t *testing.T) {
	withAgentCA(t, "AGENT-CA")

	for _, repository := range []string{registry_const.HostWithPath, registry_const.ProxyHostWithPath} {
		config := &RegistryConfig{Scheme: "http", CA: "UPSTREAM-CA"}
		got := config.ForRepository(repository, log.NewNop())

		assert.Equal(t, "AGENT-CA", got.CA, repository)
		assert.Equal(t, registry_const.Scheme, got.Scheme, repository)
	}
}
