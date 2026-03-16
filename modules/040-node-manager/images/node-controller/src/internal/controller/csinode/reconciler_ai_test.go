//go:build ai_tests

/*
Copyright 2025 Flant JSC

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

package csinode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = storagev1.AddToScheme(s)
	return s
}

// TestAI_RemoveCSITaintWhenDriversPresent verifies that the csi-not-bootstrapped taint
// is removed from a node when the corresponding CSINode has at least one driver.
func TestAI_RemoveCSITaintWhenDriversPresent(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    "somekey-1",
					Effect: corev1.TaintEffectPreferNoSchedule,
				},
				{
					Key:    csiNotBootstrappedTaint,
					Effect: corev1.TaintEffectNoSchedule,
					Value:  "",
				},
			},
		},
	}

	csiNode := &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
		},
		Spec: storagev1.CSINodeSpec{
			Drivers: []storagev1.CSINodeDriver{
				{Name: "test"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, csiNode).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updatedNode)
	require.NoError(t, err)

	// Only the non-CSI taint should remain
	require.Len(t, updatedNode.Spec.Taints, 1)
	assert.Equal(t, "somekey-1", updatedNode.Spec.Taints[0].Key)
}

// TestAI_CSINodeWithoutDriversSkipped verifies that a CSINode without drivers
// does not cause taint removal on the node.
func TestAI_CSINodeWithoutDriversSkipped(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-4",
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    csiNotBootstrappedTaint,
					Effect: corev1.TaintEffectNoSchedule,
					Value:  "",
				},
			},
		},
	}

	csiNode := &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-4",
		},
		Spec: storagev1.CSINodeSpec{
			Drivers: []storagev1.CSINodeDriver{},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, csiNode).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-4"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node-4"}, updatedNode)
	require.NoError(t, err)

	// Taint should remain since CSINode has no drivers
	require.Len(t, updatedNode.Spec.Taints, 1)
	assert.Equal(t, csiNotBootstrappedTaint, updatedNode.Spec.Taints[0].Key)
}

// TestAI_NodeWithoutCSITaintUnchanged verifies that a node that does not have
// the csi-not-bootstrapped taint is not modified even when CSINode has drivers.
func TestAI_NodeWithoutCSITaintUnchanged(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-3",
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    "somekey-3",
					Effect: corev1.TaintEffectPreferNoSchedule,
				},
			},
		},
	}

	csiNode := &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-3",
		},
		Spec: storagev1.CSINodeSpec{
			Drivers: []storagev1.CSINodeDriver{
				{Name: "test"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, csiNode).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-3"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node-3"}, updatedNode)
	require.NoError(t, err)

	// Taints should remain unchanged
	require.Len(t, updatedNode.Spec.Taints, 1)
	assert.Equal(t, "somekey-3", updatedNode.Spec.Taints[0].Key)
}

// TestAI_CSINodeNotFound verifies that reconciling a non-existent CSINode returns no error.
func TestAI_CSINodeNotFound(t *testing.T) {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_NodeNotFoundForCSINode verifies that when a CSINode exists but the
// corresponding Node does not, the reconciler handles it gracefully.
func TestAI_NodeNotFoundForCSINode(t *testing.T) {
	scheme := newTestScheme()

	csiNode := &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ghost-node",
		},
		Spec: storagev1.CSINodeSpec{
			Drivers: []storagev1.CSINodeDriver{
				{Name: "test"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(csiNode).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ghost-node"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_RemoveCSITaintOnlyTaint verifies that when csi-not-bootstrapped is the only
// taint on the node, it is fully removed resulting in an empty taints list.
func TestAI_RemoveCSITaintOnlyTaint(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-single-taint",
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    csiNotBootstrappedTaint,
					Effect: corev1.TaintEffectNoSchedule,
					Value:  "",
				},
			},
		},
	}

	csiNode := &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-single-taint",
		},
		Spec: storagev1.CSINodeSpec{
			Drivers: []storagev1.CSINodeDriver{
				{Name: "test-driver"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, csiNode).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-single-taint"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node-single-taint"}, updatedNode)
	require.NoError(t, err)

	assert.Empty(t, updatedNode.Spec.Taints)
}

// TestAI_NodeWithNoTaintsUnchanged verifies that a node with no taints at all
// is not modified even when CSINode has drivers.
func TestAI_NodeWithNoTaintsUnchanged(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-5",
		},
	}

	csiNode := &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-5",
		},
		Spec: storagev1.CSINodeSpec{
			Drivers: []storagev1.CSINodeDriver{
				{Name: "test"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node, csiNode).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-5"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node-5"}, updatedNode)
	require.NoError(t, err)

	assert.Empty(t, updatedNode.Spec.Taints)
}
