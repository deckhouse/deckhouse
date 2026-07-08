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

package virtualcontrolplaneapprover

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
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
}

func TestReconciler_Reconcile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("no virtual control plane nodes returns without error", func(t *testing.T) {
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
			WithObjects(testVirtualNode("n1", true)).
			Build()
		r := newReconciler(cl)

		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)
	})

	t.Run("not-ready node is ignored, no approval happens", func(t *testing.T) {
		t.Parallel()
		op := testOperation("op-api-1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer)
		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(testVirtualNode("n1", false), &op).
			WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{}).
			Build()
		r := newReconciler(cl)

		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var updated controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "op-api-1", Namespace: constants.KubeSystemNamespace}, &updated))
		require.False(t, updated.Spec.Approved)
	})

	t.Run("approves operation targeting a ready node", func(t *testing.T) {
		t.Parallel()
		op := testOperation("op-api-1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer)
		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(testVirtualNode("n1", true), &op).
			WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{}).
			Build()
		r := newReconciler(cl)

		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var updated controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "op-api-1", Namespace: constants.KubeSystemNamespace}, &updated))
		require.True(t, updated.Spec.Approved)
	})

	t.Run("second apiserver operation on distinct node is blocked by workload concurrency limit", func(t *testing.T) {
		t.Parallel()
		// 2 ready masters -> workload concurrency limit is max(1, 2-1) = 1.
		op1 := testOperation("op-api-1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer)
		op2 := testOperation("op-api-2", "n2", controlplanev1alpha1.OperationComponentKubeAPIServer)
		cl := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(testVirtualNode("n1", true), testVirtualNode("n2", true), &op1, &op2).
			WithStatusSubresource(&controlplanev1alpha1.ControlPlaneOperation{}).
			Build()
		r := newReconciler(cl)

		_, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		var updated1 controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "op-api-1", Namespace: constants.KubeSystemNamespace}, &updated1))
		require.True(t, updated1.Spec.Approved)

		var updated2 controlplanev1alpha1.ControlPlaneOperation
		require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: "op-api-2", Namespace: constants.KubeSystemNamespace}, &updated2))
		require.False(t, updated2.Spec.Approved)
	})

	// Note: filtering out normal-mode ControlPlaneOperations/ControlPlaneNodes is delegated to the
	// manager's cache scope (see manager.virtualConfigurator.configureOptions), not to this
	// reconciler — a fake client in tests bypasses that cache entirely, so there's no meaningful
	// "ignores non-virtual operations" case to assert at this layer; see reconciler.go's Reconcile
	// comment for where that guarantee actually lives.
}

func testVirtualNode(name string, ready bool) *controlplanev1alpha1.ControlPlaneNode {
	node := &controlplanev1alpha1.ControlPlaneNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: constants.KubeSystemNamespace,
			Labels: map[string]string{
				constants.ControlPlaneTypeLabelKey: string(constants.ControlPlaneTypeVirtual),
			},
		},
	}
	if ready {
		meta.SetStatusCondition(&node.Status.Conditions, metav1.Condition{
			Type:   controlplanev1alpha1.CPNConditionAPIServerReady,
			Status: metav1.ConditionTrue,
			Reason: controlplanev1alpha1.CPNReasonReady,
		})
	}
	return node
}

func testOperation(name, node string, component controlplanev1alpha1.OperationComponent) controlplanev1alpha1.ControlPlaneOperation {
	return controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: constants.KubeSystemNamespace,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey: node,
				constants.ControlPlaneTypeLabelKey:     string(constants.ControlPlaneTypeVirtual),
			},
		},
		Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
			NodeName:  node,
			Component: component,
			Approved:  false,
		},
	}
}
