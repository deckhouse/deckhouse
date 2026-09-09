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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
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

		// The dockercfg must key on the same host that RegistryConfigFromDockerConfig
		// looks up, otherwise lazy image pulls out of the cluster find no credentials.
		dockerCfg, err := image.DecodeDockerConfig(b64dc)
		require.NoError(t, err)
		registryConf, err := image.RegistryConfigFromDockerConfig(dockerCfg, "HTTPS", upstream)
		require.NoError(t, err)
		require.Equal(t, "u", registryConf.GetUsername())
		require.Equal(t, "p", registryConf.GetPassword())
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

	t.Run("out of cluster surfaces an invalid upstream instead of falling back", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createRegistryConfigSecret(t, kubeCl, map[string][]byte{
			"mode": []byte("Direct"), "imagesRepo": []byte(upstream), "scheme": []byte("invalid"),
		})
		createDeckhouseRegistrySecret(t, kubeCl, mirror)

		conf, b64dc, err := GetRegistryDataPreferUpstream(t.Context(), kubeCl, false)
		require.ErrorContains(t, err, "scheme must be HTTP or HTTPS")
		require.Nil(t, conf)
		require.Empty(t, b64dc)
	})
}

// createRegistryConfigResource plants the object the controller-based implementation resolves its
// configuration into: cluster-scoped, singleton, and the only source of the upstream that is kept
// current on such a cluster.
func createRegistryConfigResource(t *testing.T, kubeCl *client.KubernetesClient, host, path string, auth map[string]interface{}) {
	t.Helper()

	upstream := map[string]interface{}{"scheme": "HTTPS", "host": host, "path": path}
	if auth != nil {
		upstream["auth"] = auth
	}

	object := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "RegistryConfig",
		"metadata":   map[string]interface{}{"name": registryConfigResourceName},
		"spec": map[string]interface{}{
			"mode":    "Managed",
			"primary": map[string]interface{}{"upstream": upstream},
		},
	}}

	gvr := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "registryconfigs"}
	_, err := kubeCl.Dynamic().Resource(gvr).Create(t.Context(), object, metav1.CreateOptions{})
	require.NoError(t, err)
}

// TestTheConfigResourceWinsOverTheLegacySecret is about a stale answer being worse than no answer.
//
// `registry-config` is rendered from the PREVIOUS implementation's settings in `mc/deckhouse`, and on a
// migrated cluster nobody writes those any more — so it keeps describing whatever registry the cluster
// was migrated from. Measured on such a cluster: the secret named `111.88.253.76.sslip.io/dh-dev-registry/...`
// with that mirror's robot account while the cluster's upstream had been moved to
// `dev-registry.deckhouse.io/sys/deckhouse-oss`, and this package preferred the secret. Absence falls
// back and works; staleness dials the wrong registry with the wrong account.
func TestTheConfigResourceWinsOverTheLegacySecret(t *testing.T) {
	const (
		current = "dev-registry.deckhouse.io/sys/deckhouse-oss"
		stale   = "111.88.253.76.sslip.io/dh-dev-registry/sys/deckhouse-oss"
		mirror  = "registry.d8-system.svc:5001/system/deckhouse"
	)

	kubeCl := client.NewFakeKubernetesClient()
	createRegistryConfigResource(t, kubeCl, "dev-registry.deckhouse.io", "/sys/deckhouse-oss",
		map[string]interface{}{"username": "license-token", "password": "current-key"})
	createRegistryConfigSecret(t, kubeCl, map[string][]byte{
		"mode": []byte("Unmanaged"), "imagesRepo": []byte(stale), "scheme": []byte("HTTPS"),
		"username": []byte("robot$old"), "password": []byte("old-key"),
	})
	createDeckhouseRegistrySecret(t, kubeCl, mirror)

	conf, b64dc, err := GetRegistryDataPreferUpstream(t.Context(), kubeCl, false)
	require.NoError(t, err)
	require.Equal(t, current, conf.GetRegistry(), "the resource the cluster keeps current has to win")

	// And the credentials travel with it, keyed on the host that will be asked for.
	dockerCfg, err := image.DecodeDockerConfig(b64dc)
	require.NoError(t, err)
	registryConf, err := image.RegistryConfigFromDockerConfig(dockerCfg, "HTTPS", current)
	require.NoError(t, err)
	require.Equal(t, "license-token", registryConf.GetUsername())
	require.Equal(t, "current-key", registryConf.GetPassword())
}

// TestAnAirGappedResourceFallsThrough: a resource with no upstream is not a gap in the configuration,
// it is the definition of an air-gapped cluster — and the caller must then reach the store through the
// SSH connection rather than be handed an empty registry.
func TestAnAirGappedResourceFallsThrough(t *testing.T) {
	const mirror = "registry.d8-system.svc:5001/system/deckhouse"

	kubeCl := client.NewFakeKubernetesClient()
	gvr := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "registryconfigs"}
	_, err := kubeCl.Dynamic().Resource(gvr).Create(t.Context(), &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "RegistryConfig",
		"metadata":   map[string]interface{}{"name": registryConfigResourceName},
		"spec":       map[string]interface{}{"mode": "Managed", "storage": map[string]interface{}{"cache": true}},
	}}, metav1.CreateOptions{})
	require.NoError(t, err)
	createDeckhouseRegistrySecret(t, kubeCl, mirror)

	conf, _, err := GetRegistryDataPreferUpstream(t.Context(), kubeCl, false)
	require.NoError(t, err)
	require.Equal(t, mirror, conf.GetRegistry(), "with no upstream the store is the only thing left")
}
