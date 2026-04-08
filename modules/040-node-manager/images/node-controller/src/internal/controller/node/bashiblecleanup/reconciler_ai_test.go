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

package bashiblecleanup

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	return s
}

// TestAI_RemoveLabelAndTaint verifies that both the bashible-first-run-finished label
// and the bashible-uninitialized taint are removed from a node that has both.
func TestAI_RemoveLabelAndTaint(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Labels: map[string]string{
				bashibleFirstRunFinishedLabel: "",
				"example-label":               "value",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    bashibleUninitializedTaintKey,
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "example-taint",
					Effect: corev1.TaintEffectNoExecute,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node1"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node1"}, updatedNode)
	require.NoError(t, err)

	// Label should be removed
	_, hasLabel := updatedNode.Labels[bashibleFirstRunFinishedLabel]
	assert.False(t, hasLabel, "bashible-first-run-finished label should be removed")

	// Other labels should be preserved
	assert.Equal(t, "value", updatedNode.Labels["example-label"])

	// Bashible taint should be removed, other taints preserved
	assert.Len(t, updatedNode.Spec.Taints, 1)
	assert.Equal(t, "example-taint", updatedNode.Spec.Taints[0].Key)
}

// TestAI_RemoveLabelOnly verifies that only the label is removed when the node has
// the bashible label but no bashible taint.
func TestAI_RemoveLabelOnly(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Labels: map[string]string{
				bashibleFirstRunFinishedLabel: "",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node1"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node1"}, updatedNode)
	require.NoError(t, err)

	_, hasLabel := updatedNode.Labels[bashibleFirstRunFinishedLabel]
	assert.False(t, hasLabel, "bashible-first-run-finished label should be removed")
	assert.Empty(t, updatedNode.Spec.Taints)
}

// TestAI_NoLabelNoAction verifies that a node without the bashible label
// is not modified (taint remains if present).
func TestAI_NoLabelNoAction(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node1",
			Labels: map[string]string{},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    bashibleUninitializedTaintKey,
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node1"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node1"}, updatedNode)
	require.NoError(t, err)

	// Taint should remain unchanged since the label was not present
	assert.Len(t, updatedNode.Spec.Taints, 1)
	assert.Equal(t, bashibleUninitializedTaintKey, updatedNode.Spec.Taints[0].Key)
}

// TestAI_NodeNotFound verifies that reconciling a non-existent node returns no error.
func TestAI_NodeNotFound(t *testing.T) {
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

// TestAI_MultipleNodesProcessedCorrectly verifies that each node is processed
// independently based on its own labels and taints.
func TestAI_MultipleNodesProcessedCorrectly(t *testing.T) {
	scheme := newTestScheme()

	// node1: has both label and taint
	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Labels: map[string]string{
				bashibleFirstRunFinishedLabel: "",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    bashibleUninitializedTaintKey,
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

	// node2: has only label, no taint
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
			Labels: map[string]string{
				bashibleFirstRunFinishedLabel: "",
			},
		},
	}

	// node3: has only taint, no label
	node3 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node3",
			Labels: map[string]string{},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    bashibleUninitializedTaintKey,
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node1, node2, node3).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	// Process node1
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node1"},
	})
	require.NoError(t, err)

	// Process node2
	_, err = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node2"},
	})
	require.NoError(t, err)

	// Process node3
	_, err = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node3"},
	})
	require.NoError(t, err)

	// Verify node1: label and taint removed
	updatedNode1 := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node1"}, updatedNode1)
	require.NoError(t, err)
	_, hasLabel1 := updatedNode1.Labels[bashibleFirstRunFinishedLabel]
	assert.False(t, hasLabel1)
	assert.Empty(t, updatedNode1.Spec.Taints)

	// Verify node2: label removed
	updatedNode2 := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node2"}, updatedNode2)
	require.NoError(t, err)
	_, hasLabel2 := updatedNode2.Labels[bashibleFirstRunFinishedLabel]
	assert.False(t, hasLabel2)

	// Verify node3: taint remains (no label trigger)
	updatedNode3 := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node3"}, updatedNode3)
	require.NoError(t, err)
	assert.Len(t, updatedNode3.Spec.Taints, 1)
	assert.Equal(t, bashibleUninitializedTaintKey, updatedNode3.Spec.Taints[0].Key)
}
