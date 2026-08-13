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

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"control-plane-manager/internal/controllers/update-observer/cluster"
)

func makeCM(data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-kubernetes", Namespace: "kube-system"},
		Data:       data,
	}
}

func setVersionEnv(t *testing.T, desired, updateMode, maxUsed string) {
	t.Helper()
	t.Setenv("DESIRED_KUBERNETES_VERSION", desired)
	t.Setenv("KUBERNETES_UPDATE_MODE", updateMode)
	t.Setenv("MAX_USED_KUBERNETES_VERSION", maxUsed)
}

func TestDesiredConfiguration(t *testing.T) {
	t.Run("builds the spec from the environment", func(t *testing.T) {
		setVersionEnv(t, "1.35", "Manual", "1.35")

		cfg, err := desiredConfiguration(makeCM(nil))
		require.NoError(t, err)
		assert.Equal(t, "1.35", cfg.DesiredVersion)
		assert.Equal(t, cluster.UpdateModeManual, cfg.UpdateMode)
		assert.Equal(t, "1.35", cfg.MaxUsedVersion)
	})

	t.Run("Automatic mode", func(t *testing.T) {
		setVersionEnv(t, "1.34", "Automatic", "1.34")

		cfg, err := desiredConfiguration(makeCM(nil))
		require.NoError(t, err)
		assert.Equal(t, cluster.UpdateModeAutomatic, cfg.UpdateMode)
	})

	t.Run("normalizes full semantic versions and surrounding whitespace", func(t *testing.T) {
		setVersionEnv(t, " v1.35.4 ", " Manual ", " v1.36.0 ")

		cfg, err := desiredConfiguration(makeCM(nil))
		require.NoError(t, err)
		assert.Equal(t, "1.35", cfg.DesiredVersion)
		assert.Equal(t, cluster.UpdateModeManual, cfg.UpdateMode)
		assert.Equal(t, "1.36", cfg.MaxUsedVersion)
	})

	// The recorded maximum must never walk backwards. A Pod whose template still carries the
	// previous value — mid-rollout, or after leadership moved to a not-yet-updated Pod — would
	// otherwise lower the floor and hand a downgrade an extra minor of room.
	t.Run("keeps the higher of the stored and the declared maxUsed", func(t *testing.T) {
		setVersionEnv(t, "1.33", "Manual", "1.34")

		cfg, err := desiredConfiguration(makeCM(map[string]string{
			"spec": "desiredVersion: \"1.33\"\nupdateMode: Manual\nmaxUsedKubernetesVersion: \"1.36\"\n",
		}))
		require.NoError(t, err)
		assert.Equal(t, "1.36", cfg.MaxUsedVersion)
	})

	t.Run("raises the stored maxUsed when the environment is ahead", func(t *testing.T) {
		setVersionEnv(t, "1.36", "Manual", "1.36")

		cfg, err := desiredConfiguration(makeCM(map[string]string{
			"spec": "desiredVersion: \"1.35\"\nupdateMode: Manual\nmaxUsedKubernetesVersion: \"1.35\"\n",
		}))
		require.NoError(t, err)
		assert.Equal(t, "1.36", cfg.MaxUsedVersion)
	})

	t.Run("an unreadable stored spec does not block the reconcile that repairs it", func(t *testing.T) {
		setVersionEnv(t, "1.35", "Manual", "1.35")

		cfg, err := desiredConfiguration(makeCM(map[string]string{"spec": "desiredVersion: [broken\n"}))
		require.NoError(t, err)
		assert.Equal(t, "1.35", cfg.MaxUsedVersion)
	})

	// Each of these would otherwise be written into the ConfigMap as if it were declared, and read
	// back from there by node-controller, the release requirements check and two webhooks.
	t.Run("rejects malformed environment instead of defaulting", func(t *testing.T) {
		for name, env := range map[string][3]string{
			"empty desiredVersion":   {"", "Manual", "1.35"},
			"empty updateMode":       {"1.35", "", "1.35"},
			"empty maxUsed":          {"1.35", "Manual", ""},
			"unknown updateMode":     {"1.35", "automatic", "1.35"},
			"invalid desiredVersion": {"not-a-version", "Manual", "1.35"},
			"invalid maxUsed":        {"1.35", "Manual", "not-a-version"},
		} {
			t.Run(name, func(t *testing.T) {
				setVersionEnv(t, env[0], env[1], env[2])

				_, err := desiredConfiguration(makeCM(nil))
				require.Error(t, err)
			})
		}
	})
}

// The whole data.spec block is authored here now, so a hand edit must be corrected rather than
// preserved — the opposite of the byte-for-byte passthrough this controller used to do.
func TestFillConfigMapRewritesSpec(t *testing.T) {
	cm := makeCM(map[string]string{"spec": "desiredVersion: \"1.32\"\nupdateMode: Automatic\n"})

	got, err := fillConfigMap(cm, &cluster.State{
		Spec: cluster.Spec{
			DesiredVersion: "1.35",
			UpdateMode:     cluster.UpdateModeManual,
			MaxUsedVersion: "1.36",
		},
	}, ReconcileTriggerIdle)
	require.NoError(t, err)

	assert.Contains(t, got.Data["spec"], "desiredVersion: \"1.35\"")
	assert.Contains(t, got.Data["spec"], "updateMode: Manual")
	assert.Contains(t, got.Data["spec"], "maxUsedKubernetesVersion: \"1.36\"")
	assert.Equal(t, "d8-cluster-kubernetes", got.Labels["name"])
	assert.Equal(t, "deckhouse", got.Labels["heritage"])
}

// getConfigMapSpecPredicate is the only thing standing between this controller and an infinite
// self-trigger loop (fillConfigMap stamps an annotation on every pass), and it had no unit test.
func TestGetConfigMapSpecPredicate(t *testing.T) {
	pred := getConfigMapSpecPredicate()

	withData := func(data map[string]string) *corev1.ConfigMap { return makeCM(data) }

	specA := map[string]string{"spec": "desiredVersion: \"1.35\"\nupdateMode: Manual\n", "status": "currentVersion: \"1.35\"\n"}

	t.Run("ignores status-only changes so the controller does not retrigger itself", func(t *testing.T) {
		older := withData(specA)
		newer := withData(map[string]string{"spec": specA["spec"], "status": "currentVersion: \"1.34\"\n"})
		newer.Annotations = map[string]string{"lastReconciliationTime": "now"}

		assert.False(t, pred.Update(event.UpdateEvent{ObjectOld: older, ObjectNew: newer}))
	})

	t.Run("reacts to a spec change from the hook", func(t *testing.T) {
		older := withData(specA)
		newer := withData(map[string]string{"spec": "desiredVersion: \"1.36\"\nupdateMode: Manual\n", "status": specA["status"]})

		assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: older, ObjectNew: newer}))
	})

	// Once the cluster is UpToDate, Reconcile returns with no requeue, so data.spec is the only
	// remaining wake-up. Without this, an externally wiped status would never be restored.
	t.Run("reacts when status is wiped externally", func(t *testing.T) {
		older := withData(specA)
		newer := withData(map[string]string{"spec": specA["spec"]})

		assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: older, ObjectNew: newer}))
	})

	t.Run("ignores a same-named ConfigMap in another namespace", func(t *testing.T) {
		older := withData(specA)
		newer := withData(map[string]string{"spec": "desiredVersion: \"1.36\"\nupdateMode: Manual\n"})
		newer.Namespace = "default"

		assert.False(t, pred.Update(event.UpdateEvent{ObjectOld: older, ObjectNew: newer}))
	})

	// dhctl creates this object and the label-objects admission policy keeps it, so the controller
	// no longer carries a recreate path — and a reconcile that cannot read it only logs and requeues.
	// Deletion is therefore not something to react to.
	t.Run("ignores deletion", func(t *testing.T) {
		assert.False(t, pred.Delete(event.DeleteEvent{Object: withData(specA)}))
	})

	t.Run("ignores deletion of a same-named ConfigMap in another namespace", func(t *testing.T) {
		other := withData(specA)
		other.Namespace = "default"

		assert.False(t, pred.Delete(event.DeleteEvent{Object: other}))
	})
}
