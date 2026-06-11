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

package requirements

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

func TestLegacyRBACBindingsRequirement(t *testing.T) {
	t.Run("no value saved, requirement met", func(t *testing.T) {
		requirements.RemoveValue(legacyRBACBindingsCountKey)
		requirements.RemoveValue(legacyRBACBindingsListKey)
		ok, err := requirements.CheckRequirement(hasLegacyRBACBindingsKey, "false")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("zero legacy bindings, requirement met", func(t *testing.T) {
		requirements.SaveValue(legacyRBACBindingsCountKey, 0)
		ok, err := requirements.CheckRequirement(hasLegacyRBACBindingsKey, "false")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("legacy bindings exist, requirement failed", func(t *testing.T) {
		requirements.SaveValue(legacyRBACBindingsCountKey, 2)
		requirements.SaveValue(legacyRBACBindingsListKey, "ClusterRoleBinding/my-admins, RoleBinding/default/my-users")
		ok, err := requirements.CheckRequirement(hasLegacyRBACBindingsKey, "false")
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ClusterRoleBinding/my-admins")
	})

	t.Run("unexpected requirement value, requirement met", func(t *testing.T) {
		requirements.SaveValue(legacyRBACBindingsCountKey, 2)
		ok, err := requirements.CheckRequirement(hasLegacyRBACBindingsKey, "true")
		assert.True(t, ok)
		require.NoError(t, err)
	})
}
