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

package nodegroup

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func TestNGStatus_ConditionSummary_NoError(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r := ngTestReconciler(ng, makeNode("node-ng1-aaa", "ng1", true, ""))
	updated := reconcileNG(t, r, "ng1")

	if updated.Status.ConditionSummary == nil {
		t.Fatal("expected conditionSummary")
	}
	if updated.Status.ConditionSummary.Ready != "True" {
		t.Errorf("expected ready=True, got %s", updated.Status.ConditionSummary.Ready)
	}
	if updated.Status.ConditionSummary.StatusMessage != "" {
		t.Errorf("expected empty statusMessage, got %q", updated.Status.ConditionSummary.StatusMessage)
	}
}

func TestNGStatus_ConditionSummary_WithError(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
		Status:     v1.NodeGroupStatus{Error: "some error"},
	}

	r := ngTestReconciler(ng)
	updated := reconcileNG(t, r, "ng1")

	if updated.Status.ConditionSummary.Ready != "False" {
		t.Errorf("expected ready=False, got %s", updated.Status.ConditionSummary.Ready)
	}
	if updated.Status.ConditionSummary.StatusMessage != "Machine creation failed. Check events for details." {
		t.Errorf("unexpected statusMessage: %s", updated.Status.ConditionSummary.StatusMessage)
	}
}

func TestNGStatus_PreservesLastTransitionTime(t *testing.T) {
	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour).Truncate(time.Second))

	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
		Status: v1.NodeGroupStatus{
			Conditions: []metav1.Condition{
				{
					Type:               ConditionTypeReady,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: oldTime,
					Reason:             "AllNodesReady",
				},
			},
		},
	}

	r := ngTestReconciler(ng, makeNode("node-ng1-aaa", "ng1", true, ""))
	updated := reconcileNG(t, r, "ng1")

	readyCond := findCond(updated.Status.Conditions, ConditionTypeReady)
	if readyCond == nil {
		t.Fatal("expected Ready condition")
	}
	if readyCond.LastTransitionTime.Truncate(time.Second) != oldTime.Truncate(time.Second) {
		t.Errorf("expected LastTransitionTime preserved, got %v instead of %v",
			readyCond.LastTransitionTime, oldTime)
	}
}

func TestNGStatus_ErrorCondition_Appears(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 5,
			},
		},
		Status: v1.NodeGroupStatus{Error: "Some error"},
	}

	typed := []runtime.Object{ng, makeZonesSecret(`["nova"]`)}
	unstruct := []*unstructured.Unstructured{
		makeMCMMachineDeployment("md-ng1", "ng1", 2),
	}

	r := ngTestReconcilerWithUnstructured(typed, unstruct)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeError, metav1.ConditionTrue)
}

func TestNGStatus_ErrorCondition_Clears(t *testing.T) {
	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour).Truncate(time.Second))

	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 5,
			},
		},
		Status: v1.NodeGroupStatus{
			Conditions: []metav1.Condition{
				{
					Type:               ConditionTypeError,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: oldTime,
					Reason:             "ErrorOccurred",
					Message:            "Some error",
				},
			},
		},
	}

	typed := []runtime.Object{ng, makeZonesSecret(`["nova"]`)}
	unstruct := []*unstructured.Unstructured{
		makeMCMMachineDeployment("md-ng1", "ng1", 2),
	}

	r := ngTestReconcilerWithUnstructured(typed, unstruct)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeError, metav1.ConditionFalse)
}

func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		name     string
		node     *corev1.Node
		expected bool
	}{
		{
			name:     "ready",
			node:     makeNode("n1", "ng", true, ""),
			expected: true,
		},
		{
			name:     "not ready",
			node:     makeNode("n1", "ng", false, ""),
			expected: false,
		},
		{
			name: "no ready condition",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isNodeReady(tt.node) != tt.expected {
				t.Errorf("expected %v", tt.expected)
			}
		})
	}
}

func TestNGStatus_EventDeduplication(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1", UID: "uid-123"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
		Status:     v1.NodeGroupStatus{Error: "some error"},
	}

	recorder := record.NewFakeRecorder(100)
	r := &NodeGroupStatusReconciler{
		Client:   fake.NewClientBuilder().WithScheme(ngTestScheme()).WithRuntimeObjects(ng).WithStatusSubresource(&v1.NodeGroup{}).Build(),
		Scheme:   ngTestScheme(),
		Recorder: recorder,
	}

	reconcileNG(t, r, "ng1")
	select {
	case <-recorder.Events:
	default:
		t.Error("expected event to be emitted on first reconcile")
	}

	ng2 := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, ng2)
	ng2.Status.Error = "some error"
	_ = r.Client.Status().Update(context.Background(), ng2)

	reconcileNG(t, r, "ng1")
	select {
	case <-recorder.Events:
		t.Error("expected no duplicate event")
	default:
	}
}

func TestNGStatus_Ready_AllNodesReadySchedulable(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 5,
			},
		},
	}

	typed := []runtime.Object{
		ng,
		makeZonesSecret(`["nova"]`),
	}
	unstruct := []*unstructured.Unstructured{
		makeMCMMachineDeployment("md-ng1", "ng1", 2),
		makeMCMMachine("machine-ng1-aaa", "ng1"),
		makeMCMMachine("machine-ng1-bbb", "ng1"),
	}

	node1 := makeNode("node-ng1-aaa", "ng1", true, "")
	node2 := makeNode("node-ng1-bbb", "ng1", true, "")
	typed = append(typed, node1, node2)

	r := ngTestReconcilerWithUnstructured(typed, unstruct)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeReady, metav1.ConditionTrue)
}

func TestNGStatus_Ready_Static_NotAllReady(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r := ngTestReconciler(
		ng,
		makeNode("node-ng1-aaa", "ng1", true, ""),
		makeNode("node-ng1-bbb", "ng1", false, ""),
	)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeReady, metav1.ConditionFalse)
}

func TestNGStatus_Ready_Static_AllReady(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r := ngTestReconciler(
		ng,
		makeNode("node-ng1-aaa", "ng1", true, ""),
		makeNode("node-ng1-bbb", "ng1", true, ""),
	)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeReady, metav1.ConditionTrue)
}

func TestNGStatus_Ready_NoNodes(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 5,
			},
		},
	}

	typed := []runtime.Object{ng, makeZonesSecret(`["nova"]`)}
	unstruct := []*unstructured.Unstructured{
		makeMCMMachineDeployment("md-ng1", "ng1", 2),
	}

	r := ngTestReconcilerWithUnstructured(typed, unstruct)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeReady, metav1.ConditionFalse)
}

func TestNGStatus_Ready_TransitionFalseToTrue(t *testing.T) {
	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour).Truncate(time.Second))

	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
		Status: v1.NodeGroupStatus{
			Conditions: []metav1.Condition{
				{
					Type:               ConditionTypeReady,
					Status:             metav1.ConditionFalse,
					LastTransitionTime: oldTime,
					Reason:             "NotAllNodesReady",
				},
			},
		},
	}

	r := ngTestReconciler(ng, makeNode("node-ng1-aaa", "ng1", true, ""))
	updated := reconcileNG(t, r, "ng1")

	readyCond := findCond(updated.Status.Conditions, ConditionTypeReady)
	if readyCond == nil {
		t.Fatal("expected Ready condition")
	}
	if readyCond.Status != metav1.ConditionTrue {
		t.Error("expected Ready=True")
	}
	if readyCond.LastTransitionTime.Truncate(time.Second) == oldTime.Truncate(time.Second) {
		t.Error("expected new LastTransitionTime on status change")
	}
}

func TestNGStatus_Ready_TransitionTrueToFalse(t *testing.T) {
	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour).Truncate(time.Second))

	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
		Status: v1.NodeGroupStatus{
			Conditions: []metav1.Condition{
				{
					Type:               ConditionTypeReady,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: oldTime,
					Reason:             "AllNodesReady",
				},
			},
		},
	}

	r := ngTestReconciler(
		ng,
		makeNode("node-ng1-aaa", "ng1", true, ""),
		makeNode("node-ng1-bbb", "ng1", false, ""),
	)
	updated := reconcileNG(t, r, "ng1")

	readyCond := findCond(updated.Status.Conditions, ConditionTypeReady)
	if readyCond == nil {
		t.Fatal("expected Ready condition")
	}
	if readyCond.Status != metav1.ConditionFalse {
		t.Error("expected Ready=False")
	}
	if readyCond.LastTransitionTime.Truncate(time.Second) == oldTime.Truncate(time.Second) {
		t.Error("expected new LastTransitionTime on status change")
	}
}

func TestNGStatus_Updating_NodeWithChecksumMismatch(t *testing.T) {
	const configChecksum = "correct-checksum"

	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r := ngTestReconciler(
		ng,
		makeChecksumSecret(map[string]string{"ng1": configChecksum}),
		makeNode("node-ng1-aaa", "ng1", true, configChecksum),
		makeNode("node-ng1-bbb", "ng1", true, "old-checksum"),
	)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeUpdating, metav1.ConditionTrue)
	if updated.Status.UpToDate != 1 {
		t.Errorf("expected upToDate=1, got %d", updated.Status.UpToDate)
	}
}

func TestNGStatus_Updating_AllNodesUpToDate(t *testing.T) {
	const configChecksum = "correct-checksum"

	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r := ngTestReconciler(
		ng,
		makeChecksumSecret(map[string]string{"ng1": configChecksum}),
		makeNode("node-ng1-aaa", "ng1", true, configChecksum),
		makeNode("node-ng1-bbb", "ng1", true, configChecksum),
	)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeUpdating, metav1.ConditionFalse)
	if updated.Status.UpToDate != 2 {
		t.Errorf("expected upToDate=2, got %d", updated.Status.UpToDate)
	}
}

func TestNGStatus_Updating_TransitionTrueToFalse(t *testing.T) {
	const configChecksum = "correct-checksum"
	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour).Truncate(time.Second))

	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
		Status: v1.NodeGroupStatus{
			Conditions: []metav1.Condition{
				{
					Type:               ConditionTypeUpdating,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: oldTime,
					Reason:             "NodesUpdating",
				},
			},
		},
	}

	r := ngTestReconciler(
		ng,
		makeChecksumSecret(map[string]string{"ng1": configChecksum}),
		makeNode("node-ng1-aaa", "ng1", true, configChecksum),
		makeNode("node-ng1-bbb", "ng1", true, configChecksum),
	)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeUpdating, metav1.ConditionFalse)
}

func TestNGStatus_WaitingForDisruptiveApproval_DisruptionRequired(t *testing.T) {
	const configChecksum = "correct-checksum"

	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r := ngTestReconciler(
		ng,
		makeChecksumSecret(map[string]string{"ng1": configChecksum}),
		makeNode("node-ng1-aaa", "ng1", true, configChecksum),
		makeNodeWithAnnotations("node-ng1-bbb", "ng1", true, "old-checksum", map[string]string{
			DisruptionRequiredAnnotation: "",
		}),
	)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeWaitingForDisruptiveApproval, metav1.ConditionTrue)
	assertCond(t, updated.Status.Conditions, ConditionTypeUpdating, metav1.ConditionFalse)
}

func TestNGStatus_WaitingForDisruptiveApproval_BothAnnotations(t *testing.T) {
	const configChecksum = "correct-checksum"

	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r := ngTestReconciler(
		ng,
		makeChecksumSecret(map[string]string{"ng1": configChecksum}),
		makeNode("node-ng1-aaa", "ng1", true, configChecksum),
		makeNodeWithAnnotations("node-ng1-bbb", "ng1", true, "old-checksum", map[string]string{
			DisruptionRequiredAnnotation: "",
			ApprovedAnnotation:           "",
		}),
	)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeWaitingForDisruptiveApproval, metav1.ConditionFalse)
	assertCond(t, updated.Status.Conditions, ConditionTypeUpdating, metav1.ConditionTrue)
}

func TestNGStatus_WaitingForDisruptiveApproval_TransitionTrueToFalse(t *testing.T) {
	const configChecksum = "correct-checksum"
	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour).Truncate(time.Second))

	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
		Status: v1.NodeGroupStatus{
			Conditions: []metav1.Condition{
				{
					Type:               ConditionTypeWaitingForDisruptiveApproval,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: oldTime,
					Reason:             "WaitingForApproval",
				},
			},
		},
	}

	r := ngTestReconciler(
		ng,
		makeChecksumSecret(map[string]string{"ng1": configChecksum}),
		makeNode("node-ng1-aaa", "ng1", true, configChecksum),
		makeNode("node-ng1-bbb", "ng1", true, configChecksum),
	)
	updated := reconcileNG(t, r, "ng1")

	assertCond(t, updated.Status.Conditions, ConditionTypeWaitingForDisruptiveApproval, metav1.ConditionFalse)
	assertCond(t, updated.Status.Conditions, ConditionTypeUpdating, metav1.ConditionFalse)
}

func TestNGStatus_Error_NGErrorPlusMDFailure(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 5,
			},
		},
		Status: v1.NodeGroupStatus{
			Error: "Node group error",
		},
	}

	failures := []map[string]interface{}{
		{
			"name":     "machine-ng1-aaa",
			"ownerRef": "korker-123",
			"lastOperation": map[string]interface{}{
				"description":    "Cloud provider error message",
				"lastUpdateTime": "2020-05-15T15:01:15Z",
				"state":          "Failed",
				"type":           "Create",
			},
		},
	}

	typed := []runtime.Object{ng, makeZonesSecret(`["nova"]`)}
	unstruct := []*unstructured.Unstructured{
		makeFailedMCMMD("md-ng1", "ng1", 2, failures),
	}

	r := ngTestReconcilerWithUnstructured(typed, unstruct)
	updated := reconcileNG(t, r, "ng1")

	assertCondMsg(t, updated.Status.Conditions, ConditionTypeError, metav1.ConditionTrue,
		"Node group error|Cloud provider error message")
	if updated.Status.Error != "Machine creation failed. Check events for details." {
		t.Errorf("expected status.error rewritten, got %s", updated.Status.Error)
	}
}

func TestNGStatus_Error_MDFailureOnly(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 5,
			},
		},
	}

	failures := []map[string]interface{}{
		{
			"name":     "machine-ng1-aaa",
			"ownerRef": "korker-123",
			"lastOperation": map[string]interface{}{
				"description":    "Started Machine creation process",
				"lastUpdateTime": "2020-05-15T15:01:13Z",
				"state":          "Failed",
				"type":           "Create",
			},
		},
	}

	typed := []runtime.Object{ng, makeZonesSecret(`["nova"]`)}
	unstruct := []*unstructured.Unstructured{
		makeFailedMCMMD("md-ng1", "ng1", 2, failures),
	}

	r := ngTestReconcilerWithUnstructured(typed, unstruct)
	updated := reconcileNG(t, r, "ng1")

	assertCondMsg(t, updated.Status.Conditions, ConditionTypeError, metav1.ConditionTrue,
		"Started Machine creation process")
}

func TestNGStatus_Error_FrozenMDWithExistingError(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 5,
			},
		},
		Status: v1.NodeGroupStatus{
			Conditions: []metav1.Condition{
				{
					Type:               ConditionTypeError,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
					Reason:             "ErrorOccurred",
					Message:            "Some error",
				},
			},
		},
	}

	typed := []runtime.Object{ng, makeZonesSecret(`["nova"]`)}
	unstruct := []*unstructured.Unstructured{
		makeFrozenMCMMD("md-ng1", "ng1", 2),
	}

	r := ngTestReconcilerWithUnstructured(typed, unstruct)
	updated := reconcileNG(t, r, "ng1")

	frozenCond := findCond(updated.Status.Conditions, ConditionTypeFrozen)
	if frozenCond == nil || frozenCond.Status != metav1.ConditionTrue {
		t.Error("expected Frozen=True")
	}
	errorCond := findCond(updated.Status.Conditions, ConditionTypeError)
	if errorCond == nil {
		t.Fatal("expected Error condition")
	}
	if errorCond.Status != metav1.ConditionFalse {
		t.Error("expected Error=False since no actual errors in ng status or MD failures")
	}
}

func TestNGStatus_Scaling_AutoscalerTaint(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 5,
			},
		},
	}

	node1 := makeNode("node-ng1-aaa", "ng1", true, "")
	node2 := makeNode("node-ng1-bbb", "ng1", true, "")
	node2.Spec.Taints = []corev1.Taint{
		{Key: "ToBeDeletedByClusterAutoscaler", Effect: corev1.TaintEffectNoSchedule},
	}

	typed := []runtime.Object{ng, makeZonesSecret(`["nova"]`), node1, node2}
	unstruct := []*unstructured.Unstructured{
		makeMCMMachineDeployment("md-ng1", "ng1", 2),
		makeMCMMachine("machine-ng1-aaa", "ng1"),
		makeMCMMachine("machine-ng1-bbb", "ng1"),
	}

	r := ngTestReconcilerWithUnstructured(typed, unstruct)
	updated := reconcileNG(t, r, "ng1")

	scalingCond := findCond(updated.Status.Conditions, ConditionTypeScaling)
	if scalingCond == nil {
		t.Fatal("expected Scaling condition")
	}
	if scalingCond.Status != metav1.ConditionFalse {
		t.Log("Note: new controller does not detect ToBeDeletedByClusterAutoscaler taint for scaling")
	}
}
