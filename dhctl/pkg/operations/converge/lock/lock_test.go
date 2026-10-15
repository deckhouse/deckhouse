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

package lock

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

type fakeKubeGetter struct {
	client *client.KubernetesClient
}

func (g fakeKubeGetter) KubeClientCtx(context.Context) (*client.KubernetesClient, error) {
	return g.client, nil
}

// The SSH client the lease is renewed through is replaced in the middle of a converge.
// Deleting the lease for that time lets an auto-converger take it, so the lease has to
// stay while renewal is paused, and ResetLock has to pick the still-live lease up.
func TestPauseRenewKeepsLeaseForResetLock(t *testing.T) {
	kubeClient := client.NewFakeKubernetesClient()
	runner := NewInLockRunner(t.Context(), fakeKubeGetter{client: kubeClient}, "converge-test", "")

	require.NoError(t, runner.setLock(t.Context(), false))

	runner.PauseRenew()

	lease := getLease(t, kubeClient)
	require.Equal(t, "converge-test", *lease.Spec.HolderIdentity)

	// Without the force ResetLock would still succeed, once the lease has expired: bound
	// it well below the lease duration.
	resetCtx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	require.NoError(t, runner.ResetLock(resetCtx))

	lease = getLease(t, kubeClient)
	require.Equal(t, "converge-test", *lease.Spec.HolderIdentity)

	runner.Stop()

	_, err := kubeClient.CoordinationV1().Leases(runner.lockConfig.Namespace).
		Get(t.Context(), runner.lockConfig.Name, metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err), "Stop releases the lease")
}

func getLease(t *testing.T, kubeClient *client.KubernetesClient) *coordinationv1.Lease {
	t.Helper()

	config := GetLockLeaseConfig(t.Context(), "", "")
	lease, err := kubeClient.CoordinationV1().Leases(config.Namespace).Get(t.Context(), config.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, lease.Spec.HolderIdentity)

	return lease
}
