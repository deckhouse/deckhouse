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

package controlplane

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

	infrastructurev1alpha1 "github.com/deckhouse/node-controller/api/infrastructure.cluster.x-k8s.io/v1alpha1"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = infrastructurev1alpha1.AddToScheme(s)
	return s
}

// TestAI_DCPFoundWithMasterNodes verifies that when a DeckhouseControlPlane exists
// and master nodes are present, the status is updated with correct replica counts and readiness.
func TestAI_DCPFoundWithMasterNodes(t *testing.T) {
	scheme := newTestScheme()

	dcp := &infrastructurev1alpha1.DeckhouseControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cp",
			Namespace: controlPlaneNamespace,
		},
	}

	readyNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "master-1",
			Labels: map[string]string{masterNodeGroupLabel: ""},
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

	notReadyNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "master-2",
			Labels: map[string]string{masterNodeGroupLabel: ""},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

	readyNode2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "master-3",
			Labels: map[string]string{masterNodeGroupLabel: ""},
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

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dcp, readyNode, notReadyNode, readyNode2).
		WithStatusSubresource(dcp).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cp",
			Namespace: controlPlaneNamespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedCP := &infrastructurev1alpha1.DeckhouseControlPlane{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "test-cp",
		Namespace: controlPlaneNamespace,
	}, updatedCP)
	require.NoError(t, err)

	assert.True(t, updatedCP.Status.Initialized)
	assert.True(t, updatedCP.Status.Ready)
	assert.True(t, updatedCP.Status.ExternalManagedControlPlane)
	assert.Equal(t, int32(3), updatedCP.Status.Replicas)
	assert.Equal(t, int32(2), updatedCP.Status.ReadyReplicas)
	assert.Equal(t, int32(1), updatedCP.Status.UnavailableReplicas)
}

// TestAI_DCPNotFound verifies that reconciling a non-existent DeckhouseControlPlane
// returns no error and an empty result.
func TestAI_DCPNotFound(t *testing.T) {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nonexistent",
			Namespace: controlPlaneNamespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_DCPNoMasterNodes verifies that when a DeckhouseControlPlane exists
// but no master nodes are present, status reflects zero replicas.
func TestAI_DCPNoMasterNodes(t *testing.T) {
	scheme := newTestScheme()

	dcp := &infrastructurev1alpha1.DeckhouseControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cp",
			Namespace: controlPlaneNamespace,
		},
	}

	// A worker node (no master label) should not be counted.
	workerNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-1",
			Labels: map[string]string{"node-role.kubernetes.io/worker": ""},
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

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dcp, workerNode).
		WithStatusSubresource(dcp).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient
	r.Scheme = scheme

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cp",
			Namespace: controlPlaneNamespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updatedCP := &infrastructurev1alpha1.DeckhouseControlPlane{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "test-cp",
		Namespace: controlPlaneNamespace,
	}, updatedCP)
	require.NoError(t, err)

	assert.True(t, updatedCP.Status.Initialized)
	assert.True(t, updatedCP.Status.Ready)
	assert.True(t, updatedCP.Status.ExternalManagedControlPlane)
	assert.Equal(t, int32(0), updatedCP.Status.Replicas)
	assert.Equal(t, int32(0), updatedCP.Status.ReadyReplicas)
	assert.Equal(t, int32(0), updatedCP.Status.UnavailableReplicas)
}
