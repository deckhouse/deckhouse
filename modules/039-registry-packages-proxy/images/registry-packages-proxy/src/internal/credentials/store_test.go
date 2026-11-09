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
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func storeSecret() map[string][]byte {
	return map[string][]byte{
		"ca.crt":   []byte("-----BEGIN CERTIFICATE-----store"),
		"username": []byte("registry-ro"),
		"password": []byte("secret"),
	}
}

func TestStoreAuthorityCarriesTheStoresOwnAccount(t *testing.T) {
	config := storeAuthority(storeSecret(), storeRepository)

	require.NotNil(t, config)
	assert.Equal(t, "registry.d8-system.svc:4281/system/deckhouse", config.Repository,
		"the store at the port its replicas serve on, not the recorded `:5001`, which is the agent's")
	assert.Equal(t, "https", config.Scheme)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----store", config.CA,
		"the store's authority, not the installer's — after the handover that address is served by the store")

	// Auth travels as base64 of `user:password`, the same encoding a docker config carries.
	decoded, err := base64.StdEncoding.DecodeString(config.Auth)
	require.NoError(t, err)
	assert.Equal(t, "registry-ro:secret", string(decoded))
}

// TestTheStoreIsReadFromTheClientSecret keeps the proxy off the storage pod's own secret.
//
// registry-storage-access carries the same authority and read account, so pointing back at it would
// still work — and would give anything that compromises the proxy the account the replicas write and
// delete with. The proxy's Role in the deckhouse module names registry-storage-client alone, so the
// two have to move together.
func TestTheStoreIsReadFromTheClientSecret(t *testing.T) {
	assert.Equal(t, "registry-storage-client", storeAccessSecret)
}

// TestStoreAuthorityRefusesHalfASecret keeps a partial secret from replacing a working answer.
//
// All three fields or none: an authority with no account authenticates nowhere, an account with no
// authority fails the handshake, and either would be taken as an answer by the caller — which is worse
// than having none, because the caller then stops looking.
func TestStoreAuthorityRefusesHalfASecret(t *testing.T) {
	for _, missing := range []string{"ca.crt", "username", "password"} {
		data := storeSecret()
		delete(data, missing)

		assert.Nil(t, storeAuthority(data, storeRepository),
			"a secret without %q must not be used", missing)
	}
}

// TestWithNoAgentTheStoreIsUsed is the window the whole path exists for.
//
// The agent is installed from a package fetched through this proxy — bashible step 034, `rpp-get
// [registry-agent]` — so on a node whose agent does not exist yet, this process has to reach a registry
// by itself, and the agent cannot be how it does that. Measured with no such path at all: eight failed
// attempts at the package, no agent CA on the node, and a bootstrap that timed out waiting for a worker
// which could never join.
//
// What it reaches is the cluster's OWN store, with the store's read-only account. Reaching an upstream
// stays the agent's business, which is the distinction the owner drew.
func TestWithNoAgentTheStoreIsUsed(t *testing.T) {
	recorded := &registry.ClientConfig{
		Repository: "registry.d8-system.svc:5001/system/deckhouse",
		Scheme:     "https",
		CA:         "INSTALLER-CA",
		Auth:       "aW5zdGFsbGVyOnBhc3M=",
	}
	store := storeAuthority(storeSecret(), storeRepository)
	require.NotNil(t, store)
	withAgentAuthority(t, noCA)

	w := &Watcher{
		registryClientConfigs: map[string]*registry.ClientConfig{recorded.Repository: recorded},
		fromStoreSecret:       store,
	}

	got, err := w.Get(recorded.Repository)
	require.NoError(t, err)
	assert.Same(t, store, got,
		"with no agent on the node the store is how the package is fetched")
	assert.NotEqual(t, recorded.CA, got.CA,
		"and with the store's authority: the installer's describes a registry that address no longer serves")
}

// TestAModuleSourceIsNeverServedFromTheStore keeps the substitution to the one repository it belongs to.
//
// A ModuleSource brings its own address and its own credentials. The store's read-only account
// authorizes nothing there, so answering with it would turn a working fetch into a 401.
func TestAModuleSourceIsNeverServedFromTheStore(t *testing.T) {
	elsewhere := &registry.ClientConfig{
		Repository: "registry.deckhouse.io/deckhouse/ee/modules",
		Scheme:     "https",
		Auth:       "bW9kdWxlOnNvdXJjZQ==",
	}

	w := &Watcher{
		registryClientConfigs: map[string]*registry.ClientConfig{elsewhere.Repository: elsewhere},
		fromStoreSecret:       storeAuthority(storeSecret(), storeRepository),
	}

	got, err := w.Get(elsewhere.Repository)
	require.NoError(t, err)
	assert.Same(t, elsewhere, got, "a third-party source is fetched as it was recorded")
}

// withAgentAuthority makes Get see the agent's authority as readCA returns it, for one test.
func withAgentAuthority(t *testing.T, readCA func() ([]byte, error)) {
	t.Helper()

	previous := agentAuthority
	agentAuthority = readCA
	t.Cleanup(func() { agentAuthority = previous })
}

// watcherWithStore is a watcher that has seen the store access secret and holds the cluster's own
// registry as recorded.
func watcherWithStore(t *testing.T) (*Watcher, *registry.ClientConfig) {
	t.Helper()

	recorded := &registry.ClientConfig{
		Repository: recordedHost + "/" + storePath,
		Scheme:     "https",
		CA:         "AS-RECORDED",
		Auth:       "dXNlcm5hbWU6cGFzc3dvcmQ=",
	}

	w := &Watcher{
		registryClientConfigs: map[string]*registry.ClientConfig{recorded.Repository: recorded},
		logger:                log.NewNop(),
	}
	w.processStoreEvent(watch.Event{
		Type:   watch.Added,
		Object: &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: storeAccessSecret}, Data: storeSecret()},
	})
	require.NotNil(t, w.fromStoreSecret, "a complete store access secret is an answer")

	return w, recorded
}

// TestWithoutAnAgentTheStoreIsDialledAtItsServingPort is the regression e2e job 8069309 found.
//
// The store moved from the masters' node addresses at 5001 to 4281, while the recorded address kept
// its `:5001`, because it is a key in every image reference. This path dialled the recorded address:
// the Service is headless, so that was `<master>:5001`, where only the agent listens, and only on the
// loopback. Every fetch was refused, the agent package was never fetched — on the master or the
// worker — and the bootstrap timed out waiting for a node that could not join.
func TestWithoutAnAgentTheStoreIsDialledAtItsServingPort(t *testing.T) {
	withAgentAuthority(t, noCA)
	w, recorded := watcherWithStore(t)

	got, err := w.Get(recorded.Repository)
	require.NoError(t, err)

	assert.Equal(t, "registry.d8-system.svc:4281/system/deckhouse", got.Repository,
		"the store's serving port under the Service's name, which its certificate covers")
	assert.Equal(t, "https", got.Scheme)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----store", got.CA, "the store's authority")

	// The account is a plain `auth` value, not a docker config keyed by host, so it applies to
	// whichever host is dialled.
	decoded, err := base64.StdEncoding.DecodeString(got.Auth)
	require.NoError(t, err)
	assert.Equal(t, "registry-ro:secret", string(decoded), "the store's read-only account")

	// The request is still matched by the recorded address, and that entry is left as it was.
	assert.Equal(t, "registry.d8-system.svc:5001/system/deckhouse", recorded.Repository)
	assert.Equal(t, "AS-RECORDED", recorded.CA)
}

// TestWithAnAgentTheStoreIsNotDialled keeps the other side of the decision where it was: with the
// agent on the node, the request goes to the agent on the loopback at 5001, whose port did not move,
// and the store's address and account are not used even though they are known.
func TestWithAnAgentTheStoreIsNotDialled(t *testing.T) {
	withAgentAuthority(t, caFrom("AGENT-CA"))
	w, recorded := watcherWithStore(t)

	got, err := w.Get(recorded.Repository)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1:5001/system/deckhouse", got.Repository)
	assert.Equal(t, "AGENT-CA", got.CA)
	assert.Empty(t, got.Auth, "nothing authenticates to the agent")
}
