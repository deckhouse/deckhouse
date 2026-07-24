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

package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeConfigSecret(data map[string]string) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-configuration", Namespace: "kube-system"},
		Data:       map[string][]byte{},
	}
	for k, v := range data {
		secret.Data[k] = []byte(v)
	}
	return secret
}

func TestGetConfiguration(t *testing.T) {
	t.Run("explicit CC version, no MC override", func(t *testing.T) {
		secret := makeConfigSecret(map[string]string{
			clusterConfigurationYAML: "kubernetesVersion: \"1.33\"\n",
		})

		cfg, err := GetConfiguration(secret)
		require.NoError(t, err)
		assert.Equal(t, "1.33", cfg.KubernetesVersion)
		assert.Equal(t, "1.33", cfg.DesiredVersion)
		assert.Equal(t, UpdateModeManual, cfg.UpdateMode)
	})

	t.Run("Automatic CC version resolved via secret default", func(t *testing.T) {
		secret := makeConfigSecret(map[string]string{
			clusterConfigurationYAML: "kubernetesVersion: Automatic\n",
			defaultKubernetesVersion: "1.34",
		})

		cfg, err := GetConfiguration(secret)
		require.NoError(t, err)
		assert.Equal(t, "Automatic", cfg.KubernetesVersion)
		assert.Equal(t, "1.34", cfg.DesiredVersion)
		assert.Equal(t, UpdateModeAutomatic, cfg.UpdateMode)
	})

	t.Run("ModuleConfig override takes precedence over an explicit CC version", func(t *testing.T) {
		secret := makeConfigSecret(map[string]string{
			clusterConfigurationYAML:      "kubernetesVersion: \"1.33\"\n",
			moduleConfigKubernetesVersion: "1.35",
		})

		cfg, err := GetConfiguration(secret)
		require.NoError(t, err)
		assert.Equal(t, "1.35", cfg.KubernetesVersion)
		assert.Equal(t, "1.35", cfg.DesiredVersion)
		assert.Equal(t, UpdateModeManual, cfg.UpdateMode)
	})

	t.Run("ModuleConfig override takes precedence over Automatic CC version", func(t *testing.T) {
		secret := makeConfigSecret(map[string]string{
			clusterConfigurationYAML:      "kubernetesVersion: Automatic\n",
			defaultKubernetesVersion:      "1.34",
			moduleConfigKubernetesVersion: "1.36",
		})

		cfg, err := GetConfiguration(secret)
		require.NoError(t, err)
		assert.Equal(t, "1.36", cfg.KubernetesVersion)
		assert.Equal(t, "1.36", cfg.DesiredVersion)
		assert.Equal(t, UpdateModeManual, cfg.UpdateMode)
	})

	t.Run("empty ModuleConfig override key falls back to CC", func(t *testing.T) {
		secret := makeConfigSecret(map[string]string{
			clusterConfigurationYAML:      "kubernetesVersion: \"1.33\"\n",
			moduleConfigKubernetesVersion: "",
		})

		cfg, err := GetConfiguration(secret)
		require.NoError(t, err)
		assert.Equal(t, "1.33", cfg.KubernetesVersion)
	})

	t.Run("missing cluster-configuration.yaml errors", func(t *testing.T) {
		secret := makeConfigSecret(map[string]string{})

		_, err := GetConfiguration(secret)
		require.Error(t, err)
	})
}
