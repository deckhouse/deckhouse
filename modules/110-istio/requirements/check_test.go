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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

func reservedUID1337UsageMessage(container, pod, namespace string) string {
	return fmt.Sprintf(
		"container `%s` in pod `%s` in namespace `%s` is running as UID `1337`",
		container, pod, namespace,
	)
}

func TestIstioOperatorVersionRequirement(t *testing.T) {
	requirements.RemoveValue(minVersionValuesKey)
	t.Run("requirement met", func(t *testing.T) {
		requirements.SaveValue(minVersionValuesKey, "1.21.6")
		ok, err := requirements.CheckRequirement(requirementIstioMinimalVersionKey, "1.21")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("requirement failed", func(t *testing.T) {
		requirements.SaveValue(minVersionValuesKey, "1.13")
		ok, err := requirements.CheckRequirement(requirementIstioMinimalVersionKey, "1.21")
		assert.False(t, ok)
		require.Error(t, err)
	})

	t.Run("Istio is not installed on the cluster", func(t *testing.T) {
		requirements.RemoveValue(minVersionValuesKey)
		ok, err := requirements.CheckRequirement(requirementIstioMinimalVersionKey, "1.21")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	requirements.RemoveValue(isK8sVersionAutomaticKey)
	requirements.RemoveValue(istioToK8sCompatibilityMapKey)
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
}

func TestPodsShouldNotRunAs1337UIDRequirement(t *testing.T) {
	const nsDefault = "default"

	t.Run("passes when usage key absent (empty cluster / no matching pods)", func(t *testing.T) {
		requirements.RemoveValue(reservedUIDUsageKey)
		ok, err := requirements.CheckRequirement(requirementPodsShouldNotRunAs1337UIDKey, "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("passes when usage value is empty slice", func(t *testing.T) {
		requirements.SaveValue(reservedUIDUsageKey, []string{})
		t.Cleanup(func() { requirements.RemoveValue(reservedUIDUsageKey) })
		ok, err := requirements.CheckRequirement(requirementPodsShouldNotRunAs1337UIDKey, "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("fails when usage value has unexpected type", func(t *testing.T) {
		requirements.SaveValue(reservedUIDUsageKey, "not-a-slice")
		t.Cleanup(func() { requirements.RemoveValue(reservedUIDUsageKey) })
		ok, err := requirements.CheckRequirement(requirementPodsShouldNotRunAs1337UIDKey, "")
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected []string")
	})

	t.Run("app container UID 1337 with istio-proxy present (app-pod)", func(t *testing.T) {
		lines := []string{reservedUID1337UsageMessage("app", "app-pod", nsDefault)}
		requirements.SaveValue(reservedUIDUsageKey, lines)
		t.Cleanup(func() { requirements.RemoveValue(reservedUIDUsageKey) })
		ok, err := requirements.CheckRequirement(requirementPodsShouldNotRunAs1337UIDKey, "")
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), lines[0])
	})

	t.Run("only istio-proxy UID 1337, app normal UID — hook removes key (pass)", func(t *testing.T) {
		requirements.RemoveValue(reservedUIDUsageKey)
		ok, err := requirements.CheckRequirement(requirementPodsShouldNotRunAs1337UIDKey, "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("pod without canonical-name label — not collected (pass)", func(t *testing.T) {
		requirements.RemoveValue(reservedUIDUsageKey)
		ok, err := requirements.CheckRequirement(requirementPodsShouldNotRunAs1337UIDKey, "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("pod-level runAsUser 1337 with istio-proxy (pod-level-uid)", func(t *testing.T) {
		lines := []string{reservedUID1337UsageMessage("app", "pod-level-uid", nsDefault)}
		requirements.SaveValue(reservedUIDUsageKey, lines)
		t.Cleanup(func() { requirements.RemoveValue(reservedUIDUsageKey) })
		ok, err := requirements.CheckRequirement(requirementPodsShouldNotRunAs1337UIDKey, "")
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), lines[0])
	})

	t.Run("pod-level 1337 but app overrides UID — not collected (pass)", func(t *testing.T) {
		requirements.RemoveValue(reservedUIDUsageKey)
		ok, err := requirements.CheckRequirement(requirementPodsShouldNotRunAs1337UIDKey, "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("multiple non-proxy containers UID 1337 in one pod (multi-container-pod)", func(t *testing.T) {
		lines := []string{
			reservedUID1337UsageMessage("app", "multi-container-pod", nsDefault),
			reservedUID1337UsageMessage("sidecar", "multi-container-pod", nsDefault),
		}
		requirements.SaveValue(reservedUIDUsageKey, lines)
		t.Cleanup(func() { requirements.RemoveValue(reservedUIDUsageKey) })
		ok, err := requirements.CheckRequirement(requirementPodsShouldNotRunAs1337UIDKey, "")
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), lines[0])
		assert.Contains(t, err.Error(), lines[1])
	})
}
