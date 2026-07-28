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
