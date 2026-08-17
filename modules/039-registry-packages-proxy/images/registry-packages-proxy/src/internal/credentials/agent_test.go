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

package credentials

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

func caFrom(value string) func() ([]byte, error) {
	return func() ([]byte, error) { return []byte(value), nil }
}

func noCA() ([]byte, error) { return nil, errors.New("no such file") }

// TestTheAddressesArePinnedToTheirSource covers the four literals this proxy carries in place of an
// import — it is its own Go module and cannot depend on the platform's registry package.
//
// Divergence here is not a build error, it is a proxy quietly dialling something else.
func TestTheAddressesArePinnedToTheirSource(t *testing.T) {
	require.Equal(t, "registry.d8-system.svc:5001", storeHost, "go_lib/registry/const: Host")
	require.Equal(t, "system/deckhouse", storePath, "go_lib/registry/const: Path, without its leading slash")
	require.Equal(t, "127.0.0.1:5001", proxyHost, "go_lib/registry/const: ProxyHost")
	require.Equal(t, "/etc/kubernetes/registry-agent/pki/ca.crt", agentCAFile,
		"go_lib/registry/const: AgentCAFile, and the mount path in the deployment")
}

// TestFetchingGoesThroughTheAgent is the model: a client on a node does not reach a registry, it
// reaches the agent, and the agent decides what is behind it.
//
// What this replaces is the previous attempt, which gave this client the in-cluster store's own
// credentials. That worked and was the wrong layer — it tied the proxy to one particular backend,
// and on a cluster with no store at all (Managed with an upstream, cache off) there was nothing to
// read and nothing that answered on the recorded address: `Service/registry` does not exist there.
// Through the agent, both clusters look the same from here.
func TestFetchingGoesThroughTheAgent(t *testing.T) {
	recorded := &registry.ClientConfig{
		Repository: "registry.d8-system.svc:5001/system/deckhouse",
		Scheme:     "https",
		CA:         "SOMETHING-ELSE",
		Auth:       "dXBzdHJlYW06c2VjcmV0",
	}

	dialled := throughTheAgent(recorded, caFrom("AGENT-CA"))

	assert.Equal(t, "127.0.0.1:5001/system/deckhouse", dialled.Repository,
		"the recorded address is a key; the agent is what is dialled")
	assert.Equal(t, "https", dialled.Scheme, "the agent serves TLS whatever is behind it speaks")
	assert.Equal(t, "AGENT-CA", dialled.CA, "its authority is generated on this node")
	assert.Empty(t, dialled.Auth,
		"nothing authenticates to the agent; a docker config is looked up by host, and one carrying "+
			"an entry for the wrong host fails to build a client at all")

	// The configuration held in the map is what the cluster records, and stays that way — it is
	// shared between callers and it is what belongs in anything written down.
	assert.Equal(t, "registry.d8-system.svc:5001/system/deckhouse", recorded.Repository)
	assert.Equal(t, "dXBzdHJlYW06c2VjcmV0", recorded.Auth)
}

// A repository the agent does not serve is left exactly as it was: a ModuleSource has its own
// address, its own authority and its own credentials, and none of them are the agent's business.
func TestAnythingElseIsUntouched(t *testing.T) {
	other := &registry.ClientConfig{
		Repository: "registry.example.com/modules",
		Scheme:     "http",
		CA:         "THEIR-CA",
		Auth:       "dGhlaXJz",
	}

	require.Same(t, other, throughTheAgent(other, caFrom("AGENT-CA")))
}

// Both spellings of the same agent, because a request naming the agent as its own registry is a loop
// and the agent refuses it — so a configuration already pointing at the loopback must not be rewritten
// into one that names it twice.
func TestTheAgentIsRecognisedByEitherSpelling(t *testing.T) {
	already := &registry.ClientConfig{Repository: "127.0.0.1:5001/system/deckhouse"}

	dialled := throughTheAgent(already, caFrom("AGENT-CA"))
	assert.Equal(t, "127.0.0.1:5001/system/deckhouse", dialled.Repository)
	assert.Equal(t, "AGENT-CA", dialled.CA)
}

// TestWithoutAnAgentNothingIsRewritten is the deployment circle, and the reason this check comes
// first rather than last.
//
// The agent is a static pod installed by bashible, and the package it is installed from is fetched
// through this proxy. Dialling the agent before it exists makes the proxy wait for what is waiting
// for it — measured on a fresh cluster as `rpp-get [registry-agent] attempt 14 failed … HTTP 500:
// dial tcp 127.0.0.1:5001: connect: connection refused`, thirty times over, after which no node ever
// joined. The design ADR names this circle explicitly as the hazard the whole rollout is built
// around.
func TestWithoutAnAgentNothingIsRewritten(t *testing.T) {
	recorded := &registry.ClientConfig{
		Repository: "registry.d8-system.svc:5001/system/deckhouse",
		CA:         "AS-RECORDED",
		Auth:       "dXBzdHJlYW0=",
	}

	require.Same(t, recorded, throughTheAgent(recorded, noCA),
		"with no agent on the node the request has to go the way it went before")

	// An empty file is the same answer: the host mount exists on every node, including those where
	// the module manages nothing, and `DirectoryOrCreate` leaves it empty there.
	require.Same(t, recorded, throughTheAgent(recorded, caFrom("")))
}
