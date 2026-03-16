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

package chaosmonkey

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

// TestAI_ChaosModeDrainAndDelete verifies that when a NodeGroup has Chaos.Mode=DrainAndDelete,
// a random machine is annotated and deleted.
func TestAI_ChaosModeDrainAndDelete(t *testing.T) {
	scheme := newTestScheme()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			Chaos: &deckhousev1.ChaosSpec{
				Mode: deckhousev1.ChaosModeDrainAndDelete,
			},
		},
		Status: deckhousev1.NodeGroupStatus{
			Ready:   3,
			Nodes:   3,
			Desired: 3,
		},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-node-0",
			Labels: map[string]string{
				nodeGroupLabel: "worker",
			},
		},
	}

	machine := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-machine-0",
			Namespace: machineNamespace,
			Labels: map[string]string{
				"node": "worker-node-0",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ng, node, machine).
		WithStatusSubresource(ng).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker"},
	})
	require.NoError(t, err)
	assert.Equal(t, defaultChaosPeriod, result.RequeueAfter)

	// Verify the machine was deleted (should not be found).
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "worker-machine-0",
		Namespace: machineNamespace,
	}, &mcmv1alpha1.Machine{})
	assert.True(t, err != nil, "machine should be deleted")
}

// TestAI_ChaosNotEnabled verifies that a NodeGroup without chaos mode is skipped.
func TestAI_ChaosNotEnabled(t *testing.T) {
	scheme := newTestScheme()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-no-chaos",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ng).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-no-chaos"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_ChaosDisabledMode verifies that a NodeGroup with Chaos.Mode=Disabled is skipped.
func TestAI_ChaosDisabledMode(t *testing.T) {
	scheme := newTestScheme()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-disabled",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			Chaos: &deckhousev1.ChaosSpec{
				Mode: deckhousev1.ChaosModeDisabled,
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ng).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-disabled"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_NodeGroupNotFound verifies that reconciling a non-existent NodeGroup returns no error.
func TestAI_NodeGroupNotFound(t *testing.T) {
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

// TestAI_NoMachinesAvailable verifies that when there are no machines matching
// the NodeGroup, nothing is deleted and reconcile still succeeds.
func TestAI_NoMachinesAvailable(t *testing.T) {
	scheme := newTestScheme()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-empty",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			Chaos: &deckhousev1.ChaosSpec{
				Mode: deckhousev1.ChaosModeDrainAndDelete,
			},
		},
		Status: deckhousev1.NodeGroupStatus{
			Ready:   2,
			Nodes:   2,
			Desired: 2,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ng).
		WithStatusSubresource(ng).
		Build()

	r := &Reconciler{}
	r.Client = fakeClient

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-empty"},
	})
	require.NoError(t, err)
	assert.Equal(t, defaultChaosPeriod, result.RequeueAfter)
}
