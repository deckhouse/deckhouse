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

package controlplaneapprover

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

var testScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(controlplanev1alpha1.AddToScheme(testScheme))
	utilruntime.Must(corev1.AddToScheme(testScheme))
}

func TestReconciler_Reconcile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("no control plane nodes returns without error", func(t *testing.T) {
		t.Parallel()
		cl := fake.NewClientBuilder().WithScheme(testScheme).Build()
		r := newReconciler(cl)
		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)
	})

	t.Run("no operations returns without error", func(t *testing.T) {
		t.Parallel()
		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(testControlPlaneNode("n1")).
			Build()
		r := newReconciler(cl)
		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)
	})

	t.Run("workload components ignore arbiters", func(t *testing.T) {
		t.Parallel()
		// 2 masters + 5 arbiters.
		// For KubeAPIServer, the limit is max(1, masters-1) = max(1, 2-1) = 1.
		// Arbiters should be ignored for workload components.
		op1 := newOperation("op-api-1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)
		op2 := newOperation("op-api-2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer, false)

		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(
				testControlPlaneNode("n1"),
				testControlPlaneNode("n2"),
				testArbiterNode("a1"),
				testArbiterNode("a2"),
				testArbiterNode("a3"),
				testArbiterNode("a4"),
				testArbiterNode("a5"),
				&op1,
				&op2,
			).
			WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{}).
			Build()
		r := newReconciler(cl)

		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var updated1 controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "op-api-1", Namespace: constants.KubeSystemNamespace}, &updated1))
		// First one should be approved (limit is 1)
		require.True(t, updated1.Spec.Approved)

		var updated2 controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "op-api-2", Namespace: constants.KubeSystemNamespace}, &updated2))
		// Second one should NOT be approved because limit is 1 (based only on 2 masters), despite 5 arbiters
		require.False(t, updated2.Spec.Approved)
	})

	t.Run("patches Spec.Approved when approver allows", func(t *testing.T) {
		t.Parallel()
		op := newOperation("op-etcd", "n1", controlplanev1alpha1.OperationComponentEtcd, false)
		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(testControlPlaneNode("n1"), &op).
			WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{}).
			Build()
		r := newReconciler(cl)

		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var updated controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "op-etcd", Namespace: constants.KubeSystemNamespace}, &updated))
		require.True(t, updated.Spec.Approved)
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
		r := newReconciler(cl)

		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var updated controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "etcd-next", Namespace: constants.KubeSystemNamespace}, &updated))
		require.False(t, updated.Spec.Approved)
	})
}

func testControlPlaneNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{constants.ControlPlaneNodeLabelKey: ""},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func testArbiterNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{constants.EtcdArbiterNodeLabelKey: ""},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func newOperation(name, node string, component controlplanev1alpha1.OperationComponent, approved bool) controlplanev1alpha1.ControlPlaneOperation {
	return controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: constants.KubeSystemNamespace,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey: node,
			},
		},
		Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
			NodeName:  node,
			Component: component,
			Approved:  approved,
		},
	}
}
