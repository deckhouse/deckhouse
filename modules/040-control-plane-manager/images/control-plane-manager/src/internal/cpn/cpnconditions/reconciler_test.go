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

package cpnconditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

var testScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(controlplanev1alpha1.AddToScheme(testScheme))
	utilruntime.Must(corev1.AddToScheme(testScheme))
}

const (
	testNodeName = "master-0"
	testCPNUID   = types.UID("cpn-uid-1")
)

// minimalCPN returns a CPN with only etcd configured and empty status.
func minimalCPN() *controlplanev1alpha1.ControlPlaneNode {
	return &controlplanev1alpha1.ControlPlaneNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testNodeName,
			Namespace:  constants.KubeSystemNamespace,
			UID:        testCPNUID,
			Generation: 2,
		},
		Spec: controlplanev1alpha1.ControlPlaneNodeSpec{
			CAChecksum: "ca-1",
			Components: controlplanev1alpha1.ComponentsSpec{
				Etcd: controlplanev1alpha1.ComponentSpec{
					Checksums: controlplanev1alpha1.Checksums{Config: "cfg-etcd-1", PKI: "pki-etcd-1"},
				},
			},
		},
	}
}

// runningEtcdOperation returns an approved in-progress operation owned by the CPN.
func runningEtcdOperation() *controlplanev1alpha1.ControlPlaneOperation {
	return &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd-op-1",
			Namespace: constants.KubeSystemNamespace,
			UID:       types.UID("op-uid-1"),
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey: testNodeName,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: controlplanev1alpha1.GroupVersion.String(),
				Kind:       "ControlPlaneNode",
				Name:       testNodeName,
				UID:        testCPNUID,
				Controller: ptr.To(true),
			}},
		},
		Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
			NodeName:              testNodeName,
			Component:             controlplanev1alpha1.OperationComponentEtcd,
			Steps:                 []controlplanev1alpha1.StepName{controlplanev1alpha1.StepSyncManifests},
			DesiredConfigChecksum: "cfg-etcd-1",
			DesiredPKIChecksum:    "pki-etcd-1",
			DesiredCAChecksum:     "ca-1",
			Approved:              true,
		},
		Status: controlplanev1alpha1.ControlPlaneOperationStatus{
			Conditions: []metav1.Condition{{
				Type:   controlplanev1alpha1.CPOConditionCompleted,
				Status: metav1.ConditionFalse,
				Reason: controlplanev1alpha1.CPOReasonOperationInProgress,
			}},
		},
	}
}

// readyEtcdPod returns a ready static etcd pod of the test node.
func readyEtcdPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd-" + testNodeName,
			Namespace: constants.KubeSystemNamespace,
			Labels: map[string]string{
				constants.StaticPodComponentLabelKey: "etcd",
			},
		},
		Spec: corev1.PodSpec{NodeName: testNodeName},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}
}

func TestReconcilerUpdatesConditionsFromOperations(t *testing.T) {
	ctx := context.Background()
	cpn := minimalCPN()

	cl := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(cpn, runningEtcdOperation(), readyEtcdPod()).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneNode{}).
		Build()

	r := New(cl, cl)
	res, err := r.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKey{Name: testNodeName, Namespace: constants.KubeSystemNamespace},
	})
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter)

	updated := &controlplanev1alpha1.ControlPlaneNode{}
	require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: testNodeName, Namespace: constants.KubeSystemNamespace}, updated))

	// The pod is healthy while an operation runs: no false alarms (PodReady).
	cond := meta.FindStatusCondition(updated.Status.Conditions, controlplanev1alpha1.CPNConditionEtcdPodReady)
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionTrue, cond.Status)
	require.Equal(t, controlplanev1alpha1.CPNReasonReady, cond.Reason)

	inSync := meta.FindStatusCondition(updated.Status.Conditions, controlplanev1alpha1.CPNConditionEtcdInSync)
	require.NotNil(t, inSync)
	require.Equal(t, metav1.ConditionFalse, inSync.Status)
	require.Equal(t, controlplanev1alpha1.CPNReasonOperationInFlight, inSync.Reason)
	require.Equal(t, "etcd-op-1", inSync.Message)
	require.Equal(t, int64(2), inSync.ObservedGeneration)

	// The summary (kubectl column, dhctl): a ready pod with a running operation is not Ready yet.
	ready := meta.FindStatusCondition(updated.Status.Conditions, controlplanev1alpha1.CPNConditionEtcdReady)
	require.NotNil(t, ready)
	require.Equal(t, metav1.ConditionFalse, ready.Status)
	require.Equal(t, controlplanev1alpha1.CPNReasonOperationInFlight, ready.Reason)
}

func TestReconcilerIgnoresForeignObjects(t *testing.T) {
	ctx := context.Background()
	cpn := minimalCPN()

	foreign := runningEtcdOperation()
	foreign.OwnerReferences[0].UID = types.UID("another-cpn-uid") // owned by a stale CPN instance

	foreignPod := readyEtcdPod()
	foreignPod.Name, foreignPod.Spec.NodeName = "etcd-master-1", "master-1" // another node's mirror pod

	cl := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(cpn, foreign, foreignPod).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneNode{}).
		Build()

	r := New(cl, cl)
	_, err := r.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKey{Name: testNodeName, Namespace: constants.KubeSystemNamespace},
	})
	require.NoError(t, err)

	updated := &controlplanev1alpha1.ControlPlaneNode{}
	require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: testNodeName, Namespace: constants.KubeSystemNamespace}, updated))

	// No pod of this node -> not ready (the foreign pod does not count).
	inSync := meta.FindStatusCondition(updated.Status.Conditions, controlplanev1alpha1.CPNConditionEtcdInSync)
	require.NotNil(t, inSync)
	// No owned operations and nothing applied yet (empty status) -> out of sync.
	require.Equal(t, metav1.ConditionFalse, inSync.Status)
	require.Equal(t, controlplanev1alpha1.CPNReasonOutOfSync, inSync.Reason)

	cond := meta.FindStatusCondition(updated.Status.Conditions, controlplanev1alpha1.CPNConditionEtcdPodReady)
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionFalse, cond.Status)
	require.Equal(t, "static pod not found", cond.Message)
}

func TestReconcilerReturnsOnMissingCPN(t *testing.T) {
	ctx := context.Background()
	cl := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneNode{}).
		Build()

	r := New(cl, cl)
	res, err := r.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKey{Name: testNodeName, Namespace: constants.KubeSystemNamespace},
	})
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
}
func TestReconcilerIsNoOpWhenConditionsInSync(t *testing.T) {
	ctx := context.Background()
	key := client.ObjectKey{Name: testNodeName, Namespace: constants.KubeSystemNamespace}

	cl := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(minimalCPN(), runningEtcdOperation(), readyEtcdPod()).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneNode{}).
		Build()
	r := New(cl, cl)

	_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
	require.NoError(t, err)
	first := &controlplanev1alpha1.ControlPlaneNode{}
	require.NoError(t, cl.Get(ctx, key, first))

	res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
	require.NoError(t, err)
	require.Equal(t, requeueInterval, res.RequeueAfter)

	second := &controlplanev1alpha1.ControlPlaneNode{}
	require.NoError(t, cl.Get(ctx, key, second))
	// No patch was sent; LastTransitionTime of unchanged conditions is preserved.
	require.Equal(t, first.ResourceVersion, second.ResourceVersion)
	require.Equal(t, first.Status.Conditions, second.Status.Conditions)
}

func TestReconcilerKeepsForeignConditions(t *testing.T) {
	ctx := context.Background()
	key := client.ObjectKey{Name: testNodeName, Namespace: constants.KubeSystemNamespace}

	cpn := minimalCPN()
	foreign := metav1.Condition{
		Type:               "SomeoneElsesCondition",
		Status:             metav1.ConditionTrue,
		Reason:             "Foreign",
		LastTransitionTime: metav1.Now(),
	}
	cpn.Status.Conditions = []metav1.Condition{foreign}

	cl := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(cpn, readyEtcdPod()).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneNode{}).
		Build()

	_, err := New(cl, cl).Reconcile(ctx, reconcile.Request{NamespacedName: key})
	require.NoError(t, err)

	updated := &controlplanev1alpha1.ControlPlaneNode{}
	require.NoError(t, cl.Get(ctx, key, updated))
	require.NotNil(t, meta.FindStatusCondition(updated.Status.Conditions, foreign.Type))
	require.NotNil(t, meta.FindStatusCondition(updated.Status.Conditions, controlplanev1alpha1.CPNConditionEtcdPodReady))
}

func TestReconcilerReturnsConflictWithOptimisticLock(t *testing.T) {
	ctx := context.Background()
	key := client.ObjectKey{Name: testNodeName, Namespace: constants.KubeSystemNamespace}

	var patchData string
	cl := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(minimalCPN(), readyEtcdPod()).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneNode{}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourcePatch: func(_ context.Context, _ client.Client, _ string, obj client.Object, patch client.Patch, _ ...client.SubResourcePatchOption) error {
				data, err := patch.Data(obj)
				require.NoError(t, err)
				patchData = string(data)
				return apierrors.NewConflict(controlplanev1alpha1.GroupVersion.WithResource("controlplanenodes").GroupResource(), obj.GetName(), nil)
			},
		}).
		Build()

	cpn := &controlplanev1alpha1.ControlPlaneNode{}
	require.NoError(t, cl.Get(ctx, key, cpn))

	// ADR controller-standard/050: a conflict is returned, the next iteration recomputes.
	_, err := New(cl, cl).Reconcile(ctx, reconcile.Request{NamespacedName: key})
	require.True(t, apierrors.IsConflict(err), "expected conflict, got %v", err)
	require.Contains(t, patchData, `"resourceVersion":"`+cpn.ResourceVersion+`"`, "patch must carry the optimistic lock")
}

func TestReconcilerIgnoresNotFoundOnPatch(t *testing.T) {
	ctx := context.Background()
	key := client.ObjectKey{Name: testNodeName, Namespace: constants.KubeSystemNamespace}

	cl := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(minimalCPN(), readyEtcdPod()).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneNode{}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourcePatch: func(_ context.Context, _ client.Client, _ string, obj client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
				return apierrors.NewNotFound(controlplanev1alpha1.GroupVersion.WithResource("controlplanenodes").GroupResource(), obj.GetName())
			},
		}).
		Build()

	res, err := New(cl, cl).Reconcile(ctx, reconcile.Request{NamespacedName: key})
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
}

func TestReconcilerReplacesStaleConditions(t *testing.T) {
	ctx := context.Background()
	key := client.ObjectKey{Name: testNodeName, Namespace: constants.KubeSystemNamespace}

	cpn := minimalCPN()
	cpn.Status.Components.Etcd.Checksums.Config = "applied-by-cpnreconcile"
	cpn.Status.Conditions = []metav1.Condition{{
		Type:               controlplanev1alpha1.CPNConditionEtcdPodReady,
		Status:             metav1.ConditionTrue,
		Reason:             controlplanev1alpha1.CPNReasonReady,
		LastTransitionTime: metav1.Now(),
	}}

	// The operation carries another node's label: filtered out by the label selector despite the owner UID.
	otherNodeOp := runningEtcdOperation()
	otherNodeOp.Labels[constants.ControlPlaneNodeNameLabelKey] = "master-1"

	cl := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(cpn, otherNodeOp).
		WithStatusSubresource(&controlplanev1alpha1.ControlPlaneNode{}).
		Build()

	_, err := New(cl, cl).Reconcile(ctx, reconcile.Request{NamespacedName: key})
	require.NoError(t, err)

	updated := &controlplanev1alpha1.ControlPlaneNode{}
	require.NoError(t, cl.Get(ctx, key, updated))
	ready := meta.FindStatusCondition(updated.Status.Conditions, controlplanev1alpha1.CPNConditionEtcdPodReady)
	require.NotNil(t, ready)
	require.Equal(t, metav1.ConditionFalse, ready.Status)
	require.Equal(t, "static pod not found", ready.Message)
	inSync := meta.FindStatusCondition(updated.Status.Conditions, controlplanev1alpha1.CPNConditionEtcdInSync)
	require.NotNil(t, inSync)
	require.Equal(t, controlplanev1alpha1.CPNReasonOutOfSync, inSync.Reason)
	// Fields owned by cpnreconcile are not touched.
	require.Equal(t, "applied-by-cpnreconcile", updated.Status.Components.Etcd.Checksums.Config)
}
