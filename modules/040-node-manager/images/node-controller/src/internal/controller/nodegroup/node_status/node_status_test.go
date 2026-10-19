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

package node_status

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := internalv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add internal scheme: %v", err)
	}
	return scheme
}

func makeNode(name, ngName string, ready bool, checksum string) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"node.deckhouse.io/group": ngName},
		},
	}
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}}
	if checksum != "" {
		node.Annotations = map[string]string{"node.deckhouse.io/configuration-checksum": checksum}
	}
	return node
}

func makeChecksumSecret(data map[string]string) *corev1.Secret {
	secretData := make(map[string][]byte, len(data))
	for k, v := range data {
		secretData[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configuration-checksums",
			Namespace: "d8-cloud-instance-manager",
		},
		Data: secretData,
	}
}

func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want bool
	}{
		{
			name: "ready",
			node: &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			}}},
			want: true,
		},
		{
			name: "not ready",
			node: &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
			}}},
			want: false,
		},
		{
			name: "no ready condition",
			node: &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
			}}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNodeReady(tt.node); got != tt.want {
				t.Fatalf("isNodeReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompute(t *testing.T) {
	tests := []struct {
		name         string
		objs         []runtime.Object
		wantNodes    int32
		wantReady    int32
		wantUpToDate int32
	}{
		{
			name:      "no nodes",
			objs:      nil,
			wantNodes: 0,
			wantReady: 0,
		},
		{
			name: "mixed readiness no checksum secret",
			objs: []runtime.Object{
				makeNode("n1", "worker", true, ""),
				makeNode("n2", "worker", true, ""),
				makeNode("n3", "worker", false, ""),
			},
			wantNodes:    3,
			wantReady:    2,
			wantUpToDate: 0,
		},
		{
			name: "checksum match counts up-to-date",
			objs: []runtime.Object{
				makeNode("n1", "worker", true, "abc"),
				makeNode("n2", "worker", true, "abc"),
				makeNode("n3", "worker", true, "stale"),
				makeChecksumSecret(map[string]string{"worker": "abc"}),
			},
			wantNodes:    3,
			wantReady:    3,
			wantUpToDate: 2,
		},
		{
			name: "nodes from other group excluded",
			objs: []runtime.Object{
				makeNode("n1", "worker", true, ""),
				makeNode("o1", "other", true, ""),
			},
			wantNodes: 1,
			wantReady: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(newScheme(t)).WithRuntimeObjects(tt.objs...).Build()
			s := &Service{Client: cl}

			res, err := s.Compute(context.Background(), &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}})
			if err != nil {
				t.Fatalf("Compute: %v", err)
			}
			if res.NodesCount != tt.wantNodes {
				t.Errorf("NodesCount = %d, want %d", res.NodesCount, tt.wantNodes)
			}
			if res.ReadyCount != tt.wantReady {
				t.Errorf("ReadyCount = %d, want %d", res.ReadyCount, tt.wantReady)
			}
			if res.UpToDateCount != tt.wantUpToDate {
				t.Errorf("UpToDateCount = %d, want %d", res.UpToDateCount, tt.wantUpToDate)
			}
			if int32(len(res.NodesForConditions)) != tt.wantNodes {
				t.Errorf("NodesForConditions len = %d, want %d", len(res.NodesForConditions), tt.wantNodes)
			}
		})
	}
}

func TestGetConfigurationChecksum_MissingSecretReturnsEmpty(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(newScheme(t)).Build()
	s := &Service{Client: cl}
	if got := s.getConfigurationChecksum(context.Background(), "worker"); got != "" {
		t.Fatalf("expected empty checksum when secret missing, got %q", got)
	}
}

func makeNodeConfig(name string, generation, applied int64, conditions ...metav1.Condition) *internalv1alpha1.NodeConfig {
	return &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Generation: generation},
		Status:     internalv1alpha1.NodeConfigStatus{AppliedGeneration: applied, Conditions: conditions},
	}
}

// An Engine node never carries the bashible checksum: its rollout is read off its NodeConfig.
func TestCompute_ImmutableGroupReadsNodeConfig(t *testing.T) {
	appliedAt := func(generation int64) metav1.Condition {
		return metav1.Condition{Type: "ConfigurationApplied", Status: metav1.ConditionTrue, ObservedGeneration: generation}
	}
	tests := []struct {
		name         string
		config       *internalv1alpha1.NodeConfig
		wantUpToDate int32
		wantUpdating bool
		wantWaiting  bool
	}{
		{
			name:         "applied the current generation",
			config:       makeNodeConfig("n1", 3, 3, appliedAt(3)),
			wantUpToDate: 1,
		},
		{
			name:         "applied condition stamped with an older generation",
			config:       makeNodeConfig("n1", 3, 3, appliedAt(2)),
			wantUpToDate: 0,
		},
		{
			name:         "rejected the current generation",
			config:       makeNodeConfig("n1", 3, 3, metav1.Condition{Type: "ConfigurationApplied", Status: metav1.ConditionFalse, Reason: "Rejected", ObservedGeneration: 3}),
			wantUpToDate: 0,
		},
		{
			name:         "behind the current generation",
			config:       makeNodeConfig("n1", 3, 2, appliedAt(2)),
			wantUpdating: true,
		},
		{
			name: "holds the current generation for approval",
			config: makeNodeConfig("n1", 3, 2,
				metav1.Condition{Type: "ConfigurationApplied", Status: metav1.ConditionFalse, Reason: "DisruptionApprovalPending", ObservedGeneration: 3},
				metav1.Condition{Type: "DisruptionApproved", Status: metav1.ConditionFalse, Reason: "ApprovalPending", ObservedGeneration: 3}),
			wantUpdating: true,
			wantWaiting:  true,
		},
		{
			name: "no NodeConfig yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The checksum annotation matches: an Immutable group must not count it.
			objs := []runtime.Object{
				makeChecksumSecret(map[string]string{"worker": "abc"}),
				makeNode("n1", "worker", true, "abc"),
			}
			if tt.config != nil {
				objs = append(objs, tt.config)
			}
			cl := fake.NewClientBuilder().WithScheme(newScheme(t)).WithRuntimeObjects(objs...).Build()
			ng := &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec:       v1.NodeGroupSpec{SystemType: v1.SystemTypeImmutable},
			}

			res, err := (&Service{Client: cl}).Compute(context.Background(), ng)
			if err != nil {
				t.Fatalf("Compute: %v", err)
			}
			if res.UpToDateCount != tt.wantUpToDate {
				t.Errorf("UpToDateCount = %d, want %d", res.UpToDateCount, tt.wantUpToDate)
			}
			node := res.NodesForConditions[0]
			if node.Updating != tt.wantUpdating {
				t.Errorf("Updating = %v, want %v", node.Updating, tt.wantUpdating)
			}
			if node.WaitingDisruptiveApproval != tt.wantWaiting {
				t.Errorf("WaitingDisruptiveApproval = %v, want %v", node.WaitingDisruptiveApproval, tt.wantWaiting)
			}
		})
	}
}

// The agent republishes its status on every pass; only a change of what Compute reads may
// wake the group status, whose pass lists every Machine in the cluster.
func TestNodeConfigChanged(t *testing.T) {
	appliedAt := func(generation int64) metav1.Condition {
		return metav1.Condition{Type: "ConfigurationApplied", Status: metav1.ConditionTrue, ObservedGeneration: generation}
	}
	before := makeNodeConfig("n1", 3, 2, appliedAt(2))
	before.Labels = map[string]string{"node.deckhouse.io/group": "worker"}

	heartbeat := before.DeepCopy()
	heartbeat.Status.LastReconcileTime = metav1.Now()
	caughtUp := before.DeepCopy()
	caughtUp.Status.AppliedGeneration = 3
	caughtUp.Status.Conditions = []metav1.Condition{appliedAt(3)}
	moved := before.DeepCopy()
	moved.Labels = map[string]string{"node.deckhouse.io/group": "other"}

	tests := []struct {
		name  string
		after *internalv1alpha1.NodeConfig
		want  bool
	}{
		{name: "heartbeat", after: heartbeat, want: false},
		{name: "applied the current generation", after: caughtUp, want: true},
		{name: "moved to another group", after: moved, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NodeConfigChanged(event.UpdateEvent{ObjectOld: before, ObjectNew: tt.after}); got != tt.want {
				t.Fatalf("NodeConfigChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}
