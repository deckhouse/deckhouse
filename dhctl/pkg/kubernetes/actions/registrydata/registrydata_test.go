// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registrydata

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func createRegistryConfigSecret(t *testing.T, kubeCl *client.KubernetesClient, data map[string][]byte) {
	t.Helper()
	_, err := kubeCl.CoreV1().Secrets(d8RppSecretNamespace).Create(t.Context(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: registryConfigSecret, Namespace: d8RppSecretNamespace},
		Data:       data,
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func TestGetUpstreamRegistryData(t *testing.T) {
	t.Run("absent secret is not found, no error", func(t *testing.T) {
		_, found, err := GetUpstreamRegistryData(t.Context(), client.NewFakeKubernetesClient())
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("Direct mode returns the upstream imagesRepo", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createRegistryConfigSecret(t, kubeCl, map[string][]byte{
			"mode":       []byte("Direct"),
			"imagesRepo": []byte("dev-registry.deckhouse.io/sys/deckhouse-oss"),
			"scheme":     []byte("HTTPS"),
			"username":   []byte("u"),
			"password":   []byte("p"),
		})

		conf, found, err := GetUpstreamRegistryData(t.Context(), kubeCl)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, "dev-registry.deckhouse.io/sys/deckhouse-oss", conf.GetRegistry())
	})

	t.Run("empty imagesRepo (Local mode) is not found", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createRegistryConfigSecret(t, kubeCl, map[string][]byte{"mode": []byte("Local")})

		_, found, err := GetUpstreamRegistryData(t.Context(), kubeCl)
		require.NoError(t, err)
		require.False(t, found)
	})
}
