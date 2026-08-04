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

func TestGetDesiredConfiguration(t *testing.T) {
	t.Run("parses spec written by the hook", func(t *testing.T) {
		cm := makeCM(map[string]string{
			"spec": "desiredVersion: \"1.35\"\nupdateMode: Manual\n",
		})

		cfg, err := getDesiredConfiguration(cm)
		require.NoError(t, err)
		assert.Equal(t, "1.35", cfg.DesiredVersion)
		assert.Equal(t, cluster.UpdateModeManual, cfg.UpdateMode)
	})

	t.Run("Automatic mode", func(t *testing.T) {
		cm := makeCM(map[string]string{
			"spec": "desiredVersion: \"1.34\"\nupdateMode: Automatic\n",
		})

		cfg, err := getDesiredConfiguration(cm)
		require.NoError(t, err)
		assert.Equal(t, "1.34", cfg.DesiredVersion)
		assert.Equal(t, cluster.UpdateModeAutomatic, cfg.UpdateMode)
	})

	t.Run("normalizes a full semantic version", func(t *testing.T) {
		cm := makeCM(map[string]string{
			"spec": "desiredVersion: \"v1.35.4\"\nupdateMode: Manual\n",
		})

		cfg, err := getDesiredConfiguration(cm)
		require.NoError(t, err)
		assert.Equal(t, "1.35", cfg.DesiredVersion)
	})

	t.Run("rejects an unknown updateMode instead of degrading silently", func(t *testing.T) {
		// desiredVersion is checked three times; updateMode used to be cast straight into the
		// typed constant, where a typo matched neither value and quietly weakened the
		// Automatic-mode drift detection in cluster/state.go.
		cm := makeCM(map[string]string{
			"spec": "desiredVersion: \"1.35\"\nupdateMode: automatic\n",
		})

		_, err := getDesiredConfiguration(cm)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spec.updateMode")
	})

	t.Run("an absent updateMode is still accepted", func(t *testing.T) {
		cm := makeCM(map[string]string{
			"spec": "desiredVersion: \"1.35\"\n",
		})

		cfg, err := getDesiredConfiguration(cm)
		require.NoError(t, err)
		assert.Equal(t, cluster.UpdateMode(""), cfg.UpdateMode)
	})

	t.Run("tolerates surrounding whitespace in both fields", func(t *testing.T) {
		// data.spec is a hand-editable ConfigMap field now, not a base64 Secret value, and the
		// global hook trims the same values on its side.
		cm := makeCM(map[string]string{
			"spec": "desiredVersion: \"  1.35  \"\nupdateMode: \"  Manual  \"\n",
		})

		cfg, err := getDesiredConfiguration(cm)
		require.NoError(t, err)
		assert.Equal(t, "1.35", cfg.DesiredVersion)
		assert.Equal(t, cluster.UpdateModeManual, cfg.UpdateMode)
	})

	t.Run("missing spec key errors", func(t *testing.T) {
		cm := makeCM(map[string]string{})

		_, err := getDesiredConfiguration(cm)
		require.Error(t, err)
	})

	t.Run("empty desiredVersion errors", func(t *testing.T) {
		cm := makeCM(map[string]string{
			"spec": "desiredVersion: \"\"\nupdateMode: Automatic\n",
		})

		_, err := getDesiredConfiguration(cm)
		require.Error(t, err)
	})

	t.Run("invalid desiredVersion errors", func(t *testing.T) {
		cm := makeCM(map[string]string{
			"spec": "desiredVersion: invalid\nupdateMode: Automatic\n",
		})

		_, err := getDesiredConfiguration(cm)
		require.Error(t, err)
	})
}

func TestFillConfigMapPreservesRawSpec(t *testing.T) {
	rawSpec := "updateMode: Manual\ndesiredVersion: \"1.35\"\n"
	cm := makeCM(map[string]string{"spec": rawSpec})

	got, err := fillConfigMap(cm, &cluster.State{}, ReconcileTriggerIdle)
	require.NoError(t, err)
	assert.Equal(t, rawSpec, got.Data["spec"])
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
}
