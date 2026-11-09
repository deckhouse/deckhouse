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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func createRegistryStorage(t *testing.T, kubeCl *client.KubernetesClient, addresses ...string) {
	t.Helper()

	replicas := make([]interface{}, 0, len(addresses))
	for _, address := range addresses {
		replicas = append(replicas, map[string]interface{}{"node": "master", "address": address})
	}
	_, err := kubeCl.Dynamic().Resource(registryStorageGVR).Create(t.Context(), &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1alpha1",
			"kind":       "RegistryStorage",
			"metadata":   map[string]interface{}{"name": registryStorageName},
			"status":     map[string]interface{}{"replicas": replicas},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

// TestTheTunnelGoesWhereTheReplicaReportedServing: during the update to the store's current ports a
// replica serves on the previous port until the module replaces it, and a dhctl of the new version
// has to reach it there.
func TestTheTunnelGoesWhereTheReplicaReportedServing(t *testing.T) {
	t.Run("a replica on the previous pod spec", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createRegistryStorage(t, kubeCl, "10.0.0.2:4281", "10.0.0.1:5001")

		require.Equal(t, registry_const.LegacyStoragePort, servingPort(t.Context(), kubeCl, "10.0.0.1"))
		require.Equal(t, registry_const.StoragePort, servingPort(t.Context(), kubeCl, "10.0.0.2"))
	})

	t.Run("IPv6 in either spelling", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		createRegistryStorage(t, kubeCl, "[fd00:0::1]:5001")

		require.Equal(t, registry_const.LegacyStoragePort, servingPort(t.Context(), kubeCl, "fd00::1"))
		require.Equal(t, registry_const.LegacyStoragePort, servingPort(t.Context(), kubeCl, "[fd00::1]"))
	})

	t.Run("no report, an unreadable one, or none for this host: the current port", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()
		require.Equal(t, registry_const.StoragePort, servingPort(t.Context(), kubeCl, "10.0.0.1"))

		createRegistryStorage(t, kubeCl, "10.0.0.1", "10.0.0.2:0", "10.0.0.3:5001")
		require.Equal(t, registry_const.StoragePort, servingPort(t.Context(), kubeCl, "10.0.0.1"))
		require.Equal(t, registry_const.StoragePort, servingPort(t.Context(), kubeCl, "10.0.0.2"))
		require.Equal(t, registry_const.StoragePort, servingPort(t.Context(), kubeCl, "10.0.0.30"))
	})
}
