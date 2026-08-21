/*
Copyright 2023 Flant JSC

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

func TestIstioOperatorVersionRequirement(t *testing.T) {
	t.Cleanup(func() { requirements.RemoveValue(minVersionValuesKey) })

	t.Run("configured 1.25 satisfies requirement", func(t *testing.T) {
		requirements.SaveValue(minVersionValuesKey, "1.25")
		ok, err := requirements.CheckRequirement(requirementIstioMinimalVersionKey, "1.25")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("configured 1.21 fails requirement", func(t *testing.T) {
		requirements.SaveValue(minVersionValuesKey, "1.21.6")
		ok, err := requirements.CheckRequirement(requirementIstioMinimalVersionKey, "1.25")
		assert.False(t, ok)
		require.EqualError(t, err, "installed Istio version '1.21.6' is lower than required")
	})

	t.Run("newer configured version satisfies requirement", func(t *testing.T) {
		requirements.SaveValue(minVersionValuesKey, "1.29")
		ok, err := requirements.CheckRequirement(requirementIstioMinimalVersionKey, "1.25")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("Istio is not installed on the cluster", func(t *testing.T) {
		requirements.RemoveValue(minVersionValuesKey)
		ok, err := requirements.CheckRequirement(requirementIstioMinimalVersionKey, "1.25")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("minimum of multiple configured revisions is checked", func(t *testing.T) {
		requirements.SaveValue(minVersionValuesKey, "1.25")
		ok, err := requirements.CheckRequirement(requirementIstioMinimalVersionKey, "1.25")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	requirements.RemoveValue(isK8sVersionAutomaticKey)
	requirements.RemoveValue(istioToK8sCompatibilityMapKey)
	requirements.RemoveValue(installedVersionsValuesKey)
	requirements.RemoveValue(minVersionValuesKey)
	t.Run("requirement for k8s version pass", func(t *testing.T) {
		requirements.SaveValue(isK8sVersionAutomaticKey, true)
		requirements.SaveValue(minVersionValuesKey, "1.13")
		var mapVersions = map[string][]string{"1.13": {"1.21", "1.20", "1.21"}}
		requirements.SaveValue(istioToK8sCompatibilityMapKey, mapVersions)
		ok, err := requirements.CheckRequirement(requirementDefaultK8sKey, "1.20.0")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("requirement for k8s version failed", func(t *testing.T) {
		requirements.SaveValue(isK8sVersionAutomaticKey, true)
		requirements.SaveValue(minVersionValuesKey, "1.13")
		var mapVersions = map[string][]string{"1.13": {"1.21", "1.20", "1.21"}}
		requirements.SaveValue(istioToK8sCompatibilityMapKey, mapVersions)
		ok, err := requirements.CheckRequirement(requirementDefaultK8sKey, "1.22.0")
		assert.False(t, ok)
		require.Error(t, err)
	})

	t.Run("requirement for k8s version checks all installed versions", func(t *testing.T) {
		requirements.SaveValue(isK8sVersionAutomaticKey, true)
		requirements.SaveValue(installedVersionsValuesKey, []string{"1.25", "1.27"})
		var mapVersions = map[string][]string{
			"1.25": {"1.32", "1.33"},
			"1.27": {"1.32", "1.33", "1.34"},
		}
		requirements.SaveValue(istioToK8sCompatibilityMapKey, mapVersions)
		ok, err := requirements.CheckRequirement(requirementDefaultK8sKey, "1.33.0")
		assert.True(t, ok)
		require.NoError(t, err)

		ok, err = requirements.CheckRequirement(requirementDefaultK8sKey, "1.34.0")
		assert.False(t, ok)
		require.Error(t, err)
	})
}
