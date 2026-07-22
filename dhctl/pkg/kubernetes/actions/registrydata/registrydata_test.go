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
	"strings"
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

func createDeckhouseRegistrySecret(t *testing.T, kubeCl *client.KubernetesClient, imagesRegistry string) {
	t.Helper()
	host, _, _ := strings.Cut(imagesRegistry, "/")
	_, err := kubeCl.CoreV1().Secrets(d8RppSecretNamespace).Create(t.Context(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: d8RppSecretName, Namespace: d8RppSecretNamespace},
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"` + host + `":{"auth":"dXNlcjpwYXNz"}}}`),
			"imagesRegistry":    []byte(imagesRegistry),
			"scheme":            []byte("HTTPS"),
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func TestGetRegistryDataPreferUpstream(t *testing.T) {
	const (
		upstream = "dev-registry.deckhouse.io/sys/deckhouse-oss"
		mirror   = "registry.d8-system.svc:5001/system/deckhouse"
	)

	t.Run("out of cluster prefers the upstream registry with a dockercfg", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createRegistryConfigSecret(t, kubeCl, map[string][]byte{
			"mode": []byte("Direct"), "imagesRepo": []byte(upstream),
			"scheme": []byte("HTTPS"), "username": []byte("u"), "password": []byte("p"),
		})
		createDeckhouseRegistrySecret(t, kubeCl, mirror)

		conf, b64dc, err := GetRegistryDataPreferUpstream(t.Context(), kubeCl, false)
		require.NoError(t, err)
		require.Equal(t, upstream, conf.GetRegistry())
		require.NotEmpty(t, b64dc, "upstream path must build a registryDockerCfg")
	})

	t.Run("in cluster uses the mirror", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createRegistryConfigSecret(t, kubeCl, map[string][]byte{
			"mode": []byte("Direct"), "imagesRepo": []byte(upstream), "scheme": []byte("HTTPS"),
		})
		createDeckhouseRegistrySecret(t, kubeCl, mirror)

		conf, _, err := GetRegistryDataPreferUpstream(t.Context(), kubeCl, true)
		require.NoError(t, err)
		require.Equal(t, mirror, conf.GetRegistry())
	})

	t.Run("out of cluster falls back to the mirror when no upstream is configured", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createDeckhouseRegistrySecret(t, kubeCl, mirror)

		conf, _, err := GetRegistryDataPreferUpstream(t.Context(), kubeCl, false)
		require.NoError(t, err)
		require.Equal(t, mirror, conf.GetRegistry())
	})
}
