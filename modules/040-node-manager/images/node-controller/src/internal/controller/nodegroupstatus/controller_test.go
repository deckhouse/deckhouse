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

package nodegroupstatus

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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	return scheme
}

func newTestReconciler(objs ...runtime.Object) *NodeGroupStatusReconciler {
	scheme := newTestScheme()
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		WithStatusSubresource(&v1.NodeGroup{}).
		Build()

	return &NodeGroupStatusReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(100),
	}
}

// TestReconcile_EmptyCluster tests that reconciler handles missing NodeGroup gracefully.
func TestReconcile_EmptyCluster(t *testing.T) {
	r := newTestReconciler()

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Requeue {
		t.Error("expected no requeue")
	}
}

// TestReconcile_CloudEphemeral_NoNodes tests CloudEphemeral NodeGroup with no nodes.
func TestReconcile_CloudEphemeral_NoNodes(t *testing.T) {
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

	// Cloud provider secret with zones
	zonesSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CloudProviderSecretName,
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			"zones": []byte(`["nova"]`),
		},
	}

	r := newTestReconciler(ng, zonesSecret)
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Verify status
	updatedNG := &v1.NodeGroup{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG); err != nil {
		t.Fatalf("failed to get nodegroup: %v", err)
	}

	if updatedNG.Status.Min != 1 {
		t.Errorf("expected min=1, got %d", updatedNG.Status.Min)
	}
	if updatedNG.Status.Max != 5 {
		t.Errorf("expected max=5, got %d", updatedNG.Status.Max)
	}
	if updatedNG.Status.Nodes != 0 {
		t.Errorf("expected nodes=0, got %d", updatedNG.Status.Nodes)
	}
	if updatedNG.Status.Ready != 0 {
		t.Errorf("expected ready=0, got %d", updatedNG.Status.Ready)
	}

	// Check Ready condition is False
	readyCond := findCondition(updatedNG.Status.Conditions, ConditionTypeReady)
	if readyCond == nil {
		t.Fatal("expected Ready condition")
	}
	if readyCond.Status != metav1.ConditionFalse {
		t.Errorf("expected Ready=False, got %s", readyCond.Status)
	}
}

// TestReconcile_CloudEphemeral_WithNodes tests CloudEphemeral NodeGroup with nodes.
func TestReconcile_CloudEphemeral_WithNodes(t *testing.T) {
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

	zonesSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: CloudProviderSecretName, Namespace: "kube-system"},
		Data:       map[string][]byte{"zones": []byte(`["nova"]`)},
	}

	checksumSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: ConfigurationChecksumsSecretName, Namespace: MachineNamespace},
		Data:       map[string][]byte{"ng1": []byte("checksum123")},
	}

	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-ng1-aaa",
			Labels:      map[string]string{NodeGroupLabel: "ng1"},
			Annotations: map[string]string{ConfigurationChecksumAnnotation: "checksum123"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-ng1-bbb",
			Labels:      map[string]string{NodeGroupLabel: "ng1"},
			Annotations: map[string]string{ConfigurationChecksumAnnotation: "checksum123"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	r := newTestReconciler(ng, zonesSecret, checksumSecret, node1, node2)
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	updatedNG := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG)

	if updatedNG.Status.Nodes != 2 {
		t.Errorf("expected nodes=2, got %d", updatedNG.Status.Nodes)
	}
	if updatedNG.Status.Ready != 2 {
		t.Errorf("expected ready=2, got %d", updatedNG.Status.Ready)
	}
	if updatedNG.Status.UpToDate != 2 {
		t.Errorf("expected upToDate=2, got %d", updatedNG.Status.UpToDate)
	}

	// With 2 ready nodes and desired=2 (from min), Ready should be True
	readyCond := findCondition(updatedNG.Status.Conditions, ConditionTypeReady)
	if readyCond == nil || readyCond.Status != metav1.ConditionTrue {
		t.Errorf("expected Ready=True")
	}
}

// TestReconcile_Static_AllNodesReady tests Static NodeGroup with all nodes ready.
func TestReconcile_Static_AllNodesReady(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-ng1-aaa",
			Labels: map[string]string{NodeGroupLabel: "ng1"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-ng1-bbb",
			Labels: map[string]string{NodeGroupLabel: "ng1"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	r := newTestReconciler(ng, node1, node2)
	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})

	updatedNG := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG)

	if updatedNG.Status.Nodes != 2 {
		t.Errorf("expected nodes=2, got %d", updatedNG.Status.Nodes)
	}
	if updatedNG.Status.Ready != 2 {
		t.Errorf("expected ready=2, got %d", updatedNG.Status.Ready)
	}

	readyCond := findCondition(updatedNG.Status.Conditions, ConditionTypeReady)
	if readyCond == nil || readyCond.Status != metav1.ConditionTrue {
		t.Error("expected Ready=True for Static NG with all nodes ready")
	}

	// Static NG should not have Scaling condition
	scalingCond := findCondition(updatedNG.Status.Conditions, ConditionTypeScaling)
	if scalingCond != nil {
		t.Error("Static NG should not have Scaling condition")
	}
}

// TestReconcile_Static_SomeNodesNotReady tests Static NodeGroup with some nodes not ready.
func TestReconcile_Static_SomeNodesNotReady(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-ng1-aaa",
			Labels: map[string]string{NodeGroupLabel: "ng1"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
			},
		},
	}

	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-ng1-bbb",
			Labels: map[string]string{NodeGroupLabel: "ng1"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	r := newTestReconciler(ng, node1, node2)
	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})

	updatedNG := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG)

	if updatedNG.Status.Ready != 1 {
		t.Errorf("expected ready=1, got %d", updatedNG.Status.Ready)
	}

	readyCond := findCondition(updatedNG.Status.Conditions, ConditionTypeReady)
	if readyCond == nil || readyCond.Status != metav1.ConditionFalse {
		t.Error("expected Ready=False when not all nodes are ready")
	}
}

// TestReconcile_UpdatingCondition tests Updating condition when nodes have outdated checksum.
func TestReconcile_UpdatingCondition(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	checksumSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: ConfigurationChecksumsSecretName, Namespace: MachineNamespace},
		Data:       map[string][]byte{"ng1": []byte("new-checksum")},
	}

	// Node with old checksum (not matching, not waiting for approval)
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-ng1-aaa",
			Labels:      map[string]string{NodeGroupLabel: "ng1"},
			Annotations: map[string]string{ConfigurationChecksumAnnotation: "old-checksum"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	r := newTestReconciler(ng, checksumSecret, node)
	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})

	updatedNG := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG)

	if updatedNG.Status.UpToDate != 0 {
		t.Errorf("expected upToDate=0, got %d", updatedNG.Status.UpToDate)
	}

	updatingCond := findCondition(updatedNG.Status.Conditions, ConditionTypeUpdating)
	if updatingCond == nil || updatingCond.Status != metav1.ConditionTrue {
		t.Error("expected Updating=True when node checksum doesn't match")
	}
}

// TestReconcile_WaitingForDisruptiveApproval tests condition when node needs approval.
func TestReconcile_WaitingForDisruptiveApproval(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	checksumSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: ConfigurationChecksumsSecretName, Namespace: MachineNamespace},
		Data:       map[string][]byte{"ng1": []byte("new-checksum")},
	}

	// Node waiting for disruptive approval
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-ng1-aaa",
			Labels: map[string]string{NodeGroupLabel: "ng1"},
			Annotations: map[string]string{
				ConfigurationChecksumAnnotation: "old-checksum",
				DisruptiveApprovalAnnotation:    "true",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	r := newTestReconciler(ng, checksumSecret, node)
	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})

	updatedNG := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG)

	waitingCond := findCondition(updatedNG.Status.Conditions, ConditionTypeWaitingForDisruptiveApproval)
	if waitingCond == nil || waitingCond.Status != metav1.ConditionTrue {
		t.Error("expected WaitingForDisruptiveApproval=True")
	}

	// Updating should be False since node is waiting for approval, not updating
	updatingCond := findCondition(updatedNG.Status.Conditions, ConditionTypeUpdating)
	if updatingCond == nil || updatingCond.Status != metav1.ConditionFalse {
		t.Error("expected Updating=False when node is waiting for approval")
	}
}

// TestReconcile_ErrorCondition tests Error condition from NodeGroup status.error.
func TestReconcile_ErrorCondition(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
		Status: v1.NodeGroupStatus{
			Error: "Some validation error",
		},
	}

	r := newTestReconciler(ng)
	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})

	updatedNG := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG)

	errorCond := findCondition(updatedNG.Status.Conditions, ConditionTypeError)
	if errorCond == nil || errorCond.Status != metav1.ConditionTrue {
		t.Error("expected Error=True when ng.status.error is set")
	}
	if errorCond.Message != "Machine creation failed. Check events for details." {
		t.Errorf("unexpected error message: %s", errorCond.Message)
	}
}

// TestReconcile_ScalingCondition_ScalingUp tests Scaling condition when instances < desired.
func TestReconcile_ScalingCondition_ScalingUp(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone: 2,
				MaxPerZone: 5,
			},
		},
	}

	zonesSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: CloudProviderSecretName, Namespace: "kube-system"},
		Data:       map[string][]byte{"zones": []byte(`["nova"]`)},
	}

	// Only 1 node, but min is 2
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-ng1-aaa",
			Labels: map[string]string{NodeGroupLabel: "ng1"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	r := newTestReconciler(ng, zonesSecret, node)
	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})

	updatedNG := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG)

	// Desired should be min (2), instances=0 (no machines)
	scalingCond := findCondition(updatedNG.Status.Conditions, ConditionTypeScaling)
	if scalingCond == nil || scalingCond.Status != metav1.ConditionTrue {
		t.Error("expected Scaling=True when instances < desired")
	}
	if scalingCond.Reason != "ScalingUp" {
		t.Errorf("expected reason=ScalingUp, got %s", scalingCond.Reason)
	}
}

// TestReconcile_MultipleZones tests zone calculation from NodeGroup spec.
func TestReconcile_MultipleZones(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 3,
				Zones:      []string{"a", "b", "c"},
			},
		},
	}

	r := newTestReconciler(ng)
	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})

	updatedNG := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG)

	// 3 zones * minPerZone(1) = 3
	if updatedNG.Status.Min != 3 {
		t.Errorf("expected min=3 (3 zones * 1), got %d", updatedNG.Status.Min)
	}
	// 3 zones * maxPerZone(3) = 9
	if updatedNG.Status.Max != 9 {
		t.Errorf("expected max=9 (3 zones * 3), got %d", updatedNG.Status.Max)
	}
}

// TestReconcile_ConditionSummary tests conditionSummary calculation.
func TestReconcile_ConditionSummary(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng1"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-ng1-aaa",
			Labels: map[string]string{NodeGroupLabel: "ng1"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	r := newTestReconciler(ng, node)
	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})

	updatedNG := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG)

	if updatedNG.Status.ConditionSummary == nil {
		t.Fatal("expected conditionSummary to be set")
	}
	if updatedNG.Status.ConditionSummary.Ready != "True" {
		t.Errorf("expected conditionSummary.ready=True, got %s", updatedNG.Status.ConditionSummary.Ready)
	}
}

// TestReconcile_PreservesLastTransitionTime tests that LastTransitionTime is preserved when status doesn't change.
func TestReconcile_PreservesLastTransitionTime(t *testing.T) {
	// Use a fixed time in the past (truncated to seconds to match what's stored)
	oldTime := metav1.NewTime(metav1.Now().Add(-1 * time.Hour).Truncate(time.Second))

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

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-ng1-aaa",
			Labels: map[string]string{NodeGroupLabel: "ng1"},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	r := newTestReconciler(ng, node)
	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})

	updatedNG := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG)

	readyCond := findCondition(updatedNG.Status.Conditions, ConditionTypeReady)
	if readyCond == nil {
		t.Fatal("expected Ready condition")
	}

	// LastTransitionTime should be preserved since status didn't change (still True)
	// Compare truncated to seconds since that's what's stored
	if readyCond.LastTransitionTime.Truncate(time.Second) != oldTime.Truncate(time.Second) {
		t.Errorf("expected LastTransitionTime to be preserved, got %v instead of %v",
			readyCond.LastTransitionTime, oldTime)
	}
}

// TestReconcile_FrozenCondition tests Frozen condition from MachineDeployment.
func TestReconcile_FrozenCondition(t *testing.T) {
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

	zonesSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: CloudProviderSecretName, Namespace: "kube-system"},
		Data:       map[string][]byte{"zones": []byte(`["nova"]`)},
	}

	// Create frozen MachineDeployment
	md := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "machine.sapcloud.io/v1alpha1",
			"kind":       "MachineDeployment",
			"metadata": map[string]interface{}{
				"name":      "md-ng1",
				"namespace": MachineNamespace,
				"labels":    map[string]interface{}{"node-group": "ng1"},
			},
			"spec": map[string]interface{}{
				"replicas": int64(2),
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Frozen",
						"status": "True",
					},
				},
			},
		},
	}

	scheme := newTestScheme()
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(ng, zonesSecret).
		WithStatusSubresource(&v1.NodeGroup{}).
		Build()

	// Add unstructured object separately
	_ = client.Create(context.Background(), md)

	r := &NodeGroupStatusReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(100),
	}

	_, _ = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ng1"},
	})

	updatedNG := &v1.NodeGroup{}
	_ = r.Client.Get(context.Background(), types.NamespacedName{Name: "ng1"}, updatedNG)

	frozenCond := findCondition(updatedNG.Status.Conditions, ConditionTypeFrozen)
	if frozenCond == nil || frozenCond.Status != metav1.ConditionTrue {
		t.Error("expected Frozen=True when MachineDeployment is frozen")
	}
}

// TestIsNodeReady tests the isNodeReady helper function.
func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		name     string
		node     *corev1.Node
		expected bool
	}{
		{
			name: "node is ready",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
				},
			},
			expected: true,
		},
		{
			name: "node is not ready",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
					},
				},
			},
			expected: false,
		},
		{
			name: "node has no ready condition",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
					},
				},
			},
			expected: false,
		},
		{
			name: "node has multiple conditions",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
						{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNodeReady(tt.node)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Helper function to find condition by type
func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
