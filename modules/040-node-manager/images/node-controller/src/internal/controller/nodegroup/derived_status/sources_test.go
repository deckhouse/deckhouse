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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func kubernetesSourceSecret(ccVersion, defaultVersion string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: clusterConfigSecretNamespace, Name: clusterConfigSecretName},
		Data: map[string][]byte{
			"cluster-configuration.yaml":  []byte("kubernetesVersion: " + ccVersion + "\n"),
			deckhouseDefaultK8sVersionKey: []byte(defaultVersion),
		},
	}
}

func kubernetesSourceModuleConfig(version string) *unstructured.Unstructured {
	moduleConfig := &unstructured.Unstructured{}
	moduleConfig.SetGroupVersionKind(moduleConfigGVK)
	moduleConfig.SetName(controlPlaneManagerModuleName)
	_ = unstructured.SetNestedField(moduleConfig.Object, version, "spec", "settings", "kubernetesVersion")
	return moduleConfig
}

func kubernetesSourceConfigMap(desiredVersion string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: clusterConfigSecretNamespace, Name: clusterKubernetesConfigMapName},
		Data:       map[string]string{"spec": "desiredVersion: " + desiredVersion + "\nupdateMode: Manual\n"},
	}
}

func resolvedTarget(t *testing.T, objects ...client.Object) string {
	t.Helper()
	service := newTestService(t, objects...)
	ccTarget, _, deckhouseDefault := service.readClusterConfiguration(context.Background())
	target := service.readTargetKubernetesVersion(context.Background(), ccTarget, deckhouseDefault)
	require.NotNil(t, target)
	return semverMajMin(target)
}

func TestReadTargetKubernetesVersion(t *testing.T) {
	t.Run("ConfigMap desiredVersion is the primary source", func(t *testing.T) {
		assert.Equal(t, "1.35", resolvedTarget(t,
			kubernetesSourceSecret("1.31", "1.34"),
			kubernetesSourceModuleConfig("1.33"),
			kubernetesSourceConfigMap("1.35"),
		))
	})

	t.Run("pinned ModuleConfig wins over ClusterConfiguration before ConfigMap sync", func(t *testing.T) {
		assert.Equal(t, "1.33", resolvedTarget(t,
			kubernetesSourceSecret("1.31", "1.34"),
			kubernetesSourceModuleConfig("1.33"),
		))
	})

	t.Run("ModuleConfig Automatic ignores leftover ClusterConfiguration pin", func(t *testing.T) {
		assert.Equal(t, "1.34", resolvedTarget(t,
			kubernetesSourceSecret("1.31", "1.34"),
			kubernetesSourceModuleConfig(automaticKubernetesVersion),
		))
	})

	t.Run("ClusterConfiguration remains the final compatibility fallback", func(t *testing.T) {
		assert.Equal(t, "1.31", resolvedTarget(t,
			kubernetesSourceSecret("1.31", "1.34"),
		))
	})

	// A source anyone with kube-system write access can corrupt must not be able to stop the
	// derived status of every NodeGroup in the cluster — it may only disqualify itself.
	t.Run("unparsable ConfigMap spec degrades to the next source", func(t *testing.T) {
		configMap := kubernetesSourceConfigMap("1.35")
		configMap.Data["spec"] = "desiredVersion: [broken\n"

		assert.Equal(t, "1.33", resolvedTarget(t,
			kubernetesSourceSecret("1.31", "1.34"),
			kubernetesSourceModuleConfig("1.33"),
			configMap,
		))
	})

	t.Run("invalid desiredVersion degrades to the next source", func(t *testing.T) {
		assert.Equal(t, "1.31", resolvedTarget(t,
			kubernetesSourceSecret("1.31", "1.34"),
			kubernetesSourceConfigMap("not-a-version"),
		))
	})

	t.Run("invalid ModuleConfig kubernetesVersion degrades to ClusterConfiguration", func(t *testing.T) {
		assert.Equal(t, "1.31", resolvedTarget(t,
			kubernetesSourceSecret("1.31", "1.34"),
			kubernetesSourceModuleConfig("not-a-version"),
		))
	})
}
