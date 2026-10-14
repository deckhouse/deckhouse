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

package controlplaneoperation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/deckhouse/deckhouse/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

// kcmLease is a Lease last renewed renewedAgo before now.
func kcmLease(holder string, renewedAgo time.Duration) *coordinationv1.Lease {
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Name: kcmLeaseKey.Name, Namespace: kcmLeaseKey.Namespace, UID: "lease-uid"},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: ptr.To(holder),
			RenewTime:      &metav1.MicroTime{Time: time.Now().Add(-renewedAgo)},
		},
	}
}

func oldKCMPod() *corev1.Pod {
	return kcmMirrorPod(map[string]string{constants.ConfigChecksumAnnotationKey: "old"}, true)
}

const staleRenew = time.Minute

func kcmMirrorPod(annotations map[string]string, running bool) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "kube-controller-manager-" + testNodeName,
			Namespace:   constants.KubeSystemNamespace,
			Annotations: annotations,
		},
	}
	if running {
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name:  "kube-controller-manager",
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()}},
		}}
	}
	return pod
}

func newLeaseTestReconciler(funcs *interceptor.Funcs, objs ...client.Object) *Reconciler {
	b := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{})
	if funcs != nil {
		b = b.WithInterceptorFuncs(*funcs)
	}
	c := b.Build()
	return &Reconciler{client: c, apiReader: c, log: log.NewNop(), node: NodeIdentity{Name: testNodeName}}
}

func leaseExists(t *testing.T, r *Reconciler) bool {
	t.Helper()
	err := r.client.Get(context.Background(), kcmLeaseKey, &coordinationv1.Lease{})
	if apierrors.IsNotFound(err) {
		return false
	}
	require.NoError(t, err)
	return true
}

func TestKCMLeaderLease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	logger := log.NewNop()
	kcm := controlplanev1alpha1.OperationComponentKubeControllerManager
	steps := []controlplanev1alpha1.StepName{controlplanev1alpha1.StepSyncManifests, controlplanev1alpha1.StepWaitPodReady}

	t.Run("leader on this node: Lease deleted once the new checksum is on a running pod", func(t *testing.T) {
		t.Parallel()
		op := testOperation(kcm, steps, true)
		pod := oldKCMPod()
		r := newLeaseTestReconciler(nil, op, kcmLease(testNodeName+"_a1b2", staleRenew), pod)

		r.recordKCMLeader(ctx, op, logger)
		stored := &controlplanev1alpha1.ControlPlaneOperation{}
		require.NoError(t, r.client.Get(ctx, client.ObjectKeyFromObject(op), stored))
		require.Contains(t, stored.Annotations[constants.KCMLeaderLeaseAnnotationKey], testNodeName+"_a1b2")

		// The old instance is still there: nothing is released.
		state := controlplanev1alpha1.NewOperationState(stored)
		_, err := r.waitForPod(ctx, state, logger)
		require.NoError(t, err)
		require.True(t, leaseExists(t, r))

		pod.Annotations = desiredChecksumAnnotations(checksumAnnotationsFromSpec(op.Spec))
		require.NoError(t, r.client.Update(ctx, pod))
		_, err = r.waitForPod(ctx, state, logger)
		require.NoError(t, err)
		require.False(t, leaseExists(t, r))
		require.NoError(t, r.client.Get(ctx, client.ObjectKeyFromObject(op), stored))
		require.Empty(t, stored.Annotations[constants.KCMLeaderLeaseAnnotationKey])
	})

	t.Run("holder still renews: Lease kept and checked again", func(t *testing.T) {
		t.Parallel()
		op := testOperation(kcm, steps, true)
		r := newLeaseTestReconciler(nil, op, kcmLease(testNodeName+"_a1b2", time.Second), oldKCMPod())
		r.recordKCMLeader(ctx, op, logger)
		require.NotEmpty(t, op.Annotations[constants.KCMLeaderLeaseAnnotationKey])

		require.True(t, r.releaseKCMLeader(ctx, op, time.Now(), logger))
		require.True(t, leaseExists(t, r))
		require.NotEmpty(t, op.Annotations[constants.KCMLeaderLeaseAnnotationKey])
	})

	t.Run("pod already runs the desired checksums: nothing recorded", func(t *testing.T) {
		t.Parallel()
		op := testOperation(kcm, steps, true)
		pod := kcmMirrorPod(desiredChecksumAnnotations(checksumAnnotationsFromSpec(op.Spec)), true)
		r := newLeaseTestReconciler(nil, op, kcmLease(testNodeName+"_a1b2", staleRenew), pod)
		r.recordKCMLeader(ctx, op, logger)
		require.Empty(t, op.Annotations[constants.KCMLeaderLeaseAnnotationKey])
	})

	t.Run("holder changed meanwhile: Lease kept", func(t *testing.T) {
		t.Parallel()
		op := testOperation(kcm, steps, true)
		lease := kcmLease(testNodeName+"_a1b2", staleRenew)
		r := newLeaseTestReconciler(nil, op, lease, oldKCMPod())
		r.recordKCMLeader(ctx, op, logger)
		require.NotEmpty(t, op.Annotations[constants.KCMLeaderLeaseAnnotationKey])

		require.NoError(t, r.client.Get(ctx, kcmLeaseKey, lease))
		lease.Spec.HolderIdentity = ptr.To("master-2_c3d4")
		require.NoError(t, r.client.Update(ctx, lease))

		r.releaseKCMLeader(ctx, op, time.Now(), logger)
		require.True(t, leaseExists(t, r))
	})

	t.Run("precondition conflict: Lease kept", func(t *testing.T) {
		t.Parallel()
		op := testOperation(kcm, steps, true)
		renewing := false
		// Every read of the Lease is followed by a renewal, so the delete precondition is stale.
		funcs := &interceptor.Funcs{Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if err := c.Get(ctx, key, obj, opts...); err != nil {
				return err
			}
			if lease, ok := obj.(*coordinationv1.Lease); ok && renewing {
				renewed := lease.DeepCopy()
				renewed.Spec.AcquireTime = &metav1.MicroTime{Time: time.Now()}
				return c.Update(ctx, renewed)
			}
			return nil
		}}
		r := newLeaseTestReconciler(funcs, op, kcmLease(testNodeName+"_a1b2", staleRenew), oldKCMPod())
		r.recordKCMLeader(ctx, op, logger)
		require.NotEmpty(t, op.Annotations[constants.KCMLeaderLeaseAnnotationKey])

		renewing = true
		r.releaseKCMLeader(ctx, op, time.Now(), logger)
		renewing = false
		require.True(t, leaseExists(t, r))
	})

	t.Run("leader on another node: nothing recorded", func(t *testing.T) {
		t.Parallel()
		op := testOperation(kcm, steps, true)
		r := newLeaseTestReconciler(nil, op, kcmLease("master-2_c3d4", staleRenew), oldKCMPod())
		r.recordKCMLeader(ctx, op, logger)
		require.Empty(t, op.Annotations[constants.KCMLeaderLeaseAnnotationKey])
	})

	t.Run("other components: no-op", func(t *testing.T) {
		t.Parallel()
		op := testOperation(controlplanev1alpha1.OperationComponentKubeScheduler, steps, true)
		r := newLeaseTestReconciler(nil, op, kcmLease(testNodeName+"_a1b2", staleRenew), oldKCMPod())
		r.recordKCMLeader(ctx, op, logger)
		require.Empty(t, op.Annotations[constants.KCMLeaderLeaseAnnotationKey])

		op.Annotations = map[string]string{constants.KCMLeaderLeaseAnnotationKey: `{"holderIdentity":"` + testNodeName + `_a1b2","uid":"lease-uid"}`}
		r.releaseKCMLeader(ctx, op, time.Now(), logger)
		require.True(t, leaseExists(t, r))
	})
}

func TestDecideKCMLease(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	recorded := kcmLeaderLease{HolderIdentity: testNodeName + "_a1b2", UID: "lease-uid"}
	// lease is a Lease last renewed renewedAgo before now.
	lease := func(holder string, uid types.UID, renewedAgo time.Duration) *coordinationv1.Lease {
		return &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{UID: uid},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity: ptr.To(holder),
				RenewTime:      &metav1.MicroTime{Time: now.Add(-renewedAgo)},
			},
		}
	}
	noRenewTime := lease(recorded.HolderIdentity, recorded.UID, 0)
	noRenewTime.Spec.RenewTime = nil

	for name, tc := range map[string]struct {
		lease *coordinationv1.Lease
		want  kcmLeaseAction
	}{
		"stopped renewing":    {lease: lease(recorded.HolderIdentity, recorded.UID, kcmLeaseStaleAfter), want: kcmLeaseRelease},
		"renewed recently":    {lease: lease(recorded.HolderIdentity, recorded.UID, kcmLeaseStaleAfter-time.Nanosecond), want: kcmLeaseRecheck},
		"no renewTime":        {lease: noRenewTime, want: kcmLeaseKeep},
		"another holder":      {lease: lease("master-2_c3d4", recorded.UID, kcmLeaseStaleAfter), want: kcmLeaseForget},
		"the Lease recreated": {lease: lease(recorded.HolderIdentity, "other-uid", kcmLeaseStaleAfter), want: kcmLeaseForget},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, decideKCMLease(recorded, tc.lease, now))
		})
	}
}
