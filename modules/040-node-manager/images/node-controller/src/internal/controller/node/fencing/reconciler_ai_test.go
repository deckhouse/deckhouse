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

package fencing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = coordinationv1.AddToScheme(s)
	return s
}

// TestAI_NodeWithExpiredLease verifies that when a node has the fencing-enabled label
// and its lease has been expired for longer than fencingTimeout, the node is deleted.
func TestAI_NodeWithExpiredLease(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-0",
			Labels: map[string]string{
				fencingEnabledLabel: "",
			},
		},
	}

	// Create a lease with a RenewTime far in the past (expired > 60s ago).
	expiredTime := metav1.NewMicroTime(time.Now().Add(-5 * time.Minute))
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-0",
			Namespace: leaseNamespace,
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &expiredTime,
		},
	}

	// Create a pod on the node.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-0",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node, lease, pod).
		WithIndex(&corev1.Pod{}, "spec.nodeName", func(obj client.Object) []string {
			return []string{obj.(*corev1.Pod).Spec.NodeName}
		}).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-0"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the node was deleted.
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker-0"}, &corev1.Node{})
	assert.True(t, err != nil, "node should be deleted")

	// Verify the pod was deleted.
	podList := &corev1.PodList{}
	err = fakeClient.List(context.Background(), podList)
	require.NoError(t, err)
	assert.Empty(t, podList.Items, "pods should be deleted")
}

// TestAI_NodeWithFreshLease verifies that when a node has a fresh (non-expired) lease,
// the reconciler requeues and does not delete the node.
func TestAI_NodeWithFreshLease(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			Labels: map[string]string{
				fencingEnabledLabel: "",
			},
		},
	}

	// Create a lease with a recent RenewTime.
	freshTime := metav1.NewMicroTime(time.Now())
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-1",
			Namespace: leaseNamespace,
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &freshTime,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node, lease).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-1"},
	})
	require.NoError(t, err)
	assert.Equal(t, 1*time.Minute, result.RequeueAfter)

	// Verify the node still exists.
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, &corev1.Node{})
	require.NoError(t, err, "node should still exist")
}

// TestAI_NodeWithFencingDisableAnnotation verifies that a node with the
// fencing-disable annotation is skipped (requeued) and not deleted.
func TestAI_NodeWithFencingDisableAnnotation(t *testing.T) {
	scheme := newTestScheme()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-2",
			Labels: map[string]string{
				fencingEnabledLabel: "",
			},
			Annotations: map[string]string{
				annotationFencingDisable: "",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-2"},
	})
	require.NoError(t, err)
	assert.Equal(t, 1*time.Minute, result.RequeueAfter)

	// Verify the node still exists.
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker-2"}, &corev1.Node{})
	require.NoError(t, err, "node should still exist")
}

// TestAI_NodeWithoutFencingLabel verifies that a node without the fencing-enabled
// label is skipped without error.
func TestAI_NodeWithoutFencingLabel(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-3",
			Labels: map[string]string{},
		},
	}

	// Filtering of nodes without fencing label is done in SetupForPredicates, not in Reconcile.
	r := &Reconciler{}
	preds := r.SetupForPredicates()
	require.Len(t, preds, 1)

	assert.False(t, preds[0].Create(event.CreateEvent{Object: node}),
		"predicate should reject node without fencing label")

	// Also verify that a node WITH the label passes the predicate.
	nodeWithLabel := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-4",
			Labels: map[string]string{
				fencingEnabledLabel: "",
			},
		},
	}
	assert.True(t, preds[0].Create(event.CreateEvent{Object: nodeWithLabel}),
		"predicate should accept node with fencing label")
}

// TestAI_NodeNotFound verifies that reconciling a non-existent node returns no error.
func TestAI_NodeNotFound(t *testing.T) {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}
