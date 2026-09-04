/*
Copyright 2025 Flant JSC

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

package credentials

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

// TestWithNoAgentAndNoStoreTheAnswerIsAsRecorded pins what is left when neither is available.
//
// The rule this guards is that credentials are never used to reach an UPSTREAM from this process: the
// agent reaches upstreams, everything on the node reaches the agent. What crosses the window before an
// agent exists is the cluster's own store — see TestWithNoAgentTheStoreIsUsed — and that is a different
// thing entirely, since nothing leaves the cluster.
//
// With neither an agent nor a store to read, the recorded configuration is the answer and nothing may be
// substituted for it. That case is real: a cluster whose registry the module does not manage has no store
// at all, and its packages come from wherever it was installed from.
func TestWithNoAgentNoStoreAndNoSeedTheAnswerIsAsRecorded(t *testing.T) {
	recorded := &registry.ClientConfig{
		Repository: "registry.d8-system.svc:5001/system/deckhouse",
		Scheme:     "https",
		CA:         "AS-RECORDED",
		Auth:       "dXBzdHJlYW06c2VjcmV0",
	}

	w := &Watcher{
		registryClientConfigs: map[string]*registry.ClientConfig{
			recorded.Repository: recorded,
		},
	}

	// readAgentCA is the real one, and on a machine running tests there is no agent CA to find — which
	// is exactly the case being asserted.
	got, err := w.Get(recorded.Repository)
	require.NoError(t, err)
	require.Same(t, recorded, got,
		"with no agent the recorded configuration is the answer, and nothing may substitute an upstream for it")
}

// TestNothingReachesAnUpstreamFromHere pins what happens when there is no agent and no store.
//
// Exactly two sources may serve the cluster's own repository, and they are not interchangeable:
//
//	the agent — no credentials leave this process at all, and it knows which backend is live;
//	the store — the cluster's own registry, its own read-only account, nothing leaves the cluster.
//
// There is deliberately no third. A seed used to sit here — what nodes are told, out of
// `registry-bashible-config` — reaching an UPSTREAM with credentials from a secret, and it existed
// because a cache-less cluster had no store to offer and no agent yet. The installer now puts the agent
// on the first master over its own tunnel, so that window is closed where it actually is, and this test
// is what keeps it from being reopened here: with neither source, the answer is the recorded
// configuration, unchanged, and no credentials go anywhere.
func TestWithNoAgentAndNoStoreNothingIsSubstituted(t *testing.T) {
	const repo = "registry.d8-system.svc:5001/system/deckhouse"

	recorded := &registry.ClientConfig{Repository: repo, Scheme: "https", CA: "AS-RECORDED"}
	store := storeAuthority(storeSecret(), repo)
	require.NotNil(t, store)

	withStore := &Watcher{
		registryClientConfigs: map[string]*registry.ClientConfig{repo: recorded},
		fromStoreSecret:       store,
	}
	got, err := withStore.Get(repo)
	require.NoError(t, err)
	assert.Same(t, store, got, "with a store to read, the store is the answer")

	bare := &Watcher{registryClientConfigs: map[string]*registry.ClientConfig{repo: recorded}}
	got, err = bare.Get(repo)
	require.NoError(t, err)
	assert.Same(t, recorded, got, "with neither an agent nor a store, nothing may be substituted")
}

// TestAnUnknownRepositoryIsAnError keeps the absence of an answer distinguishable from an answer.
//
// Worth its own case because every defect this component had was of that shape: a missing value that
// read as "nothing to do" instead of "cannot say".
func TestAnUnknownRepositoryIsAnError(t *testing.T) {
	w := &Watcher{registryClientConfigs: map[string]*registry.ClientConfig{}}

	_, err := w.Get("registry.example.com/whatever")
	require.Error(t, err)
}
