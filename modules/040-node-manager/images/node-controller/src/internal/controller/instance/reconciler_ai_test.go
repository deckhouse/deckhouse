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

package instance

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

func newTestInstanceReconciler(objs ...client.Object) (*InstanceReconciler, client.Client) {
	scheme := newTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(
			&deckhousev1alpha1.Instance{},
			&deckhousev1.NodeGroup{},
		).
		Build()

	r := &InstanceReconciler{}
	r.Base = dynr.Base{
		Client: c,
		Scheme: scheme,
		Logger: logr.Discard(),
	}

	return r, c
}

func makeNodeGroup(name string) *deckhousev1.NodeGroup {
	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  "ng-uid-123",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 5,
				ClassReference: deckhousev1.ClassReference{
					Kind: "YandexInstanceClass",
					Name: "worker",
				},
			},
		},
	}
}

func makeMachine(name string, ngName string, nodeName string) *mcmv1alpha1.Machine {
	now := metav1.NewTime(time.Now())
	return &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: machineNamespace,
			Labels: map[string]string{
				"node": nodeName,
			},
		},
		Spec: mcmv1alpha1.MachineSpec{
			NodeTemplateSpec: mcmv1alpha1.MachineNodeTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						nodeGroupLabel: ngName,
					},
				},
			},
		},
		Status: mcmv1alpha1.MachineStatus{
			CurrentStatus: mcmv1alpha1.MachineCurrentStatus{
				Phase:          mcmv1alpha1.MachinePhaseRunning,
				LastUpdateTime: now,
			},
			LastOperation: mcmv1alpha1.MachineLastOperation{
				Description:    "Machine is running",
				LastUpdateTime: now,
				State:          mcmv1alpha1.MachineStateSuccessful,
				Type:           mcmv1alpha1.MachineOperationCreate,
			},
		},
	}
}

func makeInstance(name string, ngName string, withFinalizer bool) *deckhousev1alpha1.Instance {
	finalizers := []string{}
	if withFinalizer {
		finalizers = append(finalizers, finalizerName)
	}

	return &deckhousev1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				nodeGroupLabel: ngName,
			},
			Finalizers: finalizers,
		},
		Status: deckhousev1alpha1.InstanceStatus{
			NodeRef: deckhousev1alpha1.InstanceNodeRef{
				Name: name,
			},
			MachineRef: deckhousev1alpha1.InstanceMachineRef{
				Kind:       "Machine",
				APIVersion: "machine.sapcloud.io/v1alpha1",
				Name:       name,
				Namespace:  machineNamespace,
			},
			ClassReference: deckhousev1alpha1.InstanceClassReference{
				Kind: "YandexInstanceClass",
				Name: "worker",
			},
		},
	}
}

func makeNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// TestAI_InstanceNotFound_Skip verifies that when neither Instance nor Machine exists,
// reconciler returns no error (skip).
func TestAI_InstanceNotFound_Skip(t *testing.T) {
	r, _ := newTestInstanceReconciler()

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "non-existent"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_InstanceWithMachine_StatusUpdated verifies that when both Instance and Machine exist,
// the Instance status is updated to reflect Machine status.
func TestAI_InstanceWithMachine_StatusUpdated(t *testing.T) {
	ng := makeNodeGroup("ng1")
	machine := makeMachine("worker-1", "ng1", "worker-1")
	instance := makeInstance("worker-1", "ng1", true)
	// Set instance to a stale phase to verify it gets updated
	instance.Status.CurrentStatus.Phase = deckhousev1alpha1.InstancePhasePending

	r, c := newTestInstanceReconciler(ng, machine, instance)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify Instance status was updated
	updated := &deckhousev1alpha1.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, updated)
	require.NoError(t, err)

	assert.Equal(t, deckhousev1alpha1.InstancePhase(mcmv1alpha1.MachinePhaseRunning), updated.Status.CurrentStatus.Phase,
		"Instance phase should match Machine phase")
	assert.Equal(t, "worker-1", updated.Status.NodeRef.Name,
		"Instance nodeRef should match Machine node label")
}

// TestAI_InstanceWithMachine_ClassReferenceUpdated verifies that Instance ClassReference
// is updated from NodeGroup spec.
func TestAI_InstanceWithMachine_ClassReferenceUpdated(t *testing.T) {
	ng := makeNodeGroup("ng1")
	ng.Spec.CloudInstances.ClassReference = deckhousev1.ClassReference{
		Kind: "AWSInstanceClass",
		Name: "new-worker",
	}

	machine := makeMachine("worker-1", "ng1", "worker-1")
	instance := makeInstance("worker-1", "ng1", true)
	// Old class reference
	instance.Status.ClassReference = deckhousev1alpha1.InstanceClassReference{
		Kind: "YandexInstanceClass",
		Name: "old-worker",
	}

	r, c := newTestInstanceReconciler(ng, machine, instance)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &deckhousev1alpha1.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, updated)
	require.NoError(t, err)

	assert.Equal(t, "AWSInstanceClass", updated.Status.ClassReference.Kind)
	assert.Equal(t, "new-worker", updated.Status.ClassReference.Name)
}

// TestAI_MachineExists_NoInstance_CreatesInstance verifies that when a Machine exists
// but no Instance, a new Instance is created.
func TestAI_MachineExists_NoInstance_CreatesInstance(t *testing.T) {
	ng := makeNodeGroup("ng1")
	machine := makeMachine("worker-new", "ng1", "worker-new")

	r, c := newTestInstanceReconciler(ng, machine)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-new"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify Instance was created
	created := &deckhousev1alpha1.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "worker-new"}, created)
	require.NoError(t, err, "Instance should be created")

	assert.Equal(t, "ng1", created.Labels[nodeGroupLabel], "Instance should have nodeGroup label")
	assert.Contains(t, created.Finalizers, finalizerName, "Instance should have finalizer")
	assert.Equal(t, 1, len(created.OwnerReferences), "Instance should have owner reference")
	assert.Equal(t, "ng1", created.OwnerReferences[0].Name, "Owner reference should point to NodeGroup")
}

// TestAI_OrphanedInstance_NoMachine_RemovesFinalizerAndDeletes verifies that when an Instance
// exists but no Machine, the finalizer is removed and Instance is deleted.
func TestAI_OrphanedInstance_NoMachine_RemovesFinalizerAndDeletes(t *testing.T) {
	instance := makeInstance("orphan-1", "ng1", true)

	r, c := newTestInstanceReconciler(instance)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "orphan-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify Instance finalizer was removed and instance deleted
	deleted := &deckhousev1alpha1.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "orphan-1"}, deleted)
	assert.Error(t, err, "orphaned Instance should be deleted")
}

// TestAI_OrphanedInstance_NoFinalizer_Deletes verifies that an orphaned Instance
// without finalizer is also deleted.
func TestAI_OrphanedInstance_NoFinalizer_Deletes(t *testing.T) {
	instance := makeInstance("orphan-nofin", "ng1", false)

	r, c := newTestInstanceReconciler(instance)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "orphan-nofin"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	deleted := &deckhousev1alpha1.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "orphan-nofin"}, deleted)
	assert.Error(t, err, "orphaned Instance without finalizer should be deleted")
}

// TestAI_MachineWithoutNodeGroupLabel_Skips verifies that a Machine without
// the node group label is skipped.
func TestAI_MachineWithoutNodeGroupLabel_Skips(t *testing.T) {
	machine := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-nolabel",
			Namespace: machineNamespace,
		},
		Spec: mcmv1alpha1.MachineSpec{
			NodeTemplateSpec: mcmv1alpha1.MachineNodeTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						// No nodeGroupLabel
					},
				},
			},
		},
	}

	r, _ := newTestInstanceReconciler(machine)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-nolabel"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_MachineNodeGroupNotFound_Skips verifies that when the NodeGroup referenced
// by Machine does not exist, reconciler skips.
func TestAI_MachineNodeGroupNotFound_Skips(t *testing.T) {
	machine := makeMachine("worker-nogroup", "nonexistent-ng", "worker-nogroup")

	r, _ := newTestInstanceReconciler(machine)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-nogroup"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_InstanceWithMachine_NodeRefUpdated verifies that when the Machine's node label
// changes, the Instance nodeRef is updated.
func TestAI_InstanceWithMachine_NodeRefUpdated(t *testing.T) {
	ng := makeNodeGroup("ng1")
	machine := makeMachine("worker-1", "ng1", "new-node-name")
	instance := makeInstance("worker-1", "ng1", true)
	instance.Status.NodeRef.Name = "old-node-name"

	r, c := newTestInstanceReconciler(ng, machine, instance)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &deckhousev1alpha1.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, updated)
	require.NoError(t, err)

	assert.Equal(t, "new-node-name", updated.Status.NodeRef.Name,
		"Instance nodeRef should be updated to match Machine node label")
}

// TestAI_InstanceWithMachine_LastOperationUpdated verifies that the Instance last operation
// is synced from Machine status.
func TestAI_InstanceWithMachine_LastOperationUpdated(t *testing.T) {
	ng := makeNodeGroup("ng1")
	now := metav1.NewTime(time.Now())
	machine := makeMachine("worker-1", "ng1", "worker-1")
	machine.Status.LastOperation = mcmv1alpha1.MachineLastOperation{
		Description:    "Deleting machine from cloud",
		LastUpdateTime: now,
		State:          mcmv1alpha1.MachineStateProcessing,
		Type:           mcmv1alpha1.MachineOperationDelete,
	}

	instance := makeInstance("worker-1", "ng1", true)
	instance.Status.LastOperation = deckhousev1alpha1.InstanceLastOperation{
		Description: "Old operation",
	}

	r, c := newTestInstanceReconciler(ng, machine, instance)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &deckhousev1alpha1.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, updated)
	require.NoError(t, err)

	assert.Equal(t, "Deleting machine from cloud", updated.Status.LastOperation.Description)
	assert.Equal(t, deckhousev1alpha1.InstanceStateProcessing, updated.Status.LastOperation.State)
	assert.Equal(t, deckhousev1alpha1.InstanceOperationDelete, updated.Status.LastOperation.Type)
}

// TestAI_InstanceWithMachine_BootstrapStatusCleared verifies that when Machine phase
// becomes Running, the bootstrap status fields are cleared.
func TestAI_InstanceWithMachine_BootstrapStatusCleared(t *testing.T) {
	ng := makeNodeGroup("ng1")
	machine := makeMachine("worker-1", "ng1", "worker-1")
	machine.Status.CurrentStatus.Phase = mcmv1alpha1.MachinePhaseRunning

	instance := makeInstance("worker-1", "ng1", true)
	instance.Status.CurrentStatus.Phase = deckhousev1alpha1.InstancePhasePending
	instance.Status.BootstrapStatus = deckhousev1alpha1.InstanceBootstrapStatus{
		LogsEndpoint: "https://logs.example.com",
		Description:  "Bootstrapping...",
	}

	r, c := newTestInstanceReconciler(ng, machine, instance)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &deckhousev1alpha1.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, updated)
	require.NoError(t, err)

	assert.Empty(t, updated.Status.BootstrapStatus.LogsEndpoint,
		"bootstrap logs endpoint should be cleared when Running")
	assert.Empty(t, updated.Status.BootstrapStatus.Description,
		"bootstrap description should be cleared when Running")
}

// TestAI_InstanceWithMachine_StatusUnchanged_NoPatch verifies that when Instance status
// already matches Machine status, no patch is applied (idempotent).
func TestAI_InstanceWithMachine_StatusUnchanged_NoPatch(t *testing.T) {
	ng := makeNodeGroup("ng1")
	now := metav1.NewTime(time.Now().Truncate(time.Second))
	machine := makeMachine("worker-1", "ng1", "worker-1")
	machine.Status.CurrentStatus.Phase = mcmv1alpha1.MachinePhaseRunning
	machine.Status.CurrentStatus.LastUpdateTime = now
	machine.Status.LastOperation = mcmv1alpha1.MachineLastOperation{
		Description:    "Machine is running",
		LastUpdateTime: now,
		State:          mcmv1alpha1.MachineStateSuccessful,
		Type:           mcmv1alpha1.MachineOperationCreate,
	}

	instance := makeInstance("worker-1", "ng1", true)
	instance.Status.CurrentStatus.Phase = deckhousev1alpha1.InstancePhaseRunning
	instance.Status.CurrentStatus.LastUpdateTime = now
	instance.Status.NodeRef.Name = "worker-1"
	instance.Status.LastOperation = deckhousev1alpha1.InstanceLastOperation{
		Description:    "Machine is running",
		LastUpdateTime: now,
		State:          deckhousev1alpha1.InstanceStateSuccessful,
		Type:           deckhousev1alpha1.InstanceOperationCreate,
	}
	instance.Status.ClassReference = deckhousev1alpha1.InstanceClassReference{
		Kind: "YandexInstanceClass",
		Name: "worker",
	}

	r, _ := newTestInstanceReconciler(ng, machine, instance)

	// This should succeed without errors — no actual patch needed
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_SetDrainingAnnotation verifies the setDrainingAnnotation method sets the
// correct annotation on a node.
func TestAI_SetDrainingAnnotation(t *testing.T) {
	node := makeNode("worker-1")

	r, c := newTestInstanceReconciler(node)

	err := r.setDrainingAnnotation(context.Background(), "worker-1")
	require.NoError(t, err)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, updated)
	require.NoError(t, err)

	assert.Equal(t, "instance-deletion", updated.Annotations["update.node.deckhouse.io/draining"],
		"draining annotation should be set to instance-deletion")
}

// TestAI_SetDrainingAnnotation_NodeNotFound verifies that setDrainingAnnotation does not
// return error when node does not exist.
func TestAI_SetDrainingAnnotation_NodeNotFound(t *testing.T) {
	r, _ := newTestInstanceReconciler()

	err := r.setDrainingAnnotation(context.Background(), "non-existent")
	require.NoError(t, err, "should not error when node not found")
}

// TestAI_SetDrainingAnnotation_Idempotent verifies that setting draining annotation
// on a node that already has it is a no-op.
func TestAI_SetDrainingAnnotation_Idempotent(t *testing.T) {
	node := makeNode("worker-1")
	node.Annotations = map[string]string{
		"update.node.deckhouse.io/draining": "instance-deletion",
	}

	r, c := newTestInstanceReconciler(node)

	err := r.setDrainingAnnotation(context.Background(), "worker-1")
	require.NoError(t, err)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, updated)
	require.NoError(t, err)

	assert.Equal(t, "instance-deletion", updated.Annotations["update.node.deckhouse.io/draining"])
}

// TestAI_RemoveFinalizer verifies that removeFinalizer correctly removes the finalizer from Instance.
func TestAI_RemoveFinalizer(t *testing.T) {
	instance := makeInstance("worker-1", "ng1", true)

	r, c := newTestInstanceReconciler(instance)

	err := r.removeFinalizer(context.Background(), instance)
	require.NoError(t, err)

	updated := &deckhousev1alpha1.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "worker-1"}, updated)
	require.NoError(t, err)

	assert.NotContains(t, updated.Finalizers, finalizerName, "finalizer should be removed")
}

// TestAI_RemoveFinalizer_NoFinalizer verifies that removeFinalizer is a no-op
// when the finalizer is not present.
func TestAI_RemoveFinalizer_NoFinalizer(t *testing.T) {
	instance := makeInstance("worker-1", "ng1", false)

	r, _ := newTestInstanceReconciler(instance)

	err := r.removeFinalizer(context.Background(), instance)
	require.NoError(t, err, "should not error when finalizer not present")
}

// TestAI_MachineToInstanceMapping verifies the machineToInstance mapping function.
func TestAI_MachineToInstanceMapping(t *testing.T) {
	machine := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-1",
			Namespace: machineNamespace,
		},
	}

	requests := machineToInstance(context.Background(), machine)

	require.Len(t, requests, 1)
	assert.Equal(t, "worker-1", requests[0].Name)
}

// TestAI_NodeToInstanceMapping verifies the nodeToInstance mapping function.
func TestAI_NodeToInstanceMapping(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
		},
	}

	requests := nodeToInstance(context.Background(), node)

	require.Len(t, requests, 1)
	assert.Equal(t, "worker-1", requests[0].Name)
}
