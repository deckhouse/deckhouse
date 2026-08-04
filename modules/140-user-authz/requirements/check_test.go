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

func TestLegacyRBACv2CustomRolesRequirement(t *testing.T) {
	// Pins the duplicated value-key literal to the hooks package contract (see the counterpart
	// assertion in hooks/discovery_legacy_custom_roles_test.go).
	assert.Equal(t, "userAuthz:legacyRBACv2CustomRoles", legacyRBACv2CustomRolesValueKey)

	t.Run("no value stored (module disabled or not synced) — pass", func(t *testing.T) {
		requirements.RemoveValue(legacyRBACv2CustomRolesValueKey)
		ok, err := requirements.CheckRequirement(legacyRBACv2CustomRolesRequirementKey, "0")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("no legacy roles in the cluster — pass", func(t *testing.T) {
		requirements.SaveValue(legacyRBACv2CustomRolesValueKey, []string{})
		ok, err := requirements.CheckRequirement(legacyRBACv2CustomRolesRequirementKey, "0")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("legacy roles present — block with names in the error", func(t *testing.T) {
		requirements.SaveValue(legacyRBACv2CustomRolesValueKey, []string{
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
		requirements.SaveValue(legacyRBACv2CustomRolesValueKey, []any{"custom:manage:mycustom:manager"})
		ok, err := requirements.CheckRequirement(legacyRBACv2CustomRolesRequirementKey, "0")
		assert.False(t, ok)
		require.Error(t, err)
	})

	t.Run("unparsable requirement value — error", func(t *testing.T) {
		requirements.SaveValue(legacyRBACv2CustomRolesValueKey, []string{"custom:manage:mycustom:manager"})
		ok, err := requirements.CheckRequirement(legacyRBACv2CustomRolesRequirementKey, "not-a-number")
		assert.False(t, ok)
		require.Error(t, err)
	})

	requirements.RemoveValue(legacyRBACv2CustomRolesValueKey)
}
