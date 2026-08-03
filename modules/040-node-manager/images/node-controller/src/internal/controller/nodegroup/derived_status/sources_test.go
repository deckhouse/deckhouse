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

package derived_status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func kubernetesSourceConfigMap(desiredVersion string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: clusterConfigSecretNamespace, Name: clusterKubernetesConfigMapName},
		Data:       map[string]string{"spec": "desiredVersion: " + desiredVersion + "\nupdateMode: Manual\n"},
	}
}

func resolvedTarget(t *testing.T, objects ...client.Object) string {
	t.Helper()
	service := newTestServiceRaw(t, objects...)
	target, err := service.readTargetKubernetesVersion(context.Background())
	require.NoError(t, err)
	require.NotNil(t, target)
	return semverMajMin(target)
}

func TestReadTargetKubernetesVersion(t *testing.T) {
	t.Run("ConfigMap desiredVersion is the only source", func(t *testing.T) {
		assert.Equal(t, "1.35", resolvedTarget(t, kubernetesSourceConfigMap("1.35")))
	})

	t.Run("missing ConfigMap requeues", func(t *testing.T) {
		_, err := newTestServiceRaw(t).readTargetKubernetesVersion(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), clusterKubernetesConfigMapName)
	})

	t.Run("empty desiredVersion requeues", func(t *testing.T) {
		cm := kubernetesSourceConfigMap("")
		cm.Data["spec"] = "desiredVersion: \"\"\nupdateMode: Automatic\n"
		_, err := newTestServiceRaw(t, cm).readTargetKubernetesVersion(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "desiredVersion")
	})

	t.Run("missing spec requeues", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: clusterConfigSecretNamespace, Name: clusterKubernetesConfigMapName},
			Data:       map[string]string{},
		}
		_, err := newTestServiceRaw(t, cm).readTargetKubernetesVersion(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spec")
	})

	t.Run("invalid desiredVersion requeues", func(t *testing.T) {
		_, err := newTestServiceRaw(t, kubernetesSourceConfigMap("not-a-version")).readTargetKubernetesVersion(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "desiredVersion")
	})

	t.Run("unparsable ConfigMap spec requeues", func(t *testing.T) {
		configMap := kubernetesSourceConfigMap("1.35")
		configMap.Data["spec"] = "desiredVersion: [broken\n"
		_, err := newTestServiceRaw(t, configMap).readTargetKubernetesVersion(context.Background())
		require.Error(t, err)
	})
}
