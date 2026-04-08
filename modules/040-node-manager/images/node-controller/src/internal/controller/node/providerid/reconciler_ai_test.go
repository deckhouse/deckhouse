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

package providerid

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
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	return s
}

// TestAI_StaticNodeGetsProviderID verifies that a Static node with empty providerID
// and no cloud-provider uninitialized taint gets providerID set to "static://".
func TestAI_StaticNodeGetsProviderID(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-3",
			Labels: map[string]string{
				"node.deckhouse.io/group": "worker",
				"node.deckhouse.io/type":  "Static",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: "",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

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
	assert.Equal(t, "static://", updatedNode.Spec.ProviderID)
}

// TestAI_CloudEphemeralNodeSkipped verifies that a CloudEphemeral node is not patched.
func TestAI_CloudEphemeralNodeSkipped(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-2",
			Labels: map[string]string{
				"node.deckhouse.io/group": "worker",
				"node.deckhouse.io/type":  "CloudEphemeral",
			},
		},
	}

	// Filtering of non-Static nodes is done in SetupForPredicates, not in Reconcile.
	r := &Reconciler{}
	preds := r.SetupForPredicates()
	require.Len(t, preds, 1)

	assert.False(t, preds[0].Create(event.CreateEvent{Object: node}),
		"predicate should reject CloudEphemeral node")
}

// TestAI_NodeWithUninitializedTaintSkipped verifies that a node with the
// cloud-provider uninitialized taint is not patched even if it is Static.
func TestAI_NodeWithUninitializedTaintSkipped(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-4",
			Labels: map[string]string{
				"node.deckhouse.io/type": "Static",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{
					Key:    "node.cloudprovider.kubernetes.io/uninitialized",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

	// Filtering of nodes with uninitialized taint is done in SetupForPredicates, not in Reconcile.
	r := &Reconciler{}
	preds := r.SetupForPredicates()
	require.Len(t, preds, 1)

	assert.False(t, preds[0].Create(event.CreateEvent{Object: node}),
		"predicate should reject Static node with uninitialized taint")
}

// TestAI_NodeWithExistingProviderIDSkipped verifies that a node that already has
// a providerID set is not patched.
func TestAI_NodeWithExistingProviderIDSkipped(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-5",
			Labels: map[string]string{
				"node.deckhouse.io/type": "Static",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: "super-provider",
		},
	}

	// Filtering of nodes with existing providerID is done in SetupForPredicates, not in Reconcile.
	r := &Reconciler{}
	preds := r.SetupForPredicates()
	require.Len(t, preds, 1)

	assert.False(t, preds[0].Create(event.CreateEvent{Object: node}),
		"predicate should reject node with existing providerID")
}

// TestAI_NodeWithoutTypeLabel verifies that a node without the type label is skipped.
func TestAI_NodeWithoutTypeLabel(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{},
		},
	}

	// Filtering of nodes without type label is done in SetupForPredicates, not in Reconcile.
	r := &Reconciler{}
	preds := r.SetupForPredicates()
	require.Len(t, preds, 1)

	assert.False(t, preds[0].Create(event.CreateEvent{Object: node}),
		"predicate should reject node without type label")
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

// TestAI_StaticNodeWithoutProviderIDField verifies that a Static node without explicit
// providerID field (defaults to empty) gets patched.
func TestAI_StaticNodeWithoutProviderIDField(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-6",
			Labels: map[string]string{
				"node.deckhouse.io/group": "worker",
				"node.deckhouse.io/type":  "Static",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-6"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedNode := &corev1.Node{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "node-6"}, updatedNode)
	require.NoError(t, err)
	assert.Equal(t, "static://", updatedNode.Spec.ProviderID)
}
