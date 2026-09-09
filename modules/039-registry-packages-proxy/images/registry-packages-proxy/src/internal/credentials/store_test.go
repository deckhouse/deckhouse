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

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

func storeSecret() map[string][]byte {
	return map[string][]byte{
		"ca.crt":   []byte("-----BEGIN CERTIFICATE-----store"),
		"username": []byte("registry-ro"),
		"password": []byte("secret"),
	}
}

func TestStoreAuthorityCarriesTheStoresOwnAccount(t *testing.T) {
	config := storeAuthority(storeSecret(), storeHost+"/"+storePath)

	require.NotNil(t, config)
	assert.Equal(t, "registry.d8-system.svc:5001/system/deckhouse", config.Repository,
		"the address every image reference in the cluster is built from")
	assert.Equal(t, "https", config.Scheme)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----store", config.CA,
		"the store's authority, not the installer's — after the handover that address is served by the store")

	// Auth travels as base64 of `user:password`, the same encoding a docker config carries.
	decoded, err := base64.StdEncoding.DecodeString(config.Auth)
	require.NoError(t, err)
	assert.Equal(t, "registry-ro:secret", string(decoded))
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

		assert.Nil(t, storeAuthority(data, storeHost+"/"+storePath),
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
	store := storeAuthority(storeSecret(), recorded.Repository)
	require.NotNil(t, store)

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
		fromStoreSecret:       storeAuthority(storeSecret(), "registry.d8-system.svc:5001/system/deckhouse"),
	}

	got, err := w.Get(elsewhere.Repository)
	require.NoError(t, err)
	assert.Same(t, elsewhere, got, "a third-party source is fetched as it was recorded")
}
