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

package status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/dynr"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = deckhousev1.AddToScheme(s)
	_ = deckhousev1alpha1.AddToScheme(s)
	_ = mcmv1alpha1.AddToScheme(s)
	return s
}

func newReconciler(cl client.Client, scheme *runtime.Scheme) *NodeGroupStatusReconciler {
	r := &NodeGroupStatusReconciler{}
	r.Base = dynr.Base{
		Client: cl,
		Scheme: scheme,
	}
	return r
}

func TestAI_StatusWithZeroNodes(t *testing.T) {
	s := newTestScheme()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ng",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(ng).
		WithStatusSubresource(ng).
		Build()

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "test-ng"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &deckhousev1.NodeGroup{}
	err = cl.Get(context.Background(), client.ObjectKey{Name: "test-ng"}, updated)
	require.NoError(t, err)

	assert.Equal(t, int32(0), updated.Status.Nodes, "nodes count should be 0")
	assert.Equal(t, int32(0), updated.Status.Ready, "ready count should be 0")
	assert.Equal(t, int32(0), updated.Status.UpToDate, "upToDate count should be 0")
	assert.Equal(t, int32(0), updated.Status.Min, "min should be 0 for Static")
	assert.Equal(t, int32(0), updated.Status.Max, "max should be 0 for Static")
	assert.Equal(t, int32(0), updated.Status.Desired, "desired should be 0 for Static")
	assert.Equal(t, int32(0), updated.Status.Instances, "instances should be 0 for Static")
	assert.NotNil(t, updated.Status.ConditionSummary)
	assert.Equal(t, "True", updated.Status.ConditionSummary.Ready)
}

func TestAI_StatusWithMixedReadyAndNotReadyNodes(t *testing.T) {
	s := newTestScheme()

	configChecksum := "abc123"

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ng",
			Annotations: map[string]string{
				"node.deckhouse.io/configuration-checksum": configChecksum,
			},
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
		},
	}

	nodeReady1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-ready-1",
			Labels: map[string]string{"node.deckhouse.io/group": "test-ng"},
			Annotations: map[string]string{
				"node.deckhouse.io/configuration-checksum": configChecksum,
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	nodeReady2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-ready-2",
			Labels: map[string]string{"node.deckhouse.io/group": "test-ng"},
			Annotations: map[string]string{
				"node.deckhouse.io/configuration-checksum": configChecksum,
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	nodeNotReady := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-not-ready",
			Labels: map[string]string{"node.deckhouse.io/group": "test-ng"},
			Annotations: map[string]string{
				"node.deckhouse.io/configuration-checksum": "different-checksum",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(ng, nodeReady1, nodeReady2, nodeNotReady).
		WithStatusSubresource(ng).
		Build()

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "test-ng"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &deckhousev1.NodeGroup{}
	err = cl.Get(context.Background(), client.ObjectKey{Name: "test-ng"}, updated)
	require.NoError(t, err)

	assert.Equal(t, int32(3), updated.Status.Nodes, "total nodes should be 3")
	assert.Equal(t, int32(2), updated.Status.Ready, "ready nodes should be 2")
	assert.Equal(t, int32(2), updated.Status.UpToDate, "upToDate should be 2 (matching checksum)")
}

func TestAI_StatusWithMachineDeployment(t *testing.T) {
	s := newTestScheme()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cloud-ng",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: 2,
				MaxPerZone: 5,
				Zones:      []string{"zone-a", "zone-b"},
				ClassReference: deckhousev1.ClassReference{
					Kind: "TestInstanceClass",
					Name: "test",
				},
			},
		},
	}

	md := &mcmv1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "md-cloud-ng-zone-a",
			Namespace: "d8-cloud-instance-manager",
			Labels:    map[string]string{"node-group": "cloud-ng"},
		},
		Spec: mcmv1alpha1.MachineDeploymentSpec{
			Replicas: 3,
		},
	}

	md2 := &mcmv1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "md-cloud-ng-zone-b",
			Namespace: "d8-cloud-instance-manager",
			Labels:    map[string]string{"node-group": "cloud-ng"},
		},
		Spec: mcmv1alpha1.MachineDeploymentSpec{
			Replicas: 2,
		},
	}

	nodeReady := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-cloud-1",
			Labels: map[string]string{"node.deckhouse.io/group": "cloud-ng"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	instance1 := &deckhousev1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "instance-1",
			Labels: map[string]string{"node.deckhouse.io/group": "cloud-ng"},
		},
	}

	instance2 := &deckhousev1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "instance-2",
			Labels: map[string]string{"node.deckhouse.io/group": "cloud-ng"},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(ng, md, md2, nodeReady, instance1, instance2).
		WithStatusSubresource(ng).
		Build()

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "cloud-ng"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &deckhousev1.NodeGroup{}
	err = cl.Get(context.Background(), client.ObjectKey{Name: "cloud-ng"}, updated)
	require.NoError(t, err)

	assert.Equal(t, int32(1), updated.Status.Nodes, "total nodes should be 1")
	assert.Equal(t, int32(1), updated.Status.Ready, "ready nodes should be 1")
	assert.Equal(t, int32(5), updated.Status.Desired, "desired = sum of MD replicas = 3+2 = 5")
	assert.Equal(t, int32(4), updated.Status.Min, "min = minPerZone * zonesCount = 2*2 = 4")
	assert.Equal(t, int32(10), updated.Status.Max, "max = maxPerZone * zonesCount = 5*2 = 10")
	assert.Equal(t, int32(2), updated.Status.Instances, "instances should be 2")
	assert.NotNil(t, updated.Status.ConditionSummary)
	assert.Equal(t, "True", updated.Status.ConditionSummary.Ready)
}

func TestAI_StatusNodeGroupNotFound(t *testing.T) {
	s := newTestScheme()

	cl := fake.NewClientBuilder().
		WithScheme(s).
		Build()

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "nonexistent"},
	})
	require.NoError(t, err, "should not error on missing NodeGroup (IgnoreNotFound)")
	assert.Equal(t, ctrl.Result{}, result)
}

func TestAI_StatusCloudEphemeralWithFailedMachines(t *testing.T) {
	s := newTestScheme()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "failed-ng",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 3,
				ClassReference: deckhousev1.ClassReference{
					Kind: "TestInstanceClass",
					Name: "test",
				},
			},
		},
	}

	failTime := metav1.Now()
	md := &mcmv1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "md-failed-ng",
			Namespace: "d8-cloud-instance-manager",
			Labels:    map[string]string{"node-group": "failed-ng"},
		},
		Spec: mcmv1alpha1.MachineDeploymentSpec{
			Replicas: 2,
		},
		Status: mcmv1alpha1.MachineDeploymentStatus{
			FailedMachines: []*mcmv1alpha1.MachineSummary{
				{
					Name:     "machine-failed-1",
					OwnerRef: "owner-ref-1",
					LastOperation: mcmv1alpha1.MachineLastOperation{
						Description:    "Image not found",
						LastUpdateTime: failTime,
						State:          mcmv1alpha1.MachineStateFailed,
						Type:           mcmv1alpha1.MachineOperationCreate,
					},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(ng, md).
		WithStatusSubresource(ng).
		Build()

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "failed-ng"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &deckhousev1.NodeGroup{}
	err = cl.Get(context.Background(), client.ObjectKey{Name: "failed-ng"}, updated)
	require.NoError(t, err)

	assert.Len(t, updated.Status.LastMachineFailures, 1)
	assert.Equal(t, "machine-failed-1", updated.Status.LastMachineFailures[0].Name)
	assert.Equal(t, "owner-ref-1", updated.Status.LastMachineFailures[0].OwnerRef)
	assert.NotNil(t, updated.Status.ConditionSummary)
	assert.Equal(t, "False", updated.Status.ConditionSummary.Ready)
	assert.Contains(t, updated.Status.ConditionSummary.StatusMessage, "Machine creation failed")
	assert.Equal(t, int32(2), updated.Status.Desired, "desired = max(md replicas sum, min) = max(2,1) = 2")
	assert.Equal(t, int32(1), updated.Status.Min, "min = minPerZone * 1 zone = 1")
	assert.Equal(t, int32(3), updated.Status.Max, "max = maxPerZone * 1 zone = 3")
}
