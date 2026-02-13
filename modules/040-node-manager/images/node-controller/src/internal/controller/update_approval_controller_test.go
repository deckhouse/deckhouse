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

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func setupTestReconciler(objs ...client.Object) (*Reconciler, client.Client) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&v1.NodeGroup{}).
		Build()

	r := &Reconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(100),
	}

	return r, fakeClient
}

// Helper functions to create test objects

func newNodeGroup(name string, nodeType v1.NodeType, opts ...func(*v1.NodeGroup)) *v1.NodeGroup {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.NodeGroupSpec{
			NodeType: nodeType,
		},
		Status: v1.NodeGroupStatus{
			Desired: 3,
			Ready:   3,
			Nodes:   3,
		},
	}
	for _, opt := range opts {
		opt(ng)
	}
	return ng
}

func withDisruptions(mode string, drainBefore *bool) func(*v1.NodeGroup) {
	return func(ng *v1.NodeGroup) {
		ng.Spec.Disruptions.ApprovalMode = v1.ApprovalMode(mode)
		if drainBefore != nil {
			ng.Spec.Disruptions.Automatic.DrainBeforeApproval = drainBefore
		}
	}
}

func withMaxConcurrent(val intstr.IntOrString) func(*v1.NodeGroup) {
	return func(ng *v1.NodeGroup) {
		ng.Spec.Update.MaxConcurrent = &val
	}
}

func withStatus(desired, ready, nodes int32) func(*v1.NodeGroup) {
	return func(ng *v1.NodeGroup) {
		ng.Status.Desired = desired
		ng.Status.Ready = ready
		ng.Status.Nodes = nodes
	}
}

func newNode(name, ngName string, opts ...func(*corev1.Node)) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				NodeGroupLabel: ngName,
			},
			Annotations: map[string]string{},
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
	for _, opt := range opts {
		opt(node)
	}
	return node
}

func withAnnotation(key, value string) func(*corev1.Node) {
	return func(n *corev1.Node) {
		if n.Annotations == nil {
			n.Annotations = make(map[string]string)
		}
		n.Annotations[key] = value
	}
}

func withChecksum(checksum string) func(*corev1.Node) {
	return func(n *corev1.Node) {
		if n.Annotations == nil {
			n.Annotations = make(map[string]string)
		}
		n.Annotations[ConfigurationChecksumAnnotation] = checksum
	}
}

func withReady(ready bool) func(*corev1.Node) {
	return func(n *corev1.Node) {
		status := corev1.ConditionFalse
		if ready {
			status = corev1.ConditionTrue
		}
		n.Status.Conditions = []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: status},
		}
	}
}

func withUnschedulable(val bool) func(*corev1.Node) {
	return func(n *corev1.Node) {
		n.Spec.Unschedulable = val
	}
}

func newChecksumSecret(checksums map[string]string) *corev1.Secret {
	data := make(map[string][]byte)
	for k, v := range checksums {
		data[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigurationChecksumsSecretName,
			Namespace: MachineNamespace,
		},
		Data: data,
	}
}

// =============================================================================
// Tests for calculateConcurrency
// =============================================================================

func TestCalculateConcurrency(t *testing.T) {
	tests := []struct {
		name       string
		maxConc    *intstr.IntOrString
		totalNodes int
		expected   int
	}{
		{
			name:       "nil returns 1",
			maxConc:    nil,
			totalNodes: 10,
			expected:   1,
		},
		{
			name:       "int value 3",
			maxConc:    intStrPtr(intstr.FromInt(3)),
			totalNodes: 10,
			expected:   3,
		},
		{
			name:       "string value 5",
			maxConc:    intStrPtr(intstr.FromString("5")),
			totalNodes: 10,
			expected:   5,
		},
		{
			name:       "percentage 25%",
			maxConc:    intStrPtr(intstr.FromString("25%")),
			totalNodes: 10,
			expected:   2,
		},
		{
			name:       "percentage 50%",
			maxConc:    intStrPtr(intstr.FromString("50%")),
			totalNodes: 10,
			expected:   5,
		},
		{
			name:       "percentage 5% rounds up to 1",
			maxConc:    intStrPtr(intstr.FromString("5%")),
			totalNodes: 10,
			expected:   1,
		},
		{
			name:       "percentage 100%",
			maxConc:    intStrPtr(intstr.FromString("100%")),
			totalNodes: 10,
			expected:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateConcurrency(tt.maxConc, tt.totalNodes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func intStrPtr(v intstr.IntOrString) *intstr.IntOrString {
	return &v
}

// =============================================================================
// Tests for processUpdatedNodes
// =============================================================================

func TestProcessUpdatedNodes(t *testing.T) {
	ctx := context.Background()

	t.Run("node becomes UpToDate when checksum matches and ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withChecksum("updated"),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "updated"})

		r, c := setupTestReconciler(ng, node, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		// Check that approved annotation was removed
		var updatedNode corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode)
		require.NoError(t, err)

		_, hasApproved := updatedNode.Annotations[ApprovedAnnotation]
		assert.False(t, hasApproved, "approved annotation should be removed")
	})

	t.Run("node stays approved when checksum differs", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withChecksum("old-checksum"),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "new-checksum"})

		r, c := setupTestReconciler(ng, node, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		var updatedNode corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode)
		require.NoError(t, err)

		_, hasApproved := updatedNode.Annotations[ApprovedAnnotation]
		assert.True(t, hasApproved, "approved annotation should remain")
	})

	t.Run("node stays approved when not ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withChecksum("updated"),
			withReady(false),
		)
		secret := newChecksumSecret(map[string]string{"worker": "updated"})

		r, c := setupTestReconciler(ng, node, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		var updatedNode corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode)
		require.NoError(t, err)

		_, hasApproved := updatedNode.Annotations[ApprovedAnnotation]
		assert.True(t, hasApproved, "approved annotation should remain when not ready")
	})

	t.Run("drained node becomes schedulable when UpToDate", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DrainedAnnotation, "bashible"),
			withChecksum("updated"),
			withReady(true),
			withUnschedulable(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "updated"})

		r, c := setupTestReconciler(ng, node, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		var updatedNode corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode)
		require.NoError(t, err)

		assert.False(t, updatedNode.Spec.Unschedulable, "node should become schedulable")
	})
}

// =============================================================================
// Tests for approveUpdates
// =============================================================================

func TestApproveUpdates(t *testing.T) {
	ctx := context.Background()

	t.Run("approves waiting node when all nodes ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 3, 3))
		node1 := newNode("worker-1", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withReady(true),
		)
		node2 := newNode("worker-2", "worker", withReady(true))
		node3 := newNode("worker-3", "worker", withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		var updatedNode corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode)
		require.NoError(t, err)

		_, hasApproved := updatedNode.Annotations[ApprovedAnnotation]
		_, hasWaiting := updatedNode.Annotations[WaitingForApprovalAnnotation]
		assert.True(t, hasApproved, "should have approved annotation")
		assert.False(t, hasWaiting, "should not have waiting annotation")
	})

	t.Run("respects maxConcurrent limit", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withStatus(3, 3, 3),
			withMaxConcurrent(intstr.FromInt(1)),
		)
		// One node already approved
		node1 := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withReady(true),
		)
		// Two nodes waiting
		node2 := newNode("worker-2", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withReady(true),
		)
		node3 := newNode("worker-3", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		// node2 and node3 should still be waiting (maxConcurrent=1, already 1 approved)
		var updatedNode2 corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-2"}, &updatedNode2)
		require.NoError(t, err)

		_, hasApproved := updatedNode2.Annotations[ApprovedAnnotation]
		assert.False(t, hasApproved, "should not be approved due to concurrency limit")
	})

	t.Run("approves not-ready node when some nodes are not ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 2, 3))
		node1 := newNode("worker-1", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withReady(true),
		)
		node2 := newNode("worker-2", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withReady(false), // Not ready
		)
		node3 := newNode("worker-3", "worker", withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		// Not-ready node should be approved first
		var updatedNode2 corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-2"}, &updatedNode2)
		require.NoError(t, err)

		_, hasApproved := updatedNode2.Annotations[ApprovedAnnotation]
		assert.True(t, hasApproved, "not-ready node should be approved first")
	})

	t.Run("CloudEphemeral does not approve when desired > ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeCloudEphemeral, withStatus(3, 2, 2))
		node1 := newNode("worker-1", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withReady(true),
		)
		node2 := newNode("worker-2", "worker", withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		var updatedNode1 corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode1)
		require.NoError(t, err)

		_, hasApproved := updatedNode1.Annotations[ApprovedAnnotation]
		assert.False(t, hasApproved, "should not approve when desired > ready for CloudEphemeral")
	})
}

// =============================================================================
// Tests for approveDisruptions
// =============================================================================

func TestApproveDisruptions(t *testing.T) {
	ctx := context.Background()

	t.Run("approves disruption in Automatic mode without drain", func(t *testing.T) {
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withDisruptions("Automatic", &drainBefore),
		)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		var updatedNode corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode)
		require.NoError(t, err)

		_, hasDisruptionApproved := updatedNode.Annotations[DisruptionApprovedAnnotation]
		_, hasDisruptionRequired := updatedNode.Annotations[DisruptionRequiredAnnotation]
		assert.True(t, hasDisruptionApproved, "should have disruption-approved")
		assert.False(t, hasDisruptionRequired, "should not have disruption-required")
	})

	t.Run("starts draining in Automatic mode with drain enabled", func(t *testing.T) {
		drainBefore := true
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withDisruptions("Automatic", &drainBefore),
			withStatus(3, 3, 3),
		)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withReady(true),
			withUnschedulable(false),
		)
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		var updatedNode corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode)
		require.NoError(t, err)

		draining := updatedNode.Annotations[DrainingAnnotation]
		assert.Equal(t, "bashible", draining, "should have draining annotation")
	})

	t.Run("approves disruption when already drained", func(t *testing.T) {
		drainBefore := true
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withDisruptions("Automatic", &drainBefore),
		)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withAnnotation(DrainedAnnotation, "bashible"),
			withReady(true),
			withUnschedulable(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		var updatedNode corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode)
		require.NoError(t, err)

		_, hasDisruptionApproved := updatedNode.Annotations[DisruptionApprovedAnnotation]
		assert.True(t, hasDisruptionApproved, "should have disruption-approved when already drained")
	})

	t.Run("does not approve disruption in Manual mode", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withDisruptions("Manual", nil),
		)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		var updatedNode corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode)
		require.NoError(t, err)

		_, hasDisruptionApproved := updatedNode.Annotations[DisruptionApprovedAnnotation]
		_, hasDisruptionRequired := updatedNode.Annotations[DisruptionRequiredAnnotation]
		assert.False(t, hasDisruptionApproved, "should not have disruption-approved in Manual mode")
		assert.True(t, hasDisruptionRequired, "should still have disruption-required in Manual mode")
	})

	t.Run("skips node already being drained", func(t *testing.T) {
		drainBefore := true
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withDisruptions("Automatic", &drainBefore),
		)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withAnnotation(DrainingAnnotation, "bashible"),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node, secret)

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		var updatedNode corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode)
		require.NoError(t, err)

		// Should not have changed
		_, hasDisruptionApproved := updatedNode.Annotations[DisruptionApprovedAnnotation]
		assert.False(t, hasDisruptionApproved, "should not approve while draining")
	})
}

// =============================================================================
// Tests for needDrainNode
// =============================================================================

func TestNeedDrainNode(t *testing.T) {
	t.Run("single master node should not be drained", func(t *testing.T) {
		r := &Reconciler{}
		ng := newNodeGroup("master", v1.NodeTypeStatic, withStatus(1, 1, 1))
		node := &nodeInfo{Name: "master-0", NodeGroup: "master"}

		result := r.needDrainNode(node, ng)
		assert.False(t, result, "single master should not be drained")
	})

	t.Run("deckhouse node should not be drained when only ready node", func(t *testing.T) {
		r := &Reconciler{deckhouseNodeName: "worker-1"}
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(2, 1, 2))
		node := &nodeInfo{Name: "worker-1", NodeGroup: "worker"}

		result := r.needDrainNode(node, ng)
		assert.False(t, result, "deckhouse node should not be drained when only ready node")
	})

	t.Run("deckhouse node can be drained when multiple ready nodes", func(t *testing.T) {
		r := &Reconciler{deckhouseNodeName: "worker-1"}
		drainBefore := true
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withStatus(3, 3, 3),
			withDisruptions("Automatic", &drainBefore),
		)
		node := &nodeInfo{Name: "worker-1", NodeGroup: "worker"}

		result := r.needDrainNode(node, ng)
		assert.True(t, result, "deckhouse node can be drained when multiple ready nodes")
	})

	t.Run("respects DrainBeforeApproval=false", func(t *testing.T) {
		r := &Reconciler{}
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withDisruptions("Automatic", &drainBefore),
		)
		node := &nodeInfo{Name: "worker-1", NodeGroup: "worker"}

		result := r.needDrainNode(node, ng)
		assert.False(t, result, "should not drain when DrainBeforeApproval=false")
	})
}

// =============================================================================
// Tests for buildNodeInfo
// =============================================================================

func TestBuildNodeInfo(t *testing.T) {
	t.Run("extracts all annotations correctly", func(t *testing.T) {
		r := &Reconciler{}
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "worker-1",
				Labels: map[string]string{
					NodeGroupLabel: "worker",
				},
				Annotations: map[string]string{
					ConfigurationChecksumAnnotation: "abc123",
					ApprovedAnnotation:              "",
					WaitingForApprovalAnnotation:    "",
					DisruptionRequiredAnnotation:    "",
					DisruptionApprovedAnnotation:    "",
					RollingUpdateAnnotation:         "",
					DrainingAnnotation:              "bashible",
					DrainedAnnotation:               "bashible",
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

		info := r.buildNodeInfo(node)

		assert.Equal(t, "worker-1", info.Name)
		assert.Equal(t, "worker", info.NodeGroup)
		assert.Equal(t, "abc123", info.ConfigurationChecksum)
		assert.True(t, info.IsApproved)
		assert.True(t, info.IsWaitingForApproval)
		assert.True(t, info.IsDisruptionRequired)
		assert.True(t, info.IsDisruptionApproved)
		assert.True(t, info.IsRollingUpdate)
		assert.True(t, info.IsDraining)
		assert.True(t, info.IsDrained)
		assert.True(t, info.IsUnschedulable)
		assert.True(t, info.IsReady)
	})

	t.Run("handles missing annotations", func(t *testing.T) {
		r := &Reconciler{}
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "worker-1",
				Labels: map[string]string{
					NodeGroupLabel: "worker",
				},
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
				},
			},
		}

		info := r.buildNodeInfo(node)

		assert.Equal(t, "worker-1", info.Name)
		assert.Equal(t, "worker", info.NodeGroup)
		assert.Empty(t, info.ConfigurationChecksum)
		assert.False(t, info.IsApproved)
		assert.False(t, info.IsWaitingForApproval)
		assert.False(t, info.IsDisruptionRequired)
		assert.False(t, info.IsDisruptionApproved)
		assert.False(t, info.IsRollingUpdate)
		assert.False(t, info.IsDraining)
		assert.False(t, info.IsDrained)
		assert.False(t, info.IsUnschedulable)
		assert.False(t, info.IsReady)
	})
}

// =============================================================================
// Integration test
// =============================================================================

func TestFullWorkflow(t *testing.T) {
	ctx := context.Background()

	t.Run("complete update workflow", func(t *testing.T) {
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withStatus(3, 3, 3),
			withDisruptions("Automatic", &drainBefore),
			withMaxConcurrent(intstr.FromInt(1)),
		)

		// Node waiting for approval
		node1 := newNode("worker-1", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withChecksum("old"),
			withReady(true),
		)
		node2 := newNode("worker-2", "worker",
			withChecksum("current"),
			withReady(true),
		)
		node3 := newNode("worker-3", "worker",
			withChecksum("current"),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "current"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)

		// First reconcile: should approve node1
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		var updatedNode1 corev1.Node
		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode1)
		require.NoError(t, err)

		_, hasApproved := updatedNode1.Annotations[ApprovedAnnotation]
		assert.True(t, hasApproved, "node should be approved")

		// Simulate node update: add disruption-required
		updatedNode1.Annotations[DisruptionRequiredAnnotation] = ""
		err = c.Update(ctx, &updatedNode1)
		require.NoError(t, err)

		// Second reconcile: should approve disruption
		_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode1)
		require.NoError(t, err)

		_, hasDisruptionApproved := updatedNode1.Annotations[DisruptionApprovedAnnotation]
		assert.True(t, hasDisruptionApproved, "disruption should be approved")

		// Simulate node becoming up to date
		updatedNode1.Annotations[ConfigurationChecksumAnnotation] = "current"
		err = c.Update(ctx, &updatedNode1)
		require.NoError(t, err)

		// Third reconcile: should mark as UpToDate
		_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
		require.NoError(t, err)

		err = c.Get(ctx, types.NamespacedName{Name: "worker-1"}, &updatedNode1)
		require.NoError(t, err)

		_, hasApproved = updatedNode1.Annotations[ApprovedAnnotation]
		assert.False(t, hasApproved, "approved should be removed when up to date")
	})
}
