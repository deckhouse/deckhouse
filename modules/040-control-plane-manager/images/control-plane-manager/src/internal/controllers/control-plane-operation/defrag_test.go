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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

func TestDefragEtcd_WaitPodDeadline(t *testing.T) {
	t.Parallel()

	newReconciler := func(objs ...client.Object) *Reconciler {
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objs...).
			WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{}).
			Build()
		return &Reconciler{
			client: c,
			log:    log.NewNop(),
			node:   NodeIdentity{Name: testNodeName},
			steps:  defaultSteps(),
		}
	}

	// newEtcdDefragState returns a defrag operation state whose start time is startedAgo in the past.
	newEtcdDefragState := func(startedAgo time.Duration) *controlplanev1alpha1.OperationState {
		op := &controlplanev1alpha1.ControlPlaneOperation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "etcd-defrag",
				Annotations: map[string]string{
					constants.OperationStartedAtAnnotationKey: time.Now().Add(-startedAgo).UTC().Format(time.RFC3339Nano),
				},
			},
			Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
				NodeName:  testNodeName,
				Component: controlplanev1alpha1.OperationComponentEtcd,
				Steps: []controlplanev1alpha1.StepName{
					controlplanev1alpha1.StepDefragEtcd,
					controlplanev1alpha1.StepWaitPodReady,
				},
			},
		}
		return controlplanev1alpha1.NewOperationState(op)
	}

	t.Run("pending while within deadline and etcd pod absent", func(t *testing.T) {
		t.Parallel()
		r := newReconciler() // no etcd pod in cluster
		state := newEtcdDefragState(10 * time.Second)

		res, err := r.defragEtcd(context.Background(), state, log.NewNop())
		require.NoError(t, err)
		require.Equal(t, OutcomePending, res.Outcome)
		require.Equal(t, requeueWaitPod, res.RequeueAfter)
	})

	t.Run("abandoned after deadline when etcd pod stays absent", func(t *testing.T) {
		t.Parallel()
		r := newReconciler() // no etcd pod in cluster
		state := newEtcdDefragState(etcdDefragWaitPodDeadline + time.Minute)

		res, err := r.defragEtcd(context.Background(), state, log.NewNop())
		require.NoError(t, err)
		require.Equal(t, OutcomeAbandoned, res.Outcome)
		require.NotEmpty(t, res.Message)
	})

	// WaitPodReady runs right after DefragEtcd in the same defrag CPO (see buildDefragCPO), and
	// must keep waiting indefinitely even well past etcdDefragWaitPodDeadline: abandoning it
	// would free the global etcd slot for another node's defrag while this node's etcd is still
	// down, risking a quorum loss.
	t.Run("WaitPodReady: keeps waiting past the DefragEtcd deadline", func(t *testing.T) {
		t.Parallel()
		r := newReconciler() // no etcd pod in cluster
		state := newEtcdDefragState(etcdDefragWaitPodDeadline + time.Minute)

		res, err := r.waitForPod(context.Background(), state, log.NewNop())
		require.NoError(t, err)
		require.Equal(t, OutcomePending, res.Outcome)
		require.Equal(t, requeueWaitPod, res.RequeueAfter)
	})
}
