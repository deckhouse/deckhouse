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

package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// TestWithPersistedAuthFillsOnlyWhatIsMissing is the rule the hold depends on: reattach what
// the cluster no longer supplies, and never overwrite what it does.
func TestWithPersistedAuthFillsOnlyWhatIsMissing(t *testing.T) {
	// Encoded here rather than written out as base64 literals, so that nothing in this file reads as
	// a real credential to a secret scanner — and so the pair each value stands for is visible in the
	// code instead of in a comment beside an opaque string.
	persisted := map[string]registryv1alpha1.Auth{
		constant.AuthKeyUpstream:                   {Auth: encodedPair("persisted", "primary")},
		mirrorAuthKey(constant.AuthKeyUpstream, 0): {Auth: encodedPair("persisted", "mirror")},
	}

	t.Run("a held upstream is given its credentials back", func(t *testing.T) {
		got := withPersistedAuth(heldUpstream("registry.deckhouse.io"), persisted)

		require.NotNil(t, got.Endpoint.Auth)
		assert.Equal(t, "cGVyc2lzdGVkOnByaW1hcnk=", got.Endpoint.Auth.Auth)
	})

	t.Run("a configured upstream keeps its own", func(t *testing.T) {
		// The case that must not regress: an upstream the configuration supplies arrives
		// with credentials, and replacing them with an older copy would keep a cluster on
		// a rotated license — silently, and for as long as the old one still worked.
		got := withPersistedAuth(testUpstream("registry.deckhouse.io"), persisted)

		require.NotNil(t, got.Endpoint.Auth)
		assert.Equal(t, "license-token", got.Endpoint.Auth.Username)
		assert.Empty(t, got.Endpoint.Auth.Auth)
	})

	t.Run("mirrors are filled under their own keys", func(t *testing.T) {
		held := heldUpstream("registry.deckhouse.io")
		held.Mirrors = []registryv1alpha1.Endpoint{{
			Scheme: registryv1alpha1.SchemeHTTPS, Host: "mirror.example.com", Path: "/deckhouse/ee",
		}}

		got := withPersistedAuth(held, persisted)

		require.Len(t, got.Mirrors, 1)
		require.NotNil(t, got.Mirrors[0].Auth)
		assert.Equal(t, "cGVyc2lzdGVkOm1pcnJvcg==", got.Mirrors[0].Auth.Auth)
	})

	t.Run("the input is not rewritten", func(t *testing.T) {
		// The caller compares the configured upstream against the recorded one to decide
		// whether to probe. Filling this in place would make it compare a value against
		// itself and conclude nothing had changed.
		held := heldUpstream("registry.deckhouse.io")
		_ = withPersistedAuth(held, persisted)

		assert.Nil(t, held.Endpoint.Auth)
	})

	t.Run("nothing to reattach", func(t *testing.T) {
		held := heldUpstream("registry.deckhouse.io")
		assert.Nil(t, withPersistedAuth(held, nil).Endpoint.Auth)
		assert.Nil(t, withPersistedAuth(nil, persisted))
	})
}
