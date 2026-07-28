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
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
)

func TestKubernetesVersionRequirement(t *testing.T) {
	t.Run("requirement met", func(t *testing.T) {
		requirements.SaveValue(minK8sVersionRequirementKey, "1.19.16")
		ok, err := requirements.CheckRequirement("k8s", "1.19")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("requirement failed", func(t *testing.T) {
		requirements.SaveValue(minK8sVersionRequirementKey, "1.18.3")
		ok, err := requirements.CheckRequirement("k8s", "1.19")
		assert.False(t, ok)
		require.Error(t, err)
	})
}

func TestAutoKubernetesVersionRequirement(t *testing.T) {
	t.Run("requirement met", func(t *testing.T) {
		requirements.SaveValue(hooks.K8sVersionsWithDeprecations, "1.22,1.23,1.24")
		ok, err := requirements.CheckRequirement("autoK8sVersion", "1.21")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("requirement failed", func(t *testing.T) {
		requirements.SaveValue(hooks.K8sVersionsWithDeprecations, "1.22,1.23,1.25")
		ok, err := requirements.CheckRequirement("autoK8sVersion", "1.30")
		assert.False(t, ok)
		require.Error(t, err)
	})

	t.Run("requirement initial", func(t *testing.T) {
		requirements.SaveValue(hooks.K8sVersionsWithDeprecations, "initial")
		ok, err := requirements.CheckRequirement("autoK8sVersion", "1.30")
		assert.False(t, ok)
		require.Error(t, err)
	})
}

func TestKubernetesVersionMigratedRequirement(t *testing.T) {
	t.Run("gate disarmed — empty requirement", func(t *testing.T) {
		requirements.SaveValue(hooks.KubernetesVersionMigratedRequirementKey, false)
		ok, err := requirements.CheckRequirement(kubernetesVersionMigratedRequirementsKey, "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("gate disarmed — false requirement", func(t *testing.T) {
		requirements.SaveValue(hooks.KubernetesVersionMigratedRequirementKey, false)
		ok, err := requirements.CheckRequirement(kubernetesVersionMigratedRequirementsKey, "false")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("gate disarmed — boolean false variants", func(t *testing.T) {
		requirements.SaveValue(hooks.KubernetesVersionMigratedRequirementKey, false)
		for _, value := range []string{"False", "FALSE", "0"} {
			ok, err := requirements.CheckRequirement(kubernetesVersionMigratedRequirementsKey, value)
			assert.True(t, ok, value)
			require.NoError(t, err, value)
		}
	})

	t.Run("invalid boolean requirement is rejected", func(t *testing.T) {
		ok, err := requirements.CheckRequirement(kubernetesVersionMigratedRequirementsKey, "no")
		assert.False(t, ok)
		require.Error(t, err)
	})

	t.Run("migrated — requirement met", func(t *testing.T) {
		requirements.SaveValue(hooks.KubernetesVersionMigratedRequirementKey, true)
		ok, err := requirements.CheckRequirement(kubernetesVersionMigratedRequirementsKey, "true")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("not migrated — requirement failed with migration command", func(t *testing.T) {
		requirements.SaveValue(hooks.KubernetesVersionMigratedRequirementKey, false)
		ok, err := requirements.CheckRequirement(kubernetesVersionMigratedRequirementsKey, "true")
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "d8 k patch moduleconfig control-plane-manager")
		assert.Contains(t, err.Error(), `awk '{gsub(/"/, "", $2); print $2}'`)
		assert.Contains(t, err.Error(), "d8 system edit cluster-configuration")
	})

	t.Run("value not published yet — fail open", func(t *testing.T) {
		requirements.RemoveValue(hooks.KubernetesVersionMigratedRequirementKey)
		ok, err := requirements.CheckRequirement(kubernetesVersionMigratedRequirementsKey, "true")
		assert.True(t, ok)
		require.NoError(t, err)
	})
}
