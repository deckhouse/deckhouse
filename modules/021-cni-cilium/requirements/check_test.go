/*
Copyright 2024 Flant JSC

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
	"github.com/deckhouse/deckhouse/modules/021-cni-cilium/hooks"
)

func TestKubernetesVersionRequirement(t *testing.T) {
	t.Run("complies with the requirements", func(t *testing.T) {
		requirements.SaveValue("cniConfigurationSettled", "")
		ok, err := requirements.CheckRequirement("cniConfigurationSettled", "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("fail: Misconfigured", func(t *testing.T) {
		requirements.SaveValue("cniConfigurationSettled", "false")
		ok, err := requirements.CheckRequirement("cniConfigurationSettled", "")
		assert.False(t, ok)
		require.Error(t, err)
	})
}

func TestLinuxKernelVersionRequirement(t *testing.T) {
	t.Run("SUCCESS: The version is above the required ", func(t *testing.T) {
		requirements.SaveValue("currentMinimalLinuxKernelVersion", "5.10.0-90-generic")

		ok, err := requirements.CheckRequirement("nodesMinimalLinuxKernelVersion", "5.8.0")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("SUCCESS: The version is equal the required ", func(t *testing.T) {
		requirements.SaveValue("currentMinimalLinuxKernelVersion", "5.8.0-90-generic")

		ok, err := requirements.CheckRequirement("nodesMinimalLinuxKernelVersion", "5.8.0")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("FAIL: The version is below the required ", func(t *testing.T) {
		requirements.SaveValue("currentMinimalLinuxKernelVersion", "5.2.0-90-generic")

		ok, err := requirements.CheckRequirement("nodesMinimalLinuxKernelVersion", "5.8.0")
		assert.False(t, ok)
		require.Error(t, err)
	})
}

func TestCiliumReadyForUpgradeToRequirement(t *testing.T) {
	t.Run("SUCCESS: No policies with requires", func(t *testing.T) {
		requirements.SaveValue(hooks.PoliciesWithRequiresRequirementKey, 0)

		ok, err := requirements.CheckRequirement("ciliumReadyForUpgradeTo", "1.20")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("SUCCESS: Check is not requested by the release", func(t *testing.T) {
		requirements.SaveValue(hooks.PoliciesWithRequiresRequirementKey, 2)

		ok, err := requirements.CheckRequirement("ciliumReadyForUpgradeTo", "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("SUCCESS: Target version still supports requires", func(t *testing.T) {
		requirements.SaveValue(hooks.PoliciesWithRequiresRequirementKey, 2)

		ok, err := requirements.CheckRequirement("ciliumReadyForUpgradeTo", "1.18")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("FAIL: Policies with requires exist", func(t *testing.T) {
		requirements.SaveValue(hooks.PoliciesWithRequiresRequirementKey, 2)

		ok, err := requirements.CheckRequirement("ciliumReadyForUpgradeTo", "1.20")
		assert.False(t, ok)
		require.Error(t, err)
	})

	t.Run("FAIL: Invalid target version", func(t *testing.T) {
		requirements.SaveValue(hooks.PoliciesWithRequiresRequirementKey, 0)

		ok, err := requirements.CheckRequirement("ciliumReadyForUpgradeTo", "true")
		assert.False(t, ok)
		require.Error(t, err)
	})
}
