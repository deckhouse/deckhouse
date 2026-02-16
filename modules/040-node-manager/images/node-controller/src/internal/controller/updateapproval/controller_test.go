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

package updateapproval

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
		if ng.Spec.Disruptions == nil {
			ng.Spec.Disruptions = &v1.DisruptionsSpec{}
		}
		ng.Spec.Disruptions.ApprovalMode = v1.DisruptionApprovalMode(mode)
		if drainBefore != nil {
			if ng.Spec.Disruptions.Automatic == nil {
				ng.Spec.Disruptions.Automatic = &v1.AutomaticDisruptionSpec{}
			}
			ng.Spec.Disruptions.Automatic.DrainBeforeApproval = drainBefore
		}
	}
}

func withMaxConcurrent(val intstr.IntOrString) func(*v1.NodeGroup) {
	return func(ng *v1.NodeGroup) {
		if ng.Spec.Update == nil {
			ng.Spec.Update = &v1.UpdateSpec{}
		}
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

func intStrPtr(v intstr.IntOrString) *intstr.IntOrString {
	return &v
}

func reconcileNG(t *testing.T, r *Reconciler, ngName string) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: ngName},
	})
	require.NoError(t, err)
}

func getNode(t *testing.T, c client.Client, name string) corev1.Node {
	t.Helper()
	var node corev1.Node
	err := c.Get(context.Background(), types.NamespacedName{Name: name}, &node)
	require.NoError(t, err)
	return node
}

func hasAnnotation(node corev1.Node, key string) bool {
	_, ok := node.Annotations[key]
	return ok
}

// fixedTime returns a deterministic time for window tests: Wed Jan 13 13:30:00 UTC 2021
func fixedTime() time.Time {
	return time.Date(2021, 1, 13, 13, 30, 0, 0, time.UTC)
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

// =============================================================================
// Tests for processUpdatedNodes
// =============================================================================

func TestProcessUpdatedNodes(t *testing.T) {
	t.Run("node becomes UpToDate when checksum matches and ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withChecksum("updated"),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "updated"})

		r, c := setupTestReconciler(ng, node, secret)
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(updated, ApprovedAnnotation), "approved annotation should be removed")
		assert.False(t, hasAnnotation(updated, DisruptionRequiredAnnotation), "disruption-required should be removed")
		assert.False(t, hasAnnotation(updated, DisruptionApprovedAnnotation), "disruption-approved should be removed")
		assert.False(t, hasAnnotation(updated, DrainedAnnotation), "drained should be removed")
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
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(updated, ApprovedAnnotation), "approved annotation should remain")
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
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(updated, ApprovedAnnotation), "approved annotation should remain when not ready")
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
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.False(t, updated.Spec.Unschedulable, "node should become schedulable")
		assert.False(t, hasAnnotation(updated, DrainedAnnotation), "drained annotation should be removed")
	})

	t.Run("node stays approved when node checksum is empty", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "updated"})

		r, c := setupTestReconciler(ng, node, secret)
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(updated, ApprovedAnnotation), "approved should remain when node has no checksum")
	})
}

// =============================================================================
// Tests for finished behavior (one mutation per reconcile)
// =============================================================================

func TestFinishedBehavior(t *testing.T) {
	t.Run("processUpdatedNodes stops after first node", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic)
		node1 := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withChecksum("current"),
			withReady(true),
		)
		node2 := newNode("worker-2", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withChecksum("current"),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "current"})

		r, c := setupTestReconciler(ng, node1, node2, secret)
		reconcileNG(t, r, "worker")

		n1 := getNode(t, c, "worker-1")
		n2 := getNode(t, c, "worker-2")

		n1Cleared := !hasAnnotation(n1, ApprovedAnnotation)
		n2Cleared := !hasAnnotation(n2, ApprovedAnnotation)

		assert.True(t, n1Cleared || n2Cleared, "at least one node should be processed")
		assert.False(t, n1Cleared && n2Cleared, "only one node should be processed per reconcile")
	})

	t.Run("processUpdatedNodes blocks approveDisruptions", func(t *testing.T) {
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withDisruptions("Automatic", &drainBefore),
		)
		node1 := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withChecksum("current"),
			withReady(true),
		)
		node2 := newNode("worker-2", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "current"})

		r, c := setupTestReconciler(ng, node1, node2, secret)
		reconcileNG(t, r, "worker")

		n1 := getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(n1, ApprovedAnnotation), "node1 should be cleared")

		n2 := getNode(t, c, "worker-2")
		assert.False(t, hasAnnotation(n2, DisruptionApprovedAnnotation),
			"disruption should not be approved in same reconcile as processUpdatedNodes")
	})

	t.Run("approveDisruptions blocks approveUpdates", func(t *testing.T) {
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withDisruptions("Automatic", &drainBefore),
			withMaxConcurrent(intstr.FromInt(2)),
		)
		node1 := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withChecksum("old"),
			withReady(true),
		)
		node2 := newNode("worker-2", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withChecksum("old"),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "current"})

		r, c := setupTestReconciler(ng, node1, node2, secret)
		reconcileNG(t, r, "worker")

		n1 := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(n1, DisruptionApprovedAnnotation),
			"disruption should be approved for node1")

		n2 := getNode(t, c, "worker-2")
		assert.True(t, hasAnnotation(n2, WaitingForApprovalAnnotation),
			"node2 should still be waiting, blocked by finished")
		assert.False(t, hasAnnotation(n2, ApprovedAnnotation),
			"node2 should not be approved yet")
	})
}

// =============================================================================
// Tests for approveUpdates
// =============================================================================

func TestApproveUpdates(t *testing.T) {
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
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(updated, ApprovedAnnotation), "should have approved annotation")
		assert.False(t, hasAnnotation(updated, WaitingForApprovalAnnotation), "should not have waiting annotation")
	})

	t.Run("respects maxConcurrent limit", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withStatus(3, 3, 3),
			withMaxConcurrent(intstr.FromInt(1)),
		)
		node1 := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withReady(true),
		)
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
		reconcileNG(t, r, "worker")

		n2 := getNode(t, c, "worker-2")
		assert.False(t, hasAnnotation(n2, ApprovedAnnotation), "should not be approved due to concurrency limit")
	})

	t.Run("approves not-ready node when some nodes are not ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 2, 3))
		node1 := newNode("worker-1", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withReady(true),
		)
		node2 := newNode("worker-2", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withReady(false),
		)
		node3 := newNode("worker-3", "worker", withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)
		reconcileNG(t, r, "worker")

		n2 := getNode(t, c, "worker-2")
		assert.True(t, hasAnnotation(n2, ApprovedAnnotation), "not-ready node should be approved")
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
		reconcileNG(t, r, "worker")

		n1 := getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(n1, ApprovedAnnotation),
			"should not approve when desired > ready for CloudEphemeral")
	})

	t.Run("CloudEphemeral approves when desired <= ready and all ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeCloudEphemeral, withStatus(2, 3, 3))
		node1 := newNode("worker-1", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withReady(true),
		)
		node2 := newNode("worker-2", "worker", withReady(true))
		node3 := newNode("worker-3", "worker", withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)
		reconcileNG(t, r, "worker")

		n1 := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(n1, ApprovedAnnotation),
			"should approve when desired <= ready for CloudEphemeral")
	})

	t.Run("does not approve when no nodes waiting", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 3, 3))
		node1 := newNode("worker-1", "worker", withReady(true))
		node2 := newNode("worker-2", "worker", withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, secret)
		reconcileNG(t, r, "worker")

		n1 := getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(n1, ApprovedAnnotation), "no nodes should be approved")
	})

	t.Run("approves multiple nodes up to concurrency", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withStatus(3, 3, 3),
			withMaxConcurrent(intstr.FromInt(2)),
		)
		node1 := newNode("worker-1", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withReady(true),
		)
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
		reconcileNG(t, r, "worker")

		approved := 0
		for _, name := range []string{"worker-1", "worker-2", "worker-3"} {
			n := getNode(t, c, name)
			if hasAnnotation(n, ApprovedAnnotation) {
				approved++
			}
		}
		assert.Equal(t, 2, approved, "should approve exactly 2 nodes")
	})
}

// =============================================================================
// Tests for approveDisruptions
// =============================================================================

func TestApproveDisruptions(t *testing.T) {
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
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(updated, DisruptionApprovedAnnotation), "should have disruption-approved")
		assert.False(t, hasAnnotation(updated, DisruptionRequiredAnnotation), "should not have disruption-required")
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
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.Equal(t, "bashible", updated.Annotations[DrainingAnnotation], "should have draining annotation")
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
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(updated, DisruptionApprovedAnnotation),
			"should have disruption-approved when already drained")
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
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(updated, DisruptionApprovedAnnotation),
			"should not have disruption-approved in Manual mode")
		assert.True(t, hasAnnotation(updated, DisruptionRequiredAnnotation),
			"should still have disruption-required in Manual mode")
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
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(updated, DisruptionApprovedAnnotation),
			"should not approve while draining")
	})

	t.Run("skips node without approved annotation", func(t *testing.T) {
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withDisruptions("Automatic", &drainBefore),
		)
		node := newNode("worker-1", "worker",
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node, secret)
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(updated, DisruptionApprovedAnnotation),
			"should not approve disruption without approved annotation")
	})

	t.Run("default approval mode is Automatic", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic)
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node, secret)
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.Equal(t, "bashible", updated.Annotations[DrainingAnnotation],
			"default mode Automatic should start draining")
	})
}

// =============================================================================
// Tests for needDrainNode
// =============================================================================

func TestNeedDrainNode(t *testing.T) {
	ctx := context.Background()

	t.Run("single master node should not be drained", func(t *testing.T) {
		r := &Reconciler{}
		ng := newNodeGroup("master", v1.NodeTypeStatic, withStatus(1, 1, 1))
		node := &nodeInfo{Name: "master-0", NodeGroup: "master"}

		assert.False(t, r.needDrainNode(ctx, node, ng), "single master should not be drained")
	})

	t.Run("deckhouse node should not be drained when only ready node", func(t *testing.T) {
		r := &Reconciler{deckhouseNodeName: "worker-1"}
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(2, 1, 2))
		node := &nodeInfo{Name: "worker-1", NodeGroup: "worker"}

		assert.False(t, r.needDrainNode(ctx, node, ng), "deckhouse node should not be drained when only ready node")
	})

	t.Run("deckhouse node can be drained when multiple ready nodes", func(t *testing.T) {
		r := &Reconciler{deckhouseNodeName: "worker-1"}
		drainBefore := true
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withStatus(3, 3, 3),
			withDisruptions("Automatic", &drainBefore),
		)
		node := &nodeInfo{Name: "worker-1", NodeGroup: "worker"}

		assert.True(t, r.needDrainNode(ctx, node, ng), "deckhouse node can be drained when multiple ready nodes")
	})

	t.Run("respects DrainBeforeApproval=false", func(t *testing.T) {
		r := &Reconciler{}
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withDisruptions("Automatic", &drainBefore),
		)
		node := &nodeInfo{Name: "worker-1", NodeGroup: "worker"}

		assert.False(t, r.needDrainNode(ctx, node, ng), "should not drain when DrainBeforeApproval=false")
	})

	t.Run("defaults to true when no disruptions spec", func(t *testing.T) {
		r := &Reconciler{}
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 3, 3))
		node := &nodeInfo{Name: "worker-1", NodeGroup: "worker"}

		assert.True(t, r.needDrainNode(ctx, node, ng), "should default to drain=true")
	})

	t.Run("multi-master can be drained", func(t *testing.T) {
		r := &Reconciler{}
		drainBefore := true
		ng := newNodeGroup("master", v1.NodeTypeStatic,
			withStatus(3, 3, 3),
			withDisruptions("Automatic", &drainBefore),
		)
		node := &nodeInfo{Name: "master-0", NodeGroup: "master"}

		assert.True(t, r.needDrainNode(ctx, node, ng), "multi-master nodes can be drained")
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
				Name:   "worker-1",
				Labels: map[string]string{NodeGroupLabel: "worker"},
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
			Spec:   corev1.NodeSpec{Unschedulable: true},
			Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
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
				Name:   "worker-1",
				Labels: map[string]string{NodeGroupLabel: "worker"},
			},
			Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}},
		}

		info := r.buildNodeInfo(node)

		assert.False(t, info.IsApproved)
		assert.False(t, info.IsWaitingForApproval)
		assert.False(t, info.IsDraining)
		assert.False(t, info.IsDrained)
		assert.False(t, info.IsReady)
	})

	t.Run("ignores non-bashible draining annotation", func(t *testing.T) {
		r := &Reconciler{}
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "worker-1",
				Labels:      map[string]string{NodeGroupLabel: "worker"},
				Annotations: map[string]string{DrainingAnnotation: "other", DrainedAnnotation: "other"},
			},
		}

		info := r.buildNodeInfo(node)
		assert.False(t, info.IsDraining)
		assert.False(t, info.IsDrained)
	})
}

// =============================================================================
// Tests for calculateNodeStatus (metrics)
// =============================================================================

func TestCalculateNodeStatus(t *testing.T) {
	ng := newNodeGroup("worker", v1.NodeTypeStatic)
	ngManual := newNodeGroup("worker", v1.NodeTypeStatic, withDisruptions("Manual", nil))

	tests := []struct {
		name     string
		node     nodeInfo
		ng       *v1.NodeGroup
		checksum string
		expected string
	}{
		{"WaitingForApproval", nodeInfo{IsWaitingForApproval: true}, ng, "abc", "WaitingForApproval"},
		{"DrainingForDisruption", nodeInfo{IsApproved: true, IsDisruptionRequired: true, IsDraining: true}, ng, "abc", "DrainingForDisruption"},
		{"Draining", nodeInfo{IsDraining: true}, ng, "abc", "Draining"},
		{"Drained", nodeInfo{IsDrained: true}, ng, "abc", "Drained"},
		{"WaitingForDisruptionApproval", nodeInfo{IsApproved: true, IsDisruptionRequired: true}, ng, "abc", "WaitingForDisruptionApproval"},
		{"WaitingForManualDisruptionApproval", nodeInfo{IsApproved: true, IsDisruptionRequired: true}, ngManual, "abc", "WaitingForManualDisruptionApproval"},
		{"DisruptionApproved", nodeInfo{IsApproved: true, IsDisruptionApproved: true}, ng, "abc", "DisruptionApproved"},
		{"Approved", nodeInfo{IsApproved: true}, ng, "abc", "Approved"},
		{"UpdateFailedNoConfigChecksum", nodeInfo{ConfigurationChecksum: ""}, ng, "abc", "UpdateFailedNoConfigChecksum"},
		{"ToBeUpdated", nodeInfo{ConfigurationChecksum: "old"}, ng, "new", "ToBeUpdated"},
		{"UpToDate", nodeInfo{ConfigurationChecksum: "abc"}, ng, "abc", "UpToDate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, calculateNodeStatus(tt.node, tt.ng, tt.checksum))
		})
	}
}

// =============================================================================
// Tests for getConfigurationChecksums
// =============================================================================

func TestGetConfigurationChecksums(t *testing.T) {
	ctx := context.Background()

	t.Run("returns checksums from secret", func(t *testing.T) {
		secret := newChecksumSecret(map[string]string{"worker": "cs1", "master": "cs2"})
		r, _ := setupTestReconciler(secret)

		checksums, err := r.getConfigurationChecksums(ctx)
		require.NoError(t, err)
		assert.Equal(t, "cs1", checksums["worker"])
		assert.Equal(t, "cs2", checksums["master"])
	})

	t.Run("returns nil when secret not found", func(t *testing.T) {
		r, _ := setupTestReconciler()

		checksums, err := r.getConfigurationChecksums(ctx)
		require.NoError(t, err)
		assert.Nil(t, checksums)
	})
}

// =============================================================================
// Tests for Reconcile edge cases
// =============================================================================

func TestReconcileEdgeCases(t *testing.T) {
	t.Run("skips when nodegroup not found", func(t *testing.T) {
		r, _ := setupTestReconciler()
		_, err := r.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: types.NamespacedName{Name: "nonexistent"},
		})
		require.NoError(t, err)
	})

	t.Run("skips when no checksums secret", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic)
		node := newNode("worker-1", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withReady(true),
		)

		r, c := setupTestReconciler(ng, node)
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(updated, WaitingForApprovalAnnotation),
			"node should be unchanged when no checksums secret")
	})

	t.Run("handles nodegroup with no nodes", func(t *testing.T) {
		ng := newNodeGroup("empty", v1.NodeTypeStatic)
		secret := newChecksumSecret(map[string]string{"empty": "checksum"})

		r, _ := setupTestReconciler(ng, secret)
		_, err := r.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: types.NamespacedName{Name: "empty"},
		})
		require.NoError(t, err)
	})
}

// =============================================================================
// Tests for isInAllowedWindow
// =============================================================================

func TestIsInAllowedWindow(t *testing.T) {
	t.Run("empty windows always allowed", func(t *testing.T) {
		assert.True(t, isInAllowedWindow(nil, fixedTime()))
		assert.True(t, isInAllowedWindow([]v1.DisruptionWindow{}, fixedTime()))
	})

	t.Run("within window", func(t *testing.T) {
		windows := []v1.DisruptionWindow{{From: "13:00", To: "14:00"}}
		assert.True(t, isInAllowedWindow(windows, fixedTime()))
	})

	t.Run("outside window", func(t *testing.T) {
		windows := []v1.DisruptionWindow{{From: "14:00", To: "15:00"}}
		assert.False(t, isInAllowedWindow(windows, fixedTime()))
	})

	t.Run("midnight crossing window", func(t *testing.T) {
		windows := []v1.DisruptionWindow{{From: "23:00", To: "02:00"}}
		midnight := time.Date(2021, 1, 13, 0, 30, 0, 0, time.UTC)
		assert.True(t, isInAllowedWindow(windows, midnight))
	})

	t.Run("invalid time format returns false", func(t *testing.T) {
		windows := []v1.DisruptionWindow{{From: "invalid", To: "15:00"}}
		assert.False(t, isInAllowedWindow(windows, fixedTime()))
	})

	t.Run("day of week match", func(t *testing.T) {
		// fixedTime is Wednesday
		windows := []v1.DisruptionWindow{{From: "13:00", To: "14:00", Days: []string{"Wednesday"}}}
		assert.True(t, isInAllowedWindow(windows, fixedTime()))
	})

	t.Run("day of week mismatch", func(t *testing.T) {
		windows := []v1.DisruptionWindow{{From: "13:00", To: "14:00", Days: []string{"Monday"}}}
		assert.False(t, isInAllowedWindow(windows, fixedTime()))
	})
}

// =============================================================================
// Integration tests
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
		node1 := newNode("worker-1", "worker",
			withAnnotation(WaitingForApprovalAnnotation, ""),
			withChecksum("old"),
			withReady(true),
		)
		node2 := newNode("worker-2", "worker", withChecksum("current"), withReady(true))
		node3 := newNode("worker-3", "worker", withChecksum("current"), withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "current"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)

		// Step 1: approve node1
		reconcileNG(t, r, "worker")
		n1 := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(n1, ApprovedAnnotation), "node should be approved")
		assert.False(t, hasAnnotation(n1, WaitingForApprovalAnnotation), "waiting should be removed")

		// Simulate: bashible sets disruption-required
		n1.Annotations[DisruptionRequiredAnnotation] = ""
		require.NoError(t, c.Update(ctx, &n1))

		// Step 2: approve disruption
		reconcileNG(t, r, "worker")
		n1 = getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(n1, DisruptionApprovedAnnotation), "disruption should be approved")

		// Simulate: node update completes, checksum matches
		n1.Annotations[ConfigurationChecksumAnnotation] = "current"
		require.NoError(t, c.Update(ctx, &n1))

		// Step 3: mark UpToDate
		reconcileNG(t, r, "worker")
		n1 = getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(n1, ApprovedAnnotation), "approved should be removed")
		assert.False(t, hasAnnotation(n1, DisruptionApprovedAnnotation), "disruption-approved should be removed")
	})

	t.Run("complete drain workflow", func(t *testing.T) {
		drainBefore := true
		ng := newNodeGroup("worker", v1.NodeTypeStatic,
			withStatus(3, 3, 3),
			withDisruptions("Automatic", &drainBefore),
			withMaxConcurrent(intstr.FromInt(1)),
		)
		node1 := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withChecksum("old"),
			withReady(true),
			withUnschedulable(false),
		)
		node2 := newNode("worker-2", "worker", withChecksum("current"), withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "current"})

		r, c := setupTestReconciler(ng, node1, node2, secret)

		// Step 1: start draining
		reconcileNG(t, r, "worker")
		n1 := getNode(t, c, "worker-1")
		assert.Equal(t, "bashible", n1.Annotations[DrainingAnnotation], "should start draining")

		// Simulate: bashible completes drain
		delete(n1.Annotations, DrainingAnnotation)
		n1.Annotations[DrainedAnnotation] = "bashible"
		n1.Spec.Unschedulable = true
		require.NoError(t, c.Update(ctx, &n1))

		// Step 2: approve disruption
		reconcileNG(t, r, "worker")
		n1 = getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(n1, DisruptionApprovedAnnotation), "disruption should be approved after drain")

		// Simulate: node update completes
		n1.Annotations[ConfigurationChecksumAnnotation] = "current"
		require.NoError(t, c.Update(ctx, &n1))

		// Step 3: mark UpToDate, uncordon
		reconcileNG(t, r, "worker")
		n1 = getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(n1, ApprovedAnnotation), "approved should be removed")
		assert.False(t, n1.Spec.Unschedulable, "node should be uncordoned")
		assert.False(t, hasAnnotation(n1, DrainedAnnotation), "drained should be removed")
	})
}
