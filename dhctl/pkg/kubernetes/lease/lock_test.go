// Copyright 2021 Flant JSC
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

package lease

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func TestRenewRetryCount(t *testing.T) {
	t.Run("Correct retries count", func(t *testing.T) {
		lockConf := &LeaseLockConfig{
			LeaseDurationSeconds: 300,
			RenewEverySeconds:    180,
			RetryWaitDuration:    3 * time.Second,
		}

		require.Equal(t, 40, lockConf.RenewRetries())
	})
}

func TestAcquireRetryCount(t *testing.T) {
	t.Run("Waits one lease lifetime and makes final attempt", func(t *testing.T) {
		lockConf := &LeaseLockConfig{
			LeaseDurationSeconds: 300,
			RetryWaitDuration:    3 * time.Second,
		}

		require.Equal(t, 101, lockConf.AcquireRetries())
	})
}

func TestTryRenewNilLease(t *testing.T) {
	t.Run("Nil lease should return error, not panic", func(t *testing.T) {
		lock := &LeaseLock{
			config: LeaseLockConfig{
				Identity: "test-id",
			},
		}

		require.NotPanics(t, func() {
			lease, err := lock.tryRenew(t.Context(), nil, true)

			require.Nil(t, lease)
			require.Error(t, err)
			require.Contains(t, err.Error(), "lease is nil")
		})
	})
}

func TestAcquireExistingLeaseNilHolder(t *testing.T) {
	t.Run("Lease without holder identity should return error", func(t *testing.T) {
		lock := &LeaseLock{
			config: LeaseLockConfig{
				Identity: "new-holder",
			},
		}

		lease, err := lock.acquireExistingLease(
			t.Context(),
			nil,
			&coordinationv1.Lease{},
			false,
		)

		require.Nil(t, lease)
		require.Error(t, err)
		require.Contains(t, err.Error(), "lease holder identity is nil")
	})
}

func TestAcquireExistingActiveForeignLease(t *testing.T) {
	t.Run("Active lease owned by another identity should not be taken over", func(t *testing.T) {
		holder := "old-holder"
		duration := int32(300)
		renewTime := metav1.NewMicroTime(time.Now())

		existingLease := &coordinationv1.Lease{
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       &holder,
				RenewTime:            &renewTime,
				LeaseDurationSeconds: &duration,
			},
		}

		lock := &LeaseLock{
			config: LeaseLockConfig{
				Identity: "new-holder",
			},
		}

		updatedLease, err := lock.acquireExistingLease(
			t.Context(),
			nil,
			existingLease,
			false,
		)

		require.Nil(t, updatedLease)
		require.Error(t, err)
		require.Contains(t, err.Error(), "old-holder")
		require.Equal(t, "old-holder", *existingLease.Spec.HolderIdentity)
	})
}

func TestAcquireExistingExpiredForeignLease(t *testing.T) {
	t.Run("Expired lease owned by another identity should be taken over", func(t *testing.T) {
		kubeClient := client.NewFakeKubernetesClient()

		oldHolder := "old-holder"
		duration := int32(300)
		acquireTime := metav1.NewMicroTime(time.Now().Add(-20 * time.Minute))
		renewTime := metav1.NewMicroTime(time.Now().Add(-10 * time.Minute))

		existingLease := &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-lock",
				Namespace: "default",
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       &oldHolder,
				AcquireTime:          &acquireTime,
				RenewTime:            &renewTime,
				LeaseDurationSeconds: &duration,
			},
		}

		_, err := kubeClient.
			CoordinationV1().
			Leases("default").
			Create(t.Context(), existingLease, metav1.CreateOptions{})
		require.NoError(t, err)

		lock := &LeaseLock{
			config: LeaseLockConfig{
				Name:                 "test-lock",
				Namespace:            "default",
				Identity:             "new-holder",
				LeaseDurationSeconds: 300,
			},
		}

		updatedLease, err := lock.acquireExistingLease(
			t.Context(),
			kubeClient,
			existingLease,
			false,
		)

		require.NoError(t, err)
		require.NotNil(t, updatedLease)
		require.NotNil(t, updatedLease.Spec.HolderIdentity)
		require.Equal(t, "new-holder", *updatedLease.Spec.HolderIdentity)

		storedLease, err := kubeClient.
			CoordinationV1().
			Leases("default").
			Get(t.Context(), "test-lock", metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, storedLease.Spec.HolderIdentity)
		require.Equal(t, "new-holder", *storedLease.Spec.HolderIdentity)
	})
}

type fakeKubeGetter struct {
	client *client.KubernetesClient
}

func (g fakeKubeGetter) KubeClientCtx(context.Context) (*client.KubernetesClient, error) {
	return g.client, nil
}

// Converge re-takes its own lease after pausing its renewal for an SSH client switch.
// The force it does that with must renew a live lease of its own identity and still
// refuse a live lease somebody else holds.
func TestAcquireExistingActiveLeaseWithForce(t *testing.T) {
	newLiveLease := func(holder string) *coordinationv1.Lease {
		duration := int32(300)
		renewTime := metav1.NewMicroTime(time.Now())

		return &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{Name: "test-lock", Namespace: "default"},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       &holder,
				RenewTime:            &renewTime,
				LeaseDurationSeconds: &duration,
			},
		}
	}

	newLock := func(kubeClient *client.KubernetesClient) *LeaseLock {
		return &LeaseLock{
			getter: fakeKubeGetter{client: kubeClient},
			config: LeaseLockConfig{
				Name:                 "test-lock",
				Namespace:            "default",
				Identity:             "converge",
				LeaseDurationSeconds: 300,
				RetryWaitDuration:    time.Millisecond,
			},
		}
	}

	t.Run("own live lease is renewed", func(t *testing.T) {
		kubeClient := client.NewFakeKubernetesClient()
		existingLease, err := kubeClient.CoordinationV1().Leases("default").
			Create(t.Context(), newLiveLease("converge"), metav1.CreateOptions{})
		require.NoError(t, err)

		updatedLease, err := newLock(kubeClient).acquireExistingLease(t.Context(), kubeClient, existingLease, true)

		require.NoError(t, err)
		require.Equal(t, "converge", *updatedLease.Spec.HolderIdentity)
	})

	t.Run("foreign live lease is refused", func(t *testing.T) {
		kubeClient := client.NewFakeKubernetesClient()
		existingLease := newLiveLease("terraform-auto-converger")

		updatedLease, err := newLock(kubeClient).acquireExistingLease(t.Context(), kubeClient, existingLease, true)

		require.ErrorContains(t, err, "terraform-auto-converger")
		require.Nil(t, updatedLease)
		require.Equal(t, "terraform-auto-converger", *existingLease.Spec.HolderIdentity)
	})
}

// pausingGetter stops the renewal the moment a renew reaches for the kube client, the
// way converge pauses it to replace the SSH client that kube client runs over.
type pausingGetter struct {
	lock   *LeaseLock
	client *client.KubernetesClient
}

func (g *pausingGetter) KubeClientCtx(context.Context) (*client.KubernetesClient, error) {
	if g.lock != nil {
		g.lock.StopAutoRenew()
		g.lock = nil
	}

	return g.client, nil
}

// A renew in flight when the renewal is paused fails on the client being replaced. That
// failure must not reach OnRenewError: in converge it sends SIGINT and kills a run whose
// lease was already taken again.
func TestRenewFailingAfterStopDoesNotReportError(t *testing.T) {
	run := func(t *testing.T, pause bool) bool {
		identity := "converge"
		renewErrors := make(chan error, 1)
		getter := &pausingGetter{client: client.NewFakeKubernetesClient()}

		lock := NewLeaseLock(getter, LeaseLockConfig{
			Name:                 "test-lock",
			Namespace:            "default",
			Identity:             identity,
			LeaseDurationSeconds: 2,
			RenewEverySeconds:    1,
			RetryWaitDuration:    10 * time.Millisecond,
			OnRenewError:         func(err error) { renewErrors <- err },
		})
		// the fake cluster has no such lease, so every renew fails
		lock.lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{Name: "test-lock", Namespace: "default"},
			Spec:       coordinationv1.LeaseSpec{HolderIdentity: &identity},
		}
		if pause {
			getter.lock = lock
		}

		done := make(chan struct{})
		go func() {
			lock.startAutoRenew(t.Context())
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("autorenew did not stop")
		}

		select {
		case <-renewErrors:
			return true
		default:
			return false
		}
	}

	t.Run("paused while renewing", func(t *testing.T) {
		require.False(t, run(t, true))
	})

	t.Run("not paused", func(t *testing.T) {
		require.True(t, run(t, false))
	})
}
