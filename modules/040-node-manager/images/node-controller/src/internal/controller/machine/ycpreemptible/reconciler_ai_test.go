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

package ycpreemptible

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = deckhousev1.AddToScheme(s)
	_ = mcmv1alpha1.AddToScheme(s)
	return s
}

// TestAI_PreemptibleMachineOldEnoughDeleted verifies that a preemptible machine
// whose node is older than the deletion threshold (20h) is deleted.
func TestAI_PreemptibleMachineOldEnoughDeleted(t *testing.T) {
	scheme := newTestScheme()

	machine := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-0",
			Namespace: machineNS,
			Labels: map[string]string{
				"node.deckhouse.io/preemptible": "",
			},
		},
	}

	// Node created 21 hours ago — beyond the 20h threshold.
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-0",
			Labels: map[string]string{
				nodeGroupLabel: "worker",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-21 * time.Hour)),
		},
	}

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
		},
		Status: deckhousev1.NodeGroupStatus{
			Ready: 10,
			Nodes: 10,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(machine, node, ng).
		WithStatusSubresource(ng).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-0", Namespace: machineNS},
	})
	require.NoError(t, err)
	assert.Equal(t, requeueInterval, result.RequeueAfter)

	// Verify the machine was deleted.
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "worker-0",
		Namespace: machineNS,
	}, &mcmv1alpha1.Machine{})
	assert.True(t, err != nil, "machine should be deleted")
}

// TestAI_PreemptibleMachineTooYoungRequeue verifies that a preemptible machine
// whose node is younger than the deletion threshold is requeued without deletion.
func TestAI_PreemptibleMachineTooYoungRequeue(t *testing.T) {
	scheme := newTestScheme()

	machine := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-1",
			Namespace: machineNS,
			Labels: map[string]string{
				"node.deckhouse.io/preemptible": "",
			},
		},
	}

	// Node created 1 hour ago — well within the threshold.
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			Labels: map[string]string{
				nodeGroupLabel: "worker",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(machine, node).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-1", Namespace: machineNS},
	})
	require.NoError(t, err)
	assert.Equal(t, requeueInterval, result.RequeueAfter)

	// Verify the machine still exists.
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "worker-1",
		Namespace: machineNS,
	}, &mcmv1alpha1.Machine{})
	require.NoError(t, err, "machine should still exist")
}

// TestAI_MachineNotFound verifies that reconciling a non-existent machine
// returns no error.
func TestAI_MachineNotFound(t *testing.T) {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: machineNS},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_MachineWithoutPreemptibleLabel verifies that a machine without the
// preemptible label is skipped.
func TestAI_MachineWithoutPreemptibleLabel(t *testing.T) {
	machine := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-2",
			Namespace: machineNS,
			Labels:    map[string]string{},
		},
	}

	// Filtering of machines without the preemptible label is done in SetupForPredicates, not in Reconcile.
	r := &Reconciler{}
	preds := r.SetupForPredicates()
	require.Len(t, preds, 1)

	assert.False(t, preds[0].Create(event.CreateEvent{Object: machine}),
		"predicate should reject machine without preemptible label")

	// Also verify that a machine WITH the label and correct namespace passes the predicate.
	machineWithLabel := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-3",
			Namespace: machineNS,
			Labels: map[string]string{
				"node.deckhouse.io/preemptible": "",
			},
		},
	}
	assert.True(t, preds[0].Create(event.CreateEvent{Object: machineWithLabel}),
		"predicate should accept machine with preemptible label")
}

// TestAI_LowReadinessRatioSkipsDeletion verifies that when the NodeGroup readiness
// ratio is below the threshold, the machine is not deleted even if old enough.
func TestAI_LowReadinessRatioSkipsDeletion(t *testing.T) {
	scheme := newTestScheme()

	machine := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-3",
			Namespace: machineNS,
			Labels: map[string]string{
				"node.deckhouse.io/preemptible": "",
			},
		},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-3",
			Labels: map[string]string{
				nodeGroupLabel: "worker",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-21 * time.Hour)),
		},
	}

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
		},
		Status: deckhousev1.NodeGroupStatus{
			Ready: 5,
			Nodes: 10,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(machine, node, ng).
		WithStatusSubresource(ng).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-3", Namespace: machineNS},
	})
	require.NoError(t, err)
	assert.Equal(t, requeueInterval, result.RequeueAfter)

	// Verify the machine still exists (not deleted due to low readiness).
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "worker-3",
		Namespace: machineNS,
	}, &mcmv1alpha1.Machine{})
	require.NoError(t, err, "machine should still exist")
}
