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
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
)

const (
	NodeGroupLabel                   = ua.NodeGroupLabel
	ConfigurationChecksumAnnotation  = ua.ConfigurationChecksumAnnotation
	MachineNamespace                 = ua.MachineNamespace
	ConfigurationChecksumsSecretName = ua.ConfigurationChecksumsSecretName
	ApprovedAnnotation               = ua.ApprovedAnnotation
	WaitingForApprovalAnnotation     = ua.WaitingForApprovalAnnotation
	DisruptionRequiredAnnotation     = ua.DisruptionRequiredAnnotation
	DisruptionApprovedAnnotation     = ua.DisruptionApprovedAnnotation
	RollingUpdateAnnotation          = ua.RollingUpdateAnnotation
	DrainingAnnotation               = ua.DrainingAnnotation
	DrainedAnnotation                = ua.DrainedAnnotation
)

type nodeInfo = ua.NodeInfo

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
		assert.True(t, hasAnnotation(updated, WaitingForApprovalAnnotation))
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
		assert.False(t, hasAnnotation(updated, ApprovedAnnotation))
		assert.False(t, hasAnnotation(updated, DisruptionRequiredAnnotation))
		assert.False(t, hasAnnotation(updated, DisruptionApprovedAnnotation))
		assert.False(t, hasAnnotation(updated, DrainedAnnotation))
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
		assert.True(t, hasAnnotation(updated, ApprovedAnnotation))
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
		assert.True(t, hasAnnotation(updated, ApprovedAnnotation))
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
		assert.False(t, updated.Spec.Unschedulable)
		assert.False(t, hasAnnotation(updated, DrainedAnnotation))
	})

	t.Run("late drained node cleanup removes unschedulable after approved was already cleared", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic)
		node := newNode("worker-1", "worker",
			withAnnotation(DisruptionApprovedAnnotation, ""),
			withAnnotation(DrainedAnnotation, "bashible"),
			withChecksum("updated"),
			withReady(true),
			withUnschedulable(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "updated"})

		r, c := setupTestReconciler(ng, node, secret)
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.False(t, updated.Spec.Unschedulable)
		assert.False(t, hasAnnotation(updated, DrainedAnnotation))
		assert.False(t, hasAnnotation(updated, DisruptionApprovedAnnotation))
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
		assert.True(t, hasAnnotation(updated, ApprovedAnnotation))
	})
}

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
		assert.True(t, n1Cleared || n2Cleared)
		assert.False(t, n1Cleared && n2Cleared)
	})

	t.Run("processUpdatedNodes blocks approveDisruptions", func(t *testing.T) {
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withDisruptions("Automatic", &drainBefore))
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
		assert.False(t, hasAnnotation(n1, ApprovedAnnotation))

		n2 := getNode(t, c, "worker-2")
		assert.False(t, hasAnnotation(n2, DisruptionApprovedAnnotation))
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
		assert.True(t, hasAnnotation(n1, DisruptionApprovedAnnotation))

		n2 := getNode(t, c, "worker-2")
		assert.True(t, hasAnnotation(n2, WaitingForApprovalAnnotation))
		assert.False(t, hasAnnotation(n2, ApprovedAnnotation))
	})
}

func TestApproveUpdates(t *testing.T) {
	t.Run("approves waiting node when all nodes ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 3, 3))
		node1 := newNode("worker-1", "worker", withAnnotation(WaitingForApprovalAnnotation, ""), withReady(true))
		node2 := newNode("worker-2", "worker", withReady(true))
		node3 := newNode("worker-3", "worker", withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(updated, ApprovedAnnotation))
		assert.False(t, hasAnnotation(updated, WaitingForApprovalAnnotation))
	})

	t.Run("respects maxConcurrent limit", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 3, 3), withMaxConcurrent(intstr.FromInt(1)))
		node1 := newNode("worker-1", "worker", withAnnotation(ApprovedAnnotation, ""), withReady(true))
		node2 := newNode("worker-2", "worker", withAnnotation(WaitingForApprovalAnnotation, ""), withReady(true))
		node3 := newNode("worker-3", "worker", withAnnotation(WaitingForApprovalAnnotation, ""), withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)
		reconcileNG(t, r, "worker")

		n2 := getNode(t, c, "worker-2")
		assert.False(t, hasAnnotation(n2, ApprovedAnnotation))
	})

	t.Run("approves not-ready node when some nodes are not ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 2, 3))
		node1 := newNode("worker-1", "worker", withAnnotation(WaitingForApprovalAnnotation, ""), withReady(true))
		node2 := newNode("worker-2", "worker", withAnnotation(WaitingForApprovalAnnotation, ""), withReady(false))
		node3 := newNode("worker-3", "worker", withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)
		reconcileNG(t, r, "worker")

		n2 := getNode(t, c, "worker-2")
		assert.True(t, hasAnnotation(n2, ApprovedAnnotation))
	})

	t.Run("CloudEphemeral does not approve when desired > ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeCloudEphemeral, withStatus(3, 2, 2))
		node1 := newNode("worker-1", "worker", withAnnotation(WaitingForApprovalAnnotation, ""), withReady(true))
		node2 := newNode("worker-2", "worker", withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, secret)
		reconcileNG(t, r, "worker")

		n1 := getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(n1, ApprovedAnnotation))
	})

	t.Run("CloudEphemeral approves when desired <= ready and all ready", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeCloudEphemeral, withStatus(2, 3, 3))
		node1 := newNode("worker-1", "worker", withAnnotation(WaitingForApprovalAnnotation, ""), withReady(true))
		node2 := newNode("worker-2", "worker", withReady(true))
		node3 := newNode("worker-3", "worker", withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)
		reconcileNG(t, r, "worker")

		n1 := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(n1, ApprovedAnnotation))
	})

	t.Run("does not approve when no nodes waiting", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 3, 3))
		node1 := newNode("worker-1", "worker", withReady(true))
		node2 := newNode("worker-2", "worker", withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node1, node2, secret)
		reconcileNG(t, r, "worker")

		n1 := getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(n1, ApprovedAnnotation))
	})

	t.Run("approves multiple nodes up to concurrency", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 3, 3), withMaxConcurrent(intstr.FromInt(2)))
		node1 := newNode("worker-1", "worker", withAnnotation(WaitingForApprovalAnnotation, ""), withReady(true))
		node2 := newNode("worker-2", "worker", withAnnotation(WaitingForApprovalAnnotation, ""), withReady(true))
		node3 := newNode("worker-3", "worker", withAnnotation(WaitingForApprovalAnnotation, ""), withReady(true))
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
		assert.Equal(t, 2, approved)
	})
}

func TestApproveDisruptions(t *testing.T) {
	t.Run("approves disruption in Automatic mode without drain", func(t *testing.T) {
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withDisruptions("Automatic", &drainBefore))
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node, secret)
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(updated, DisruptionApprovedAnnotation))
		assert.False(t, hasAnnotation(updated, DisruptionRequiredAnnotation))
	})

	t.Run("starts draining in Automatic mode with drain enabled", func(t *testing.T) {
		drainBefore := true
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withDisruptions("Automatic", &drainBefore), withStatus(3, 3, 3))
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
		assert.Equal(t, "bashible", updated.Annotations[DrainingAnnotation])
	})

	t.Run("approves disruption when already drained", func(t *testing.T) {
		drainBefore := true
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withDisruptions("Automatic", &drainBefore))
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
		assert.True(t, hasAnnotation(updated, DisruptionApprovedAnnotation))
	})

	t.Run("does not approve disruption in Manual mode", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withDisruptions("Manual", nil))
		node := newNode("worker-1", "worker",
			withAnnotation(ApprovedAnnotation, ""),
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node, secret)
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(updated, DisruptionApprovedAnnotation))
		assert.True(t, hasAnnotation(updated, DisruptionRequiredAnnotation))
	})

	t.Run("skips node already being drained", func(t *testing.T) {
		drainBefore := true
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withDisruptions("Automatic", &drainBefore))
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
		assert.False(t, hasAnnotation(updated, DisruptionApprovedAnnotation))
	})

	t.Run("skips node without approved annotation", func(t *testing.T) {
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withDisruptions("Automatic", &drainBefore))
		node := newNode("worker-1", "worker",
			withAnnotation(DisruptionRequiredAnnotation, ""),
			withReady(true),
		)
		secret := newChecksumSecret(map[string]string{"worker": "checksum"})

		r, c := setupTestReconciler(ng, node, secret)
		reconcileNG(t, r, "worker")

		updated := getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(updated, DisruptionApprovedAnnotation))
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
		assert.Equal(t, "bashible", updated.Annotations[DrainingAnnotation])
	})
}

func TestFullWorkflow(t *testing.T) {
	ctx := context.Background()

	t.Run("complete update workflow", func(t *testing.T) {
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 3, 3), withDisruptions("Automatic", &drainBefore), withMaxConcurrent(intstr.FromInt(1)))
		node1 := newNode("worker-1", "worker", withAnnotation(WaitingForApprovalAnnotation, ""), withChecksum("old"), withReady(true))
		node2 := newNode("worker-2", "worker", withChecksum("current"), withReady(true))
		node3 := newNode("worker-3", "worker", withChecksum("current"), withReady(true))
		secret := newChecksumSecret(map[string]string{"worker": "current"})

		r, c := setupTestReconciler(ng, node1, node2, node3, secret)

		reconcileNG(t, r, "worker")
		n1 := getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(n1, ApprovedAnnotation))
		assert.False(t, hasAnnotation(n1, WaitingForApprovalAnnotation))

		n1.Annotations[DisruptionRequiredAnnotation] = ""
		require.NoError(t, c.Update(ctx, &n1))

		reconcileNG(t, r, "worker")
		n1 = getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(n1, DisruptionApprovedAnnotation))

		n1.Annotations[ConfigurationChecksumAnnotation] = "current"
		require.NoError(t, c.Update(ctx, &n1))

		reconcileNG(t, r, "worker")
		n1 = getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(n1, ApprovedAnnotation))
		assert.False(t, hasAnnotation(n1, DisruptionApprovedAnnotation))
	})

	t.Run("complete drain workflow", func(t *testing.T) {
		drainBefore := true
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 3, 3), withDisruptions("Automatic", &drainBefore), withMaxConcurrent(intstr.FromInt(1)))
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

		reconcileNG(t, r, "worker")
		n1 := getNode(t, c, "worker-1")
		assert.Equal(t, "bashible", n1.Annotations[DrainingAnnotation])

		delete(n1.Annotations, DrainingAnnotation)
		n1.Annotations[DrainedAnnotation] = "bashible"
		n1.Spec.Unschedulable = true
		require.NoError(t, c.Update(ctx, &n1))

		reconcileNG(t, r, "worker")
		n1 = getNode(t, c, "worker-1")
		assert.True(t, hasAnnotation(n1, DisruptionApprovedAnnotation))

		n1.Annotations[ConfigurationChecksumAnnotation] = "current"
		require.NoError(t, c.Update(ctx, &n1))

		reconcileNG(t, r, "worker")
		n1 = getNode(t, c, "worker-1")
		assert.False(t, hasAnnotation(n1, ApprovedAnnotation))
		assert.False(t, n1.Spec.Unschedulable)
		assert.False(t, hasAnnotation(n1, DrainedAnnotation))
	})
}
