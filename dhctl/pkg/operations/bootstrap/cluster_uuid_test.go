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

package bootstrap

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	utilcache "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func clusterStampedWith(t *testing.T, clusterUUID string) *client.KubernetesClient {
	t.Helper()

	kubeCl := client.NewFakeKubernetesClient()

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifests.ClusterUUIDCm,
			Namespace: manifests.ClusterUUIDCmNamespace,
		},
		Data: map[string]string{manifests.ClusterUUIDCmKey: clusterUUID},
	}

	_, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Create(t.Context(), cm, metav1.CreateOptions{})
	require.NoError(t, err)

	return kubeCl
}

func TestClusterUUIDCheck(t *testing.T) {
	t.Run("accepts a cluster it did not bootstrap", func(t *testing.T) {
		kubeCl := clusterStampedWith(t, uuid.New().String())

		err := CheckPreventBreakAnotherBootstrappedCluster(t.Context(), kubeCl, &config.DeckhouseInstaller{})

		require.NoError(t, err, "an installation without a UUID of its own has nothing to compare and must not abort")
	})

	t.Run("rejects a cluster stamped by another installation", func(t *testing.T) {
		kubeCl := clusterStampedWith(t, uuid.New().String())

		err := CheckPreventBreakAnotherBootstrappedCluster(t.Context(), kubeCl, &config.DeckhouseInstaller{UUID: uuid.New().String()})

		require.Error(t, err)
	})

	t.Run("accepts a cluster stamped by this installation", func(t *testing.T) {
		clusterUUID := uuid.New().String()
		kubeCl := clusterStampedWith(t, clusterUUID)

		err := CheckPreventBreakAnotherBootstrappedCluster(t.Context(), kubeCl, &config.DeckhouseInstaller{UUID: clusterUUID})

		require.NoError(t, err)
	})
}

// TestGenerateClusterUUID pins where the UUID dhctl owns comes from, and for which clusters it
// owns one at all. The state cache is dhctl's record of a cluster it created; minting an entry
// there for a cluster it did not create publishes that value as the cluster's - Preparation logs
// it, the bootstrap span carries it and Commander reads it back out of the cache - while it
// belongs to whichever cluster shares the directory.
func TestGenerateClusterUUID(t *testing.T) {
	t.Run("a cluster dhctl creates keeps one UUID across runs", func(t *testing.T) {
		stateCache := utilcache.NewTestCache()
		metaConfig := &config.MetaConfig{ClusterConfig: map[string]json.RawMessage{"clusterType": json.RawMessage(`"Static"`)}}

		first, err := generateClusterUUID(t.Context(), stateCache, metaConfig)
		require.NoError(t, err)

		_, err = uuid.Parse(first)
		require.NoError(t, err, "a generated cluster UUID must be a UUID")

		second, err := generateClusterUUID(t.Context(), stateCache, metaConfig)
		require.NoError(t, err)
		require.Equal(t, first, second, "a resumed bootstrap must go on using the UUID it already stamped")
	})

	t.Run("a cluster dhctl did not create gets none", func(t *testing.T) {
		stateCache := utilcache.NewTestCache()

		clusterUUID, err := generateClusterUUID(t.Context(), stateCache, &config.MetaConfig{})
		require.NoError(t, err)
		require.Empty(t, clusterUUID, "kube-system/d8-cluster-uuid is the only authority on the identity of such a cluster")

		cached, err := stateCache.InCache(t.Context(), "uuid")
		require.NoError(t, err)
		require.False(t, cached, "a UUID left in the cache is handed to the next cluster that opens the same directory")
	})
}

// TestResolveClusterUUID pins which side owns the cluster identity. dhctl may hold a UUID of its
// own only for a cluster it created; for one it did not, a UUID it is nevertheless carrying was
// loaded out of a state cache and belongs to whichever cluster wrote it there. Letting it win
// against an already-stamped cluster would make the check above refuse a legitimate re-run, and
// letting it win against an unstamped one would stamp a stranger's UUID into it.
func TestResolveClusterUUID(t *testing.T) {
	clusterConfiguration := []byte("apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\n")

	t.Run("a cluster dhctl did not create keeps its own UUID", func(t *testing.T) {
		clusterUUID := uuid.New().String()
		cfg := &config.DeckhouseInstaller{UUID: uuid.New().String()}

		require.NoError(t, resolveClusterUUID(t.Context(), clusterStampedWith(t, clusterUUID), cfg))
		require.Equal(t, clusterUUID, cfg.UUID, "the UUID out of the shared state cache must not win")
	})

	t.Run("a cluster dhctl created keeps the UUID dhctl generated for it", func(t *testing.T) {
		ownUUID := uuid.New().String()
		cfg := &config.DeckhouseInstaller{UUID: ownUUID, ClusterConfig: clusterConfiguration}

		require.NoError(t, resolveClusterUUID(t.Context(), clusterStampedWith(t, uuid.New().String()), cfg))
		require.Equal(t, ownUUID, cfg.UUID, "overwriting it here would disarm the check for bootstrapping over a live cluster")
	})

	t.Run("an installation holding no UUID adopts the one already in the cluster", func(t *testing.T) {
		clusterUUID := uuid.New().String()
		cfg := &config.DeckhouseInstaller{ClusterConfig: clusterConfiguration}

		require.NoError(t, resolveClusterUUID(t.Context(), clusterStampedWith(t, clusterUUID), cfg))
		require.Equal(t, clusterUUID, cfg.UUID, "install-deckhouse run on its own loads no UUID and must not stamp a second one")
	})

	t.Run("an unstamped cluster dhctl did not create gets a fresh UUID", func(t *testing.T) {
		borrowedUUID := uuid.New().String()
		cfg := &config.DeckhouseInstaller{UUID: borrowedUUID}

		require.NoError(t, resolveClusterUUID(t.Context(), client.NewFakeKubernetesClient(), cfg))

		require.NotEqual(t, borrowedUUID, cfg.UUID,
			"getClusterUUIDTasks writes this value into kube-system/d8-cluster-uuid, and without a ClusterConfiguration it came out of a state cache another cluster may have written",
		)

		_, err := uuid.Parse(cfg.UUID)
		require.NoError(t, err, "a generated cluster UUID must be a UUID")
	})

	t.Run("an installation with no UUID at all generates one", func(t *testing.T) {
		cfg := &config.DeckhouseInstaller{}

		require.NoError(t, resolveClusterUUID(t.Context(), client.NewFakeKubernetesClient(), cfg))

		_, err := uuid.Parse(cfg.UUID)
		require.NoError(t, err, "a generated cluster UUID must be a UUID")
	})
}
