/*
Copyright 2022 Flant JSC

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

func TestNodeOSVersionRequirement(t *testing.T) {
	requirements.RemoveValue(minUbuntuVersionValuesKey)
	requirements.RemoveValue(minDebianVersionValuesKey)
	t.Run("requirement met", func(t *testing.T) {
		requirements.SaveValue(minUbuntuVersionValuesKey, "18.4.5")
		ok, err := requirements.CheckRequirement(requirementsUbuntuKey, "18.04")
		assert.True(t, ok)
		require.NoError(t, err)
	})
	t.Run("requirement met", func(t *testing.T) {
		requirements.SaveValue(minDebianVersionValuesKey, "11")
		ok, err := requirements.CheckRequirement(requirementsDebianKey, "10")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("requirement failed", func(t *testing.T) {
		requirements.SaveValue(minUbuntuVersionValuesKey, "16.4.5")
		ok, err := requirements.CheckRequirement(requirementsUbuntuKey, "18.04")
		assert.False(t, ok)
		require.Error(t, err)
	})

	t.Run("requirement failed", func(t *testing.T) {
		requirements.SaveValue(minDebianVersionValuesKey, "9")
		ok, err := requirements.CheckRequirement(requirementsDebianKey, "10")
		assert.False(t, ok)
		require.Error(t, err)
	})

	t.Run("containerd requirement runs successfully", func(t *testing.T) {
		requirements.SaveValue(hasNodesWithDocker, false)
		ok, err := requirements.CheckRequirement(containerdRequirementsKey, "true")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("containerd requirement fails", func(t *testing.T) {
		requirements.SaveValue(hasNodesWithDocker, true)
		ok, err := requirements.CheckRequirement(containerdRequirementsKey, "true")
		assert.False(t, ok)
		require.Error(t, err)
	})
}
