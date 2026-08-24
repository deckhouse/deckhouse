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
	"context"
	"testing"
	"time"

	klient "github.com/flant/kube-client/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// TestResolveClusterUUIDReadsThroughAStartingAPIServer pins the two halves of the read that decides
// the cluster's identity, and that both callers make before anything else: a refused connection is
// waited out, an absent config map is not. Answering a transport failure with a fresh UUID would
// hand a second identity to a cluster that already has one, and CheckPreventBreakAnotherBootstrapped
// Cluster - the very next call - would then refuse the bootstrap it was meant to protect.
func TestResolveClusterUUIDReadsThroughAStartingAPIServer(t *testing.T) {
	// init_test.go collapses every retry loop in this binary to a single attempt; the loop is what
	// is under test here, so it is restored for this test alone. Not parallel, and neither is
	// anything it shares the window with: the parallel tests of this package resume only after the
	// sequential ones are done.
	retry.InTestEnvironment = false

	t.Cleanup(func() { retry.InTestEnvironment = true })

	t.Run("a read the API server is not up for yet is waited out", func(t *testing.T) {
		clusterUUID := uuid.New().String()
		kubeCl := clusterStampedWith(t, clusterUUID)

		attempts := 0

		reactorOn(t, kubeCl, func() (bool, runtime.Object, error) {
			attempts++
			if attempts == 1 {
				return true, nil, apierrors.NewServiceUnavailable("connection refused")
			}

			return false, nil, nil
		})

		cfg := &config.DeckhouseInstaller{}

		require.NoError(t, resolveClusterUUID(t.Context(), kubeCl, cfg))
		require.Equal(t, clusterUUID, cfg.UUID, "the cluster is the authority on its identity, and it answered on the second attempt")
		require.Equal(t, 2, attempts)
	})

	t.Run("an unstamped cluster is not waited out", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()

		attempts := 0

		reactorOn(t, kubeCl, func() (bool, runtime.Object, error) {
			attempts++

			return false, nil, nil
		})

		// A retried NotFound would spend 45 seconds before minting anything; the deadline turns
		// that into a failure here instead of a slow test.
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		cfg := &config.DeckhouseInstaller{}

		require.NoError(t, resolveClusterUUID(ctx, kubeCl, cfg))
		require.Equal(t, 1, attempts, "an absent config map is the answer, not a failure to retry")

		_, err := uuid.Parse(cfg.UUID)
		require.NoError(t, err, "a generated cluster UUID must be a UUID")
	})

	t.Run("a read that never succeeds mints nothing", func(t *testing.T) {
		kubeCl := client.NewFakeKubernetesClient()

		reactorOn(t, kubeCl, func() (bool, runtime.Object, error) {
			return true, nil, apierrors.NewServiceUnavailable("connection refused")
		})

		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()

		carried := uuid.New().String()
		cfg := &config.DeckhouseInstaller{UUID: carried}

		require.Error(t, resolveClusterUUID(ctx, kubeCl, cfg))
		require.Equal(t, carried, cfg.UUID, "a cluster that could not be read is not a cluster without a UUID")
	})
}

func reactorOn(t *testing.T, kubeCl *client.KubernetesClient, react func() (bool, runtime.Object, error)) {
	t.Helper()

	clientset, ok := kubeCl.KubeClient.(*klient.Client).Interface.(*k8sfake.Clientset)
	require.True(t, ok, "fake kube client is not backed by a fake clientset")

	clientset.PrependReactor("get", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
		return react()
	})
}
