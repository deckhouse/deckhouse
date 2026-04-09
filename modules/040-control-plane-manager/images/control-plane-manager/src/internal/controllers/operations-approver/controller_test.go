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

package operationsapprover

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

var testScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(controlplanev1alpha1.AddToScheme(testScheme))
}

func TestReconciler_Reconcile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("no control plane nodes returns without error", func(t *testing.T) {
		t.Parallel()
		cl := fake.NewClientBuilder().WithScheme(testScheme).Build()
		r := &reconciler{client: cl}
		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)
	})

	t.Run("no operations returns without error", func(t *testing.T) {
		t.Parallel()
		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(testControlPlaneNode("n1")).
			Build()
		r := &reconciler{client: cl}
		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)
	})

	t.Run("patches Spec.Approved when approver allows", func(t *testing.T) {
		t.Parallel()
		op := newOperation("op-etcd", "n1", controlplanev1alpha1.OperationComponentEtcd, false)
		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(testControlPlaneNode("n1"), &op).
			WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{}).
			Build()
		r := &reconciler{client: cl}

		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var updated controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "op-etcd"}, &updated))
		require.True(t, updated.Spec.Approved)
	})

	t.Run("does not patch second operation in same reconcile when stage blocks", func(t *testing.T) {
		t.Parallel()
		// Three nodes so apiserver stage is reachable after etcd is idle; in one pass etcd reserves pipeline first.
		etcdOp := newOperation("zzz-etcd", "n1", controlplanev1alpha1.OperationComponentEtcd, false)
		apiOp := newOperation("aaa-apiserver", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false)
		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(
				testControlPlaneNode("n1"),
				testControlPlaneNode("n2"),
				testControlPlaneNode("n3"),
				&etcdOp,
				&apiOp,
			).
			WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{}).
			Build()
		r := &reconciler{client: cl}

		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var etcdUpdated controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "zzz-etcd"}, &etcdUpdated))
		require.True(t, etcdUpdated.Spec.Approved)

		var apiUpdated controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "aaa-apiserver"}, &apiUpdated))
		require.False(t, apiUpdated.Spec.Approved)
	})

	t.Run("does not patch when approver rejects", func(t *testing.T) {
		t.Parallel()
		seed := newOperation("etcd-running", "n1", controlplanev1alpha1.OperationComponentEtcd, true)
		pending := newOperation("etcd-next", "n2", controlplanev1alpha1.OperationComponentEtcd, false)
		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(testControlPlaneNode("n1"), testControlPlaneNode("n2"), &seed, &pending).
			WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{}).
			Build()
		r := &reconciler{client: cl}

		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var updated controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "etcd-next"}, &updated))
		require.False(t, updated.Spec.Approved)
	})
}

func testControlPlaneNode(name string) *controlplanev1alpha1.ControlPlaneNode {
	return &controlplanev1alpha1.ControlPlaneNode{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}
