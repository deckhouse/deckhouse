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

package update

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
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/dynr"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = deckhousev1.AddToScheme(s)
	_ = deckhousev1alpha1.AddToScheme(s)
	return s
}

func newTestReconciler(objs ...client.Object) (*Reconciler, client.Client) {
	scheme := newTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&deckhousev1.NodeGroup{}).
		Build()

	r := &Reconciler{}
	r.Base = dynr.Base{
		Client: c,
		Scheme: scheme,
		Logger: logr.Discard(),
	}

	return r, c
}

func makeNode(name string, annotations map[string]string, labels map[string]string, ready bool) *corev1.Node {
	conditions := []corev1.NodeCondition{}
	if ready {
		conditions = append(conditions, corev1.NodeCondition{
			Type:   corev1.NodeReady,
			Status: corev1.ConditionTrue,
		})
	} else {
		conditions = append(conditions, corev1.NodeCondition{
			Type:   corev1.NodeReady,
			Status: corev1.ConditionFalse,
		})
	}

	if annotations == nil {
		annotations = map[string]string{}
	}
	if labels == nil {
		labels = map[string]string{}
	}

	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
			Labels:      labels,
		},
		Status: corev1.NodeStatus{
			Conditions: conditions,
		},
	}
}

func makeNodeGroup(name string) *deckhousev1.NodeGroup {
	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
		},
	}
}

// TestAI_NodeWithoutUpdateAnnotations verifies that a node without any update annotations
// is a no-op — reconciler does nothing and returns without error.
func TestAI_NodeWithoutUpdateAnnotations(t *testing.T) {
	ng := makeNodeGroup("worker")
	node := makeNode("node-1", nil, map[string]string{nodeGroupLabel: "worker"}, true)

	r, c := newTestReconciler(ng, node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify node is unchanged — no annotations added
	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updated)
	require.NoError(t, err)

	assert.NotContains(t, updated.Annotations, annotationApproved)
	assert.NotContains(t, updated.Annotations, annotationWaitingForApproval)
	assert.NotContains(t, updated.Annotations, annotationDisruptionApproved)
	assert.NotContains(t, updated.Annotations, annotationDraining)
	assert.NotContains(t, updated.Annotations, annotationDrained)
}

// TestAI_NodeWithoutNodeGroupLabel verifies that a node without the node group label
// is skipped entirely.
func TestAI_NodeWithoutNodeGroupLabel(t *testing.T) {
	node := makeNode("node-no-label", nil, nil, true)

	r, _ := newTestReconciler(node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-no-label"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_NodeNotFound verifies that reconciling a non-existent node returns no error.
func TestAI_NodeNotFound(t *testing.T) {
	r, _ := newTestReconciler()

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "non-existent"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_WaitingForApproval_ApprovesNode verifies that a node with waiting-for-approval
// annotation gets approved when conditions are met (ready node, NG ready).
func TestAI_WaitingForApproval_ApprovesNode(t *testing.T) {
	ng := makeNodeGroup("worker")
	ng.Status = deckhousev1.NodeGroupStatus{
		Desired: 1,
		Ready:   1,
		Nodes:   1,
	}

	node := makeNode("node-1",
		map[string]string{
			annotationWaitingForApproval: "",
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)

	r, c := newTestReconciler(ng, node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify node is now approved and waiting-for-approval is removed
	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updated)
	require.NoError(t, err)

	assert.Contains(t, updated.Annotations, annotationApproved, "node should have approved annotation")
	assert.NotContains(t, updated.Annotations, annotationWaitingForApproval, "waiting-for-approval should be removed")
}

// TestAI_WaitingForApproval_NotReadyNode_ApprovesFirst verifies that not-ready nodes
// are preferred for approval over ready nodes.
func TestAI_WaitingForApproval_NotReadyNode_ApprovesFirst(t *testing.T) {
	ng := makeNodeGroup("worker")
	ng.Status = deckhousev1.NodeGroupStatus{
		Desired: 2,
		Ready:   1,
		Nodes:   2,
	}

	notReadyNode := makeNode("node-notready",
		map[string]string{
			annotationWaitingForApproval: "",
		},
		map[string]string{nodeGroupLabel: "worker"},
		false,
	)

	r, c := newTestReconciler(ng, notReadyNode)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-notready"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-notready"}, updated)
	require.NoError(t, err)

	assert.Contains(t, updated.Annotations, annotationApproved, "not-ready node should be approved first")
}

// TestAI_WaitingForApproval_ConcurrencyLimitReached verifies that when the concurrency
// limit is reached, no new nodes are approved.
func TestAI_WaitingForApproval_ConcurrencyLimitReached(t *testing.T) {
	ng := makeNodeGroup("worker")
	// Default concurrency is 1

	// One node already approved
	approvedNode := makeNode("node-approved",
		map[string]string{
			annotationApproved: "",
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)

	// Another node waiting for approval
	waitingNode := makeNode("node-waiting",
		map[string]string{
			annotationWaitingForApproval: "",
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)

	r, c := newTestReconciler(ng, approvedNode, waitingNode)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-waiting"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the waiting node is NOT approved (concurrency=1, already 1 approved)
	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-waiting"}, updated)
	require.NoError(t, err)

	assert.NotContains(t, updated.Annotations, annotationApproved, "node should NOT be approved when concurrency limit reached")
	assert.Contains(t, updated.Annotations, annotationWaitingForApproval, "waiting-for-approval should remain")
}

// TestAI_ApprovedUpToDate_ClearsAnnotations verifies that when a node is approved, ready,
// and its config checksum matches the NodeGroup, all update annotations are cleared.
func TestAI_ApprovedUpToDate_ClearsAnnotations(t *testing.T) {
	const checksum = "abc123"

	ng := makeNodeGroup("worker")
	ng.Annotations = map[string]string{
		annotationConfigChecksum: checksum,
	}

	node := makeNode("node-1",
		map[string]string{
			annotationApproved:           "",
			annotationDisruptionRequired: "",
			annotationConfigChecksum:     checksum,
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)

	r, c := newTestReconciler(ng, node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updated)
	require.NoError(t, err)

	assert.NotContains(t, updated.Annotations, annotationApproved, "approved should be cleared")
	assert.NotContains(t, updated.Annotations, annotationWaitingForApproval, "waiting-for-approval should be cleared")
	assert.NotContains(t, updated.Annotations, annotationDisruptionRequired, "disruption-required should be cleared")
	assert.NotContains(t, updated.Annotations, annotationDisruptionApproved, "disruption-approved should be cleared")
	assert.NotContains(t, updated.Annotations, annotationDrained, "drained should be cleared")
}

// TestAI_ApprovedUpToDate_WasDrained_Uncordons verifies that when a node is marked up-to-date
// and was previously drained, the node is uncordoned (unschedulable set to false).
func TestAI_ApprovedUpToDate_WasDrained_Uncordons(t *testing.T) {
	const checksum = "abc123"

	ng := makeNodeGroup("worker")
	ng.Annotations = map[string]string{
		annotationConfigChecksum: checksum,
	}

	node := makeNode("node-1",
		map[string]string{
			annotationApproved:       "",
			annotationDrained:        drainingSourceBashible,
			annotationConfigChecksum: checksum,
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)
	node.Spec.Unschedulable = true

	r, c := newTestReconciler(ng, node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updated)
	require.NoError(t, err)

	assert.False(t, updated.Spec.Unschedulable, "node should be uncordoned after up-to-date with drained")
	assert.NotContains(t, updated.Annotations, annotationDrained, "drained annotation should be cleared")
}

// TestAI_ApprovedChecksumMismatch_DoesNotClear verifies that when a node is approved but
// config checksum does NOT match the NodeGroup, annotations are NOT cleared.
func TestAI_ApprovedChecksumMismatch_DoesNotClear(t *testing.T) {
	ng := makeNodeGroup("worker")
	ng.Annotations = map[string]string{
		annotationConfigChecksum: "new-checksum",
	}

	node := makeNode("node-1",
		map[string]string{
			annotationApproved:       "",
			annotationConfigChecksum: "old-checksum",
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)

	r, c := newTestReconciler(ng, node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updated)
	require.NoError(t, err)

	// Annotations should still be present since checksums don't match
	assert.Contains(t, updated.Annotations, annotationApproved, "approved should remain when checksum mismatch")
}

// TestAI_DisruptionApproval_ManualMode_DoesNothing verifies that in Manual disruption
// approval mode, the reconciler does not auto-approve disruptions.
func TestAI_DisruptionApproval_ManualMode_DoesNothing(t *testing.T) {
	ng := makeNodeGroup("worker")
	ng.Spec.Disruptions = &deckhousev1.DisruptionsSpec{
		ApprovalMode: deckhousev1.DisruptionApprovalModeManual,
	}

	node := makeNode("node-1",
		map[string]string{
			annotationApproved:           "",
			annotationDisruptionRequired: "",
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)

	r, c := newTestReconciler(ng, node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updated)
	require.NoError(t, err)

	assert.Contains(t, updated.Annotations, annotationDisruptionRequired, "disruption-required should remain in Manual mode")
	assert.NotContains(t, updated.Annotations, annotationDisruptionApproved, "disruption should NOT be auto-approved in Manual mode")
}

// TestAI_DisruptionApproval_AutomaticMode_NoDrain_ApprovesDisruption verifies that in
// Automatic mode with drainBeforeApproval=false, disruption is directly approved.
func TestAI_DisruptionApproval_AutomaticMode_NoDrain_ApprovesDisruption(t *testing.T) {
	drainFalse := false
	ng := makeNodeGroup("worker")
	ng.Spec.Disruptions = &deckhousev1.DisruptionsSpec{
		ApprovalMode: deckhousev1.DisruptionApprovalModeAutomatic,
		Automatic: &deckhousev1.AutomaticDisruptionSpec{
			DrainBeforeApproval: &drainFalse,
		},
	}

	node := makeNode("node-1",
		map[string]string{
			annotationApproved:           "",
			annotationDisruptionRequired: "",
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)

	r, c := newTestReconciler(ng, node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updated)
	require.NoError(t, err)

	assert.Contains(t, updated.Annotations, annotationDisruptionApproved, "disruption should be approved in Automatic mode with no drain")
	assert.NotContains(t, updated.Annotations, annotationDisruptionRequired, "disruption-required should be removed")
}

// TestAI_DisruptionApproval_AutomaticMode_DrainRequired_SetsDrainingAnnotation verifies
// that in Automatic mode with drain needed, the draining annotation is set on the node.
func TestAI_DisruptionApproval_AutomaticMode_DrainRequired_SetsDrainingAnnotation(t *testing.T) {
	ng := makeNodeGroup("worker")
	ng.Spec.Disruptions = &deckhousev1.DisruptionsSpec{
		ApprovalMode: deckhousev1.DisruptionApprovalModeAutomatic,
		// DrainBeforeApproval defaults to true
	}
	ng.Status = deckhousev1.NodeGroupStatus{
		Nodes: 2, // more than 1 so drain is needed
	}

	node := makeNode("node-1",
		map[string]string{
			annotationApproved:           "",
			annotationDisruptionRequired: "",
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)
	// Node is NOT unschedulable, so draining should start
	node.Spec.Unschedulable = false

	r, c := newTestReconciler(ng, node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updated)
	require.NoError(t, err)

	assert.Equal(t, drainingSourceBashible, updated.Annotations[annotationDraining], "draining annotation should be set with bashible source")
}

// TestAI_DisruptionApproval_AutomaticMode_AlreadyDrained_ApprovesDisruption verifies
// that when the node is already drained, disruption is approved directly.
func TestAI_DisruptionApproval_AutomaticMode_AlreadyDrained_ApprovesDisruption(t *testing.T) {
	ng := makeNodeGroup("worker")
	ng.Spec.Disruptions = &deckhousev1.DisruptionsSpec{
		ApprovalMode: deckhousev1.DisruptionApprovalModeAutomatic,
		// DrainBeforeApproval defaults to true
	}

	node := makeNode("node-1",
		map[string]string{
			annotationApproved:           "",
			annotationDisruptionRequired: "",
			annotationDrained:            drainingSourceBashible,
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)

	r, c := newTestReconciler(ng, node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updated)
	require.NoError(t, err)

	assert.Contains(t, updated.Annotations, annotationDisruptionApproved, "disruption should be approved when already drained")
	assert.NotContains(t, updated.Annotations, annotationDisruptionRequired, "disruption-required should be removed")
}

// TestAI_DisruptionApproval_SingleMaster_SkipsDrain verifies that a single-master
// NodeGroup (name=master, nodes=1) does not require drain before disruption approval.
func TestAI_DisruptionApproval_SingleMaster_SkipsDrain(t *testing.T) {
	ng := makeNodeGroup("master")
	ng.Spec.Disruptions = &deckhousev1.DisruptionsSpec{
		ApprovalMode: deckhousev1.DisruptionApprovalModeAutomatic,
		// DrainBeforeApproval defaults to true, but single master skips drain
	}
	ng.Status = deckhousev1.NodeGroupStatus{
		Nodes: 1,
	}

	node := makeNode("master-0",
		map[string]string{
			annotationApproved:           "",
			annotationDisruptionRequired: "",
		},
		map[string]string{nodeGroupLabel: "master"},
		true,
	)

	r, c := newTestReconciler(ng, node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "master-0"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "master-0"}, updated)
	require.NoError(t, err)

	assert.Contains(t, updated.Annotations, annotationDisruptionApproved, "single master should skip drain and approve disruption directly")
	assert.NotContains(t, updated.Annotations, annotationDraining, "single master should NOT be drained")
}

// TestAI_DrainingAnnotation_MarksDrained verifies the annotation flow for draining:
// when a node has draining=bashible annotation, markNodeDrained swaps it to drained=bashible.
func TestAI_DrainingAnnotation_MarksDrained(t *testing.T) {
	node := makeNode("node-1",
		map[string]string{
			annotationDraining: drainingSourceBashible,
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)

	scheme := newTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		Build()

	r := &Reconciler{}
	r.Base = dynr.Base{
		Client: c,
		Scheme: scheme,
		Logger: logr.Discard(),
	}

	err := r.markNodeDrained(context.Background(), node)
	require.NoError(t, err)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updated)
	require.NoError(t, err)

	assert.NotContains(t, updated.Annotations, annotationDraining, "draining annotation should be removed")
	assert.Equal(t, drainingSourceBashible, updated.Annotations[annotationDrained], "drained annotation should be set with same source")
}

// TestAI_RollingUpdate_DeletesInstance verifies that in RollingUpdate disruption mode,
// the reconciler deletes the Instance resource for the node.
func TestAI_RollingUpdate_DeletesInstance(t *testing.T) {
	ng := makeNodeGroup("worker")
	ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
	ng.Spec.Disruptions = &deckhousev1.DisruptionsSpec{
		ApprovalMode: deckhousev1.DisruptionApprovalModeRollingUpdate,
	}

	node := makeNode("node-1",
		map[string]string{
			annotationApproved:      "",
			annotationRollingUpdate: "",
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)

	instance := &deckhousev1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
		},
	}

	r, c := newTestReconciler(ng, node, instance)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify Instance was deleted
	deletedInstance := &deckhousev1alpha1.Instance{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, deletedInstance)
	assert.Error(t, err, "Instance should be deleted in RollingUpdate mode")
}

// TestAI_CalculateConcurrency_Default verifies that default concurrency is 1.
func TestAI_CalculateConcurrency_Default(t *testing.T) {
	ng := &deckhousev1.NodeGroup{}
	assert.Equal(t, 1, calculateConcurrency(ng, 10))
}

// TestAI_CalculateConcurrency_IntValue verifies concurrency from integer MaxConcurrent.
func TestAI_CalculateConcurrency_IntValue(t *testing.T) {
	ng := &deckhousev1.NodeGroup{}
	mc := intstr.FromInt32(3)
	ng.Spec.Update = &deckhousev1.UpdateSpec{MaxConcurrent: &mc}
	assert.Equal(t, 3, calculateConcurrency(ng, 10))
}

// TestAI_CalculateConcurrency_Percent verifies concurrency from percentage MaxConcurrent.
func TestAI_CalculateConcurrency_Percent(t *testing.T) {
	ng := &deckhousev1.NodeGroup{}
	mc := intstr.FromString("50%")
	ng.Spec.Update = &deckhousev1.UpdateSpec{MaxConcurrent: &mc}
	assert.Equal(t, 5, calculateConcurrency(ng, 10))
}

// TestAI_CalculateConcurrency_PercentMinOne verifies that percentage resulting in 0 is raised to 1.
func TestAI_CalculateConcurrency_PercentMinOne(t *testing.T) {
	ng := &deckhousev1.NodeGroup{}
	mc := intstr.FromString("1%")
	ng.Spec.Update = &deckhousev1.UpdateSpec{MaxConcurrent: &mc}
	assert.Equal(t, 1, calculateConcurrency(ng, 1))
}

// TestAI_ExtractNodeInfo_AllAnnotations verifies extractNodeInfo correctly reads all annotations.
func TestAI_ExtractNodeInfo_AllAnnotations(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				annotationApproved:           "",
				annotationWaitingForApproval: "",
				annotationDisruptionRequired: "",
				annotationDisruptionApproved: "",
				annotationRollingUpdate:      "",
				annotationDraining:           drainingSourceBashible,
				annotationDrained:            drainingSourceBashible,
				annotationConfigChecksum:     "checksum-123",
			},
		},
		Spec: corev1.NodeSpec{
			Unschedulable: true,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	info := extractNodeInfo(node)

	assert.True(t, info.isApproved)
	assert.True(t, info.isWaitingForApproval)
	assert.True(t, info.isDisruptionRequired)
	assert.True(t, info.isDisruptionApproved)
	assert.True(t, info.isRollingUpdate)
	assert.True(t, info.isDraining)
	assert.True(t, info.isDrained)
	assert.True(t, info.isUnschedulable)
	assert.True(t, info.isReady)
	assert.Equal(t, "checksum-123", info.configChecksum)
}

// TestAI_ExtractNodeInfo_NilAnnotations verifies extractNodeInfo with nil annotations.
func TestAI_ExtractNodeInfo_NilAnnotations(t *testing.T) {
	node := &corev1.Node{}
	info := extractNodeInfo(node)

	assert.False(t, info.isApproved)
	assert.False(t, info.isWaitingForApproval)
	assert.False(t, info.isDraining)
	assert.False(t, info.isDrained)
	assert.False(t, info.isReady)
	assert.Equal(t, "", info.configChecksum)
}

// TestAI_ExtractNodeInfo_DrainingNonBashible verifies that draining annotation
// from a non-bashible source is not recognized.
func TestAI_ExtractNodeInfo_DrainingNonBashible(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				annotationDraining: "instance-deletion",
			},
		},
	}

	info := extractNodeInfo(node)
	assert.False(t, info.isDraining, "non-bashible draining source should not be recognized")
}

// TestAI_NodeGroupNotFound verifies that when the NodeGroup does not exist, reconciler returns no error.
func TestAI_NodeGroupNotFound(t *testing.T) {
	node := makeNode("node-1", nil, map[string]string{nodeGroupLabel: "nonexistent"}, true)

	r, _ := newTestReconciler(node)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_IsDisruptionWindowAllowed_NoWindows verifies disruptions are always allowed
// when no windows are configured.
func TestAI_IsDisruptionWindowAllowed_NoWindows(t *testing.T) {
	assert.True(t, isDisruptionWindowAllowed(nil, time.Now()))
}

// TestAI_MarkNodeUpToDate verifies the markNodeUpToDate method properly clears all annotations.
func TestAI_MarkNodeUpToDate(t *testing.T) {
	node := makeNode("node-1",
		map[string]string{
			annotationApproved:           "",
			annotationWaitingForApproval: "",
			annotationDisruptionRequired: "",
			annotationDisruptionApproved: "",
			annotationDrained:            drainingSourceBashible,
		},
		map[string]string{nodeGroupLabel: "worker"},
		true,
	)
	node.Spec.Unschedulable = true

	scheme := newTestScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		Build()

	r := &Reconciler{}
	r.Base = dynr.Base{
		Client: c,
		Scheme: scheme,
		Logger: logr.Discard(),
	}

	err := r.markNodeUpToDate(context.Background(), node, true)
	require.NoError(t, err)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-1"}, updated)
	require.NoError(t, err)

	assert.NotContains(t, updated.Annotations, annotationApproved)
	assert.NotContains(t, updated.Annotations, annotationWaitingForApproval)
	assert.NotContains(t, updated.Annotations, annotationDisruptionRequired)
	assert.NotContains(t, updated.Annotations, annotationDisruptionApproved)
	assert.NotContains(t, updated.Annotations, annotationDrained)
	assert.False(t, updated.Spec.Unschedulable, "node should be uncordoned when wasDrained=true")
}

// TestAI_CloudEphemeral_WaitingForApproval_NGNotReady_DoesNotApprove verifies that for
// CloudEphemeral NodeGroup where desired > ready and not all nodes are ready,
// a ready node waiting for approval is NOT approved.
func TestAI_CloudEphemeral_WaitingForApproval_NGNotReady_DoesNotApprove(t *testing.T) {
	ng := makeNodeGroup("cloud-worker")
	ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
	ng.Status = deckhousev1.NodeGroupStatus{
		Desired: 3,
		Ready:   1,
		Nodes:   2,
	}

	// A not-ready node exists in the group so allReady=false
	notReadyNode := makeNode("node-notready",
		nil,
		map[string]string{nodeGroupLabel: "cloud-worker"},
		false,
	)

	readyNode := makeNode("node-ready",
		map[string]string{
			annotationWaitingForApproval: "",
		},
		map[string]string{nodeGroupLabel: "cloud-worker"},
		true,
	)

	r, c := newTestReconciler(ng, notReadyNode, readyNode)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "node-ready"},
	})

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	updated := &corev1.Node{}
	err = c.Get(context.Background(), types.NamespacedName{Name: "node-ready"}, updated)
	require.NoError(t, err)

	assert.NotContains(t, updated.Annotations, annotationApproved, "ready node should NOT be approved when CloudEphemeral NG desired > ready and not all nodes ready")
	assert.Contains(t, updated.Annotations, annotationWaitingForApproval, "waiting-for-approval should remain")
}
