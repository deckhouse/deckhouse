/*
Copyright 2026 Flant JSC

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

package nodetopology

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...runtime.Object) *Controller {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add deckhouse v1 scheme: %v", err)
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		WithStatusSubresource(&v1.NodeTopology{}).
		Build()

	return &Controller{
		Base: register.Base{Client: cl},
	}
}

func doReconcile(t *testing.T, r *Controller, nodeName string) ctrl.Result {
	t.Helper()

	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: nodeName},
	})
	if err != nil {
		t.Fatalf("reconcile %s: %v", nodeName, err)
	}

	return res
}

func getNodeTopology(t *testing.T, r *Controller, name string) *v1.NodeTopology {
	t.Helper()

	nodeTopology := &v1.NodeTopology{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, nodeTopology); err != nil {
		t.Fatalf("get NodeTopology %s: %v", name, err)
	}

	return nodeTopology
}

func nodeTopologyExists(t *testing.T, r *Controller, name string) bool {
	t.Helper()

	nodeTopology := &v1.NodeTopology{}
	err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, nodeTopology)
	if err == nil {
		return true
	}
	if apierrors.IsNotFound(err) {
		return false
	}

	t.Fatalf("get NodeTopology %s: %v", name, err)
	return false
}

func makeNode(name, nodeGroupName string) *corev1.Node {
	labels := map[string]string{}
	if nodeGroupName != "" {
		labels["node.deckhouse.io/group"] = nodeGroupName
	}

	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func makeNodeGroup(name string, topologyManager *v1.TopologyManagerSpec) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.NodeGroupSpec{
			Kubelet: &v1.KubeletSpec{
				TopologyManager: topologyManager,
			},
		},
	}
}

func makeNodeTopologyState(enabled bool, policy, scope string) *v1.NodeTopologyState {
	return &v1.NodeTopologyState{
		TopologyManager: &v1.NodeTopologyManagerState{
			Enabled: &enabled,
			Policy:  policy,
			Scope:   scope,
		},
	}
}

func TestReconcile_NodeNotFound_NoError(t *testing.T) {
	r := newReconciler(t)

	doReconcile(t, r, "missing-node")
}

func TestReconcile_NodeWithoutNodeGroupLabel_Skips(t *testing.T) {
	node := makeNode("node-1", "")
	r := newReconciler(t, node)

	doReconcile(t, r, "node-1")

	if nodeTopologyExists(t, r, "node-1") {
		t.Fatal("expected NodeTopology not to be created")
	}
}

func TestReconcile_NodeGroupNotFound_Skips(t *testing.T) {
	node := makeNode("node-1", "worker")
	r := newReconciler(t, node)

	doReconcile(t, r, "node-1")

	if nodeTopologyExists(t, r, "node-1") {
		t.Fatal("expected NodeTopology not to be created")
	}
}

func TestReconcile_NodeGroupWithoutTopologyManager_CreatesDisabledDesiredState(t *testing.T) {
	node := makeNode("node-1", "worker")
	nodeGroup := makeNodeGroup("worker", nil)
	r := newReconciler(t, node, nodeGroup)

	doReconcile(t, r, "node-1")

	nodeTopology := getNodeTopology(t, r, "node-1")

	if nodeTopology.Status.NodeName != "node-1" {
		t.Fatalf("expected status.nodeName=node-1, got %q", nodeTopology.Status.NodeName)
	}
	if nodeTopology.Status.NodeGroup != "worker" {
		t.Fatalf("expected status.nodeGroup=worker, got %q", nodeTopology.Status.NodeGroup)
	}
	if nodeTopology.Status.Desired == nil {
		t.Fatal("expected status.desired to be set")
	}
	if nodeTopology.Status.Desired.TopologyManager == nil {
		t.Fatal("expected status.desired.topologyManager to be set")
	}
	if nodeTopology.Status.Desired.TopologyManager.Enabled == nil {
		t.Fatal("expected status.desired.topologyManager.enabled to be set")
	}
	if *nodeTopology.Status.Desired.TopologyManager.Enabled {
		t.Fatal("expected topology manager to be disabled")
	}
	if nodeTopology.Status.Desired.TopologyManager.Policy != "" {
		t.Fatalf("expected empty policy, got %q", nodeTopology.Status.Desired.TopologyManager.Policy)
	}
	if nodeTopology.Status.Desired.TopologyManager.Scope != "" {
		t.Fatalf("expected empty scope, got %q", nodeTopology.Status.Desired.TopologyManager.Scope)
	}

	assertInSyncUnknownCondition(t, nodeTopology)
}

func TestReconcile_NodeGroupWithTopologyManager_CreatesDesiredState(t *testing.T) {
	node := makeNode("node-1", "worker")
	nodeGroup := makeNodeGroup("worker", &v1.TopologyManagerSpec{
		Policy: "SingleNumaNode",
		Scope:  "Container",
	})
	r := newReconciler(t, node, nodeGroup)

	doReconcile(t, r, "node-1")

	nodeTopology := getNodeTopology(t, r, "node-1")

	if nodeTopology.Status.NodeName != "node-1" {
		t.Fatalf("expected status.nodeName=node-1, got %q", nodeTopology.Status.NodeName)
	}
	if nodeTopology.Status.NodeGroup != "worker" {
		t.Fatalf("expected status.nodeGroup=worker, got %q", nodeTopology.Status.NodeGroup)
	}
	if nodeTopology.Status.Desired == nil {
		t.Fatal("expected status.desired to be set")
	}
	if nodeTopology.Status.Desired.TopologyManager == nil {
		t.Fatal("expected status.desired.topologyManager to be set")
	}
	if nodeTopology.Status.Desired.TopologyManager.Enabled == nil {
		t.Fatal("expected status.desired.topologyManager.enabled to be set")
	}
	if !*nodeTopology.Status.Desired.TopologyManager.Enabled {
		t.Fatal("expected topology manager to be enabled")
	}
	if nodeTopology.Status.Desired.TopologyManager.Policy != "SingleNumaNode" {
		t.Fatalf("expected policy SingleNumaNode, got %q", nodeTopology.Status.Desired.TopologyManager.Policy)
	}
	if nodeTopology.Status.Desired.TopologyManager.Scope != "Container" {
		t.Fatalf("expected scope Container, got %q", nodeTopology.Status.Desired.TopologyManager.Scope)
	}

	assertInSyncUnknownCondition(t, nodeTopology)
}

func TestReconcile_ExistingNodeTopology_UpdatesStatus(t *testing.T) {
	node := makeNode("node-1", "worker")
	nodeGroup := makeNodeGroup("worker", &v1.TopologyManagerSpec{
		Policy: "SingleNumaNode",
		Scope:  "Container",
	})
	nodeTopology := &v1.NodeTopology{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Status: v1.NodeTopologyStatus{
			NodeName:  "node-1",
			NodeGroup: "old-worker",
		},
	}
	r := newReconciler(t, node, nodeGroup, nodeTopology)

	doReconcile(t, r, "node-1")

	updated := getNodeTopology(t, r, "node-1")

	if updated.Status.NodeGroup != "worker" {
		t.Fatalf("expected status.nodeGroup=worker, got %q", updated.Status.NodeGroup)
	}
	if updated.Status.Desired == nil || updated.Status.Desired.TopologyManager == nil {
		t.Fatal("expected desired topology manager state to be set")
	}
	if updated.Status.Desired.TopologyManager.Policy != "SingleNumaNode" {
		t.Fatalf("expected policy SingleNumaNode, got %q", updated.Status.Desired.TopologyManager.Policy)
	}

	assertInSyncUnknownCondition(t, updated)
}

func TestReconcile_ExistingEffectiveStateInSync(t *testing.T) {
	node := makeNode("node-1", "worker")
	nodeGroup := makeNodeGroup("worker", &v1.TopologyManagerSpec{
		Policy: "SingleNumaNode",
		Scope:  "Container",
	})
	nodeTopology := &v1.NodeTopology{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Status: v1.NodeTopologyStatus{
			NodeName:  "node-1",
			NodeGroup: "worker",
			Effective: makeNodeTopologyState(true, "SingleNumaNode", "Container"),
		},
	}
	r := newReconciler(t, node, nodeGroup, nodeTopology)

	doReconcile(t, r, "node-1")

	updated := getNodeTopology(t, r, "node-1")

	assertCondition(
		t,
		updated.Status.Conditions,
		metav1.ConditionTrue,
		reasonDesiredMatchesEffective,
		messageDesiredMatchesEffective,
	)
}

func TestReconcile_ExistingEffectiveStateOutOfSync(t *testing.T) {
	node := makeNode("node-1", "worker")
	nodeGroup := makeNodeGroup("worker", &v1.TopologyManagerSpec{
		Policy: "SingleNumaNode",
		Scope:  "Container",
	})
	nodeTopology := &v1.NodeTopology{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Status: v1.NodeTopologyStatus{
			NodeName:  "node-1",
			NodeGroup: "worker",
			Effective: makeNodeTopologyState(false, "", ""),
		},
	}
	r := newReconciler(t, node, nodeGroup, nodeTopology)

	doReconcile(t, r, "node-1")

	updated := getNodeTopology(t, r, "node-1")

	assertCondition(
		t,
		updated.Status.Conditions,
		metav1.ConditionFalse,
		reasonDesiredDiffersFromEffective,
		messageDesiredDiffersFromEffective,
	)
}

func assertInSyncUnknownCondition(t *testing.T, nodeTopology *v1.NodeTopology) {
	t.Helper()

	assertCondition(
		t,
		nodeTopology.Status.Conditions,
		metav1.ConditionUnknown,
		reasonEffectiveStateNotCollected,
		messageEffectiveStateNotCollected,
	)
}

func assertCondition(t *testing.T, conditions []metav1.Condition, status metav1.ConditionStatus, reason, message string) {
	t.Helper()

	for _, condition := range conditions {
		if condition.Type != conditionInSync {
			continue
		}

		if condition.Status != status {
			t.Fatalf("expected InSync status %q, got %q", status, condition.Status)
		}
		if condition.Reason != reason {
			t.Fatalf("expected InSync reason %q, got %q", reason, condition.Reason)
		}
		if condition.Message != message {
			t.Fatalf("expected InSync message %q, got %q", message, condition.Message)
		}
		if condition.LastTransitionTime.IsZero() {
			t.Fatal("expected InSync lastTransitionTime to be set")
		}

		return
	}

	t.Fatal("expected InSync condition")
}

func TestNodeGroupToNodes_ReturnsOnlyNodesFromNodeGroup(t *testing.T) {
	node1 := makeNode("node-1", "worker")
	node2 := makeNode("node-2", "worker")
	node3 := makeNode("node-3", "master")
	nodeWithoutGroup := makeNode("node-4", "")

	nodeGroup := makeNodeGroup("worker", nil)

	r := newReconciler(t, node1, node2, node3, nodeWithoutGroup, nodeGroup)

	requests := r.nodeGroupToNodes(context.Background(), nodeGroup)

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}

	got := map[string]bool{}
	for _, request := range requests {
		got[request.Name] = true
	}

	if !got["node-1"] {
		t.Fatal("expected request for node-1")
	}
	if !got["node-2"] {
		t.Fatal("expected request for node-2")
	}
	if got["node-3"] {
		t.Fatal("did not expect request for node-3")
	}
	if got["node-4"] {
		t.Fatal("did not expect request for node-4")
	}
}

func TestSetInSyncCondition_EffectiveStateNotCollected(t *testing.T) {
	desired := makeNodeTopologyState(true, "SingleNumaNode", "Container")

	updated := setInSyncCondition(desired, nil, nil)

	assertCondition(t, updated, metav1.ConditionUnknown, reasonEffectiveStateNotCollected, messageEffectiveStateNotCollected)
}

func TestSetInSyncCondition_DesiredMatchesEffective(t *testing.T) {
	desired := makeNodeTopologyState(true, "SingleNumaNode", "Container")
	effective := makeNodeTopologyState(true, "SingleNumaNode", "Container")

	updated := setInSyncCondition(desired, effective, nil)

	assertCondition(t, updated, metav1.ConditionTrue, reasonDesiredMatchesEffective, messageDesiredMatchesEffective)
}

func TestSetInSyncCondition_DesiredDiffersFromEffective(t *testing.T) {
	desired := makeNodeTopologyState(true, "SingleNumaNode", "Container")
	effective := makeNodeTopologyState(false, "", "")

	updated := setInSyncCondition(desired, effective, nil)

	assertCondition(t, updated, metav1.ConditionFalse, reasonDesiredDiffersFromEffective, messageDesiredDiffersFromEffective)
}

func TestSetInSyncCondition_PreservesLastTransitionTimeWhenConditionUnchanged(t *testing.T) {
	transitionTime := metav1.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	desired := makeNodeTopologyState(true, "SingleNumaNode", "Container")
	effective := makeNodeTopologyState(true, "SingleNumaNode", "Container")

	conditions := []metav1.Condition{
		{
			Type:               conditionInSync,
			Status:             metav1.ConditionTrue,
			Reason:             reasonDesiredMatchesEffective,
			Message:            messageDesiredMatchesEffective,
			LastTransitionTime: transitionTime,
		},
	}

	updated := setInSyncCondition(desired, effective, conditions)

	if len(updated) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(updated))
	}
	if !updated[0].LastTransitionTime.Equal(&transitionTime) {
		t.Fatalf("expected lastTransitionTime to be preserved, got %s", updated[0].LastTransitionTime.String())
	}
}
