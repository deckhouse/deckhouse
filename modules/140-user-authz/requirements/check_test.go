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
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks"
)

func TestLegacyRBACv2CustomRolesRequirement(t *testing.T) {
	t.Run("no value stored (module disabled or not synced) — pass", func(t *testing.T) {
		requirements.RemoveValue(hooks.LegacyRBACv2CustomRolesValueKey)
		ok, err := requirements.CheckRequirement(legacyRBACv2CustomRolesRequirementKey, "0")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("no legacy roles in the cluster — pass", func(t *testing.T) {
		requirements.SaveValue(hooks.LegacyRBACv2CustomRolesValueKey, []string{})
		ok, err := requirements.CheckRequirement(legacyRBACv2CustomRolesRequirementKey, "0")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("legacy roles present — block with names in the error", func(t *testing.T) {
		requirements.SaveValue(hooks.LegacyRBACv2CustomRolesValueKey, []string{
			"custom:manage:mycustom:manager",
			"custom:use:capability:mycustom:superresource:view",
		})
		ok, err := requirements.CheckRequirement(legacyRBACv2CustomRolesRequirementKey, "0")
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "custom:manage:mycustom:manager")
		assert.Contains(t, err.Error(), "custom:use:capability:mycustom:superresource:view")
		assert.Contains(t, err.Error(), "d8:custom:")
	})

	t.Run("deserialized []any representation is tolerated", func(t *testing.T) {
		requirements.SaveValue(hooks.LegacyRBACv2CustomRolesValueKey, []any{"custom:manage:mycustom:manager"})
		ok, err := requirements.CheckRequirement(legacyRBACv2CustomRolesRequirementKey, "0")
		assert.False(t, ok)
		require.Error(t, err)
	})

	t.Run("unparsable requirement value — error", func(t *testing.T) {
		requirements.SaveValue(hooks.LegacyRBACv2CustomRolesValueKey, []string{"custom:manage:mycustom:manager"})
		ok, err := requirements.CheckRequirement(legacyRBACv2CustomRolesRequirementKey, "not-a-number")
		assert.False(t, ok)
		require.Error(t, err)
	})

	requirements.RemoveValue(hooks.LegacyRBACv2CustomRolesValueKey)
}

func TestDeprecatedRBACv2BindingsRequirement(t *testing.T) {
	t.Run("no value stored (module disabled or not synced) — pass", func(t *testing.T) {
		requirements.RemoveValue(hooks.DeprecatedRBACv2BindingsValueKey)
		ok, err := requirements.CheckRequirement(deprecatedRBACv2BindingsRequirementKey, "0")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("no bindings to deprecated names — pass", func(t *testing.T) {
		requirements.SaveValue(hooks.DeprecatedRBACv2BindingsValueKey, []string{})
		ok, err := requirements.CheckRequirement(deprecatedRBACv2BindingsRequirementKey, "0")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("bindings present — block with the bindings in the error", func(t *testing.T) {
		requirements.SaveValue(hooks.DeprecatedRBACv2BindingsValueKey, []string{
			"ClusterRoleBinding legacy-observability -> d8:manage:observability:manager",
			"RoleBinding team-a/legacy-viewer -> d8:use:role:viewer",
		})
		ok, err := requirements.CheckRequirement(deprecatedRBACv2BindingsRequirementKey, "0")
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ClusterRoleBinding legacy-observability -> d8:manage:observability:manager")
		assert.Contains(t, err.Error(), "RoleBinding team-a/legacy-viewer -> d8:use:role:viewer")
		assert.Contains(t, err.Error(), "Deprecated role names")
	})

	t.Run("deserialized []any representation is tolerated", func(t *testing.T) {
		requirements.SaveValue(hooks.DeprecatedRBACv2BindingsValueKey, []any{"RoleBinding team-a/legacy-viewer -> d8:use:role:viewer"})
		ok, err := requirements.CheckRequirement(deprecatedRBACv2BindingsRequirementKey, "0")
		assert.False(t, ok)
		require.Error(t, err)
	})

	t.Run("unparsable requirement value — error", func(t *testing.T) {
		requirements.SaveValue(hooks.DeprecatedRBACv2BindingsValueKey, []string{"RoleBinding team-a/legacy-viewer -> d8:use:role:viewer"})
		ok, err := requirements.CheckRequirement(deprecatedRBACv2BindingsRequirementKey, "not-a-number")
		assert.False(t, ok)
		require.Error(t, err)
	})

	requirements.RemoveValue(hooks.DeprecatedRBACv2BindingsValueKey)
}
