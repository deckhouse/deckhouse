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
	requirements.RemoveValue(minVersionValuesKey)
	t.Run("requirement met", func(t *testing.T) {
		requirements.SaveValue(minVersionValuesKey, "1.16.2")
		ok, err := requirements.CheckRequirement(requirementIstioMinimalVersionKey, "1.16")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("requirement failed", func(t *testing.T) {
		requirements.SaveValue(minVersionValuesKey, "1.13")
		ok, err := requirements.CheckRequirement(requirementIstioMinimalVersionKey, "1.16")
		assert.False(t, ok)
		require.Error(t, err)
	})

	t.Run("Istio is not installed on the cluster", func(t *testing.T) {
		requirements.RemoveValue(minVersionValuesKey)
		ok, err := requirements.CheckRequirement(requirementIstioMinimalVersionKey, "1.16")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	requirements.RemoveValue(isK8sVersionAutomaticKey)
	requirements.RemoveValue(istioToK8sCompatibilityMapKey)
	requirements.RemoveValue(minVersionValuesKey)
	t.Run("requirement for k8s version pass", func(t *testing.T) {
		requirements.SaveValue(isK8sVersionAutomaticKey, true)
		requirements.SaveValue(minVersionValuesKey, "1.13")
		var mapVersions = map[string][]string{"1.13": {"1.19", "1.20", "1.21"}}
		requirements.SaveValue(istioToK8sCompatibilityMapKey, mapVersions)
		ok, err := requirements.CheckRequirement(requirementDefaultK8sKey, "1.20.0")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("requirement for k8s version failed", func(t *testing.T) {
		requirements.SaveValue(isK8sVersionAutomaticKey, true)
		requirements.SaveValue(minVersionValuesKey, "1.13")
		var mapVersions = map[string][]string{"1.13": {"1.19", "1.20", "1.21"}}
		requirements.SaveValue(istioToK8sCompatibilityMapKey, mapVersions)
		ok, err := requirements.CheckRequirement(requirementDefaultK8sKey, "1.22.0")
		assert.False(t, ok)
		require.Error(t, err)
	})
}
