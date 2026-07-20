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

package engine

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
	"github.com/deckhouse/node-controller/internal/controller/updateapproval/kubeclient"
)

func newProcessor(t *testing.T, objs ...client.Object) (Processor, client.Client) {
	t.Helper()
	t.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	p := Processor{
		Kube:     kubeclient.Client{Client: cl},
		Recorder: record.NewFakeRecorder(50),
	}
	return p, cl
}

func node(name string, annotations map[string]string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations}}
}

func nodeInfo(name string, mutate func(*ua.NodeInfo)) ua.NodeInfo {
	info := ua.NodeInfo{Name: name, NodeGroup: "worker"}
	if mutate != nil {
		mutate(&info)
	}
	return info
}

func getNodeAnnotations(t *testing.T, cl client.Client, name string) map[string]string {
	t.Helper()
	n := &corev1.Node{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: name}, n); err != nil {
		t.Fatalf("get node %s: %v", name, err)
	}
	return n.Annotations
}

func TestProcessUpdatedNodes(t *testing.T) {
	const ngChecksum = "abc"

	tests := []struct {
		name         string
		nodeInfo     ua.NodeInfo
		ngChecksum   string
		wantFinished bool
		// after a successful cleanup the named annotations must be gone.
		wantCleaned bool
	}{
		{
			name:         "not approved is skipped",
			nodeInfo:     nodeInfo("n1", func(i *ua.NodeInfo) { i.ConfigurationChecksum = ngChecksum; i.IsReady = true }),
			ngChecksum:   ngChecksum,
			wantFinished: false,
		},
		{
			name: "approved but checksum mismatch is skipped",
			nodeInfo: nodeInfo("n1", func(i *ua.NodeInfo) {
				i.IsApproved = true
				i.ConfigurationChecksum = "old"
				i.IsReady = true
			}),
			ngChecksum:   ngChecksum,
			wantFinished: false,
		},
		{
			name: "approved, matching checksum but not ready is skipped",
			nodeInfo: nodeInfo("n1", func(i *ua.NodeInfo) {
				i.IsApproved = true
				i.ConfigurationChecksum = ngChecksum
				i.IsReady = false
			}),
			ngChecksum:   ngChecksum,
			wantFinished: false,
		},
		{
			name: "approved, ready, matching checksum runs cleanup",
			nodeInfo: nodeInfo("n1", func(i *ua.NodeInfo) {
				i.IsApproved = true
				i.ConfigurationChecksum = ngChecksum
				i.IsReady = true
			}),
			ngChecksum:   ngChecksum,
			wantFinished: true,
			wantCleaned:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := node("n1", map[string]string{
				ua.ApprovedAnnotation:           "",
				ua.WaitingForApprovalAnnotation: "",
				ua.DisruptionRequiredAnnotation: "",
			})
			p, cl := newProcessor(t, n)
			ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}

			finished, err := p.ProcessUpdatedNodes(context.Background(), ng, []ua.NodeInfo{tt.nodeInfo}, tt.ngChecksum)
			if err != nil {
				t.Fatalf("ProcessUpdatedNodes: %v", err)
			}
			if finished != tt.wantFinished {
				t.Fatalf("finished = %v, want %v", finished, tt.wantFinished)
			}
			if tt.wantCleaned {
				ann := getNodeAnnotations(t, cl, "n1")
				if _, ok := ann[ua.ApprovedAnnotation]; ok {
					t.Fatalf("expected approved annotation cleaned, got %+v", ann)
				}
			}
		})
	}
}

func TestProcessUpdatedNodes_DrainedRemovesUnschedulable(t *testing.T) {
	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "n1",
			Annotations: map[string]string{ua.ApprovedAnnotation: "", ua.DrainedAnnotation: "bashible"},
		},
		Spec: corev1.NodeSpec{Unschedulable: true},
	}
	p, cl := newProcessor(t, n)
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}

	info := nodeInfo("n1", func(i *ua.NodeInfo) {
		i.IsApproved = true
		i.IsDrained = true
		i.IsReady = true
		i.IsUnschedulable = true
		i.ConfigurationChecksum = "abc"
	})

	finished, err := p.ProcessUpdatedNodes(context.Background(), ng, []ua.NodeInfo{info}, "abc")
	if err != nil {
		t.Fatalf("ProcessUpdatedNodes: %v", err)
	}
	if !finished {
		t.Fatal("expected finished = true")
	}
	updated := &corev1.Node{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "n1"}, updated); err != nil {
		t.Fatalf("get node: %v", err)
	}
	if updated.Spec.Unschedulable {
		t.Fatal("expected unschedulable to be removed for drained node")
	}
}

func TestApproveDisruptions(t *testing.T) {
	tests := []struct {
		name         string
		ng           *v1.NodeGroup
		nodeInfo     ua.NodeInfo
		wantFinished bool
		wantAnno     string // annotation expected to be set on node n1, "" to skip check
	}{
		{
			name: "not approved is skipped",
			ng:   &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}},
			nodeInfo: nodeInfo("n1", func(i *ua.NodeInfo) {
				i.IsDisruptionRequired = true
			}),
			wantFinished: false,
		},
		{
			name: "manual mode does not approve",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{
					ApprovalMode: v1.DisruptionApprovalModeManual,
				}},
			},
			nodeInfo: nodeInfo("n1", func(i *ua.NodeInfo) {
				i.IsApproved = true
				i.IsDisruptionRequired = true
			}),
			wantFinished: false,
		},
		{
			name: "automatic, no drain needed, approves disruption",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{
					ApprovalMode: v1.DisruptionApprovalModeAutomatic,
					Automatic:    &v1.AutomaticDisruptionSpec{DrainBeforeApproval: ptr(false)},
				}},
			},
			nodeInfo: nodeInfo("n1", func(i *ua.NodeInfo) {
				i.IsApproved = true
				i.IsDisruptionRequired = true
			}),
			wantFinished: true,
			wantAnno:     ua.DisruptionApprovedAnnotation,
		},
		{
			name: "automatic, drain needed and node schedulable, starts draining",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Status:     v1.NodeGroupStatus{Ready: 3, Nodes: 3},
				Spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{
					ApprovalMode: v1.DisruptionApprovalModeAutomatic,
					Automatic:    &v1.AutomaticDisruptionSpec{DrainBeforeApproval: ptr(true)},
				}},
			},
			nodeInfo: nodeInfo("n1", func(i *ua.NodeInfo) {
				i.IsApproved = true
				i.IsDisruptionRequired = true
			}),
			wantFinished: true,
			wantAnno:     ua.DrainingAnnotation,
		},
		{
			name: "automatic, drain needed but already drained, approves disruption",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Status:     v1.NodeGroupStatus{Ready: 3, Nodes: 3},
				Spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{
					ApprovalMode: v1.DisruptionApprovalModeAutomatic,
					Automatic:    &v1.AutomaticDisruptionSpec{DrainBeforeApproval: ptr(true)},
				}},
			},
			nodeInfo: nodeInfo("n1", func(i *ua.NodeInfo) {
				i.IsApproved = true
				i.IsDisruptionRequired = true
				i.IsDrained = true
			}),
			wantFinished: true,
			wantAnno:     ua.DisruptionApprovedAnnotation,
		},
		{
			name: "automatic, drain needed, node already unschedulable, no further action",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Status:     v1.NodeGroupStatus{Ready: 3, Nodes: 3},
				Spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{
					ApprovalMode: v1.DisruptionApprovalModeAutomatic,
					Automatic:    &v1.AutomaticDisruptionSpec{DrainBeforeApproval: ptr(true)},
				}},
			},
			nodeInfo: nodeInfo("n1", func(i *ua.NodeInfo) {
				i.IsApproved = true
				i.IsDisruptionRequired = true
				i.IsUnschedulable = true
			}),
			wantFinished: false,
		},
		{
			name: "automatic, outside maintenance window is skipped",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{
					ApprovalMode: v1.DisruptionApprovalModeAutomatic,
					Automatic: &v1.AutomaticDisruptionSpec{
						DrainBeforeApproval: ptr(false),
						Windows:             []v1.DisruptionWindow{{From: "00:00", To: "01:00"}},
					},
				}},
			},
			nodeInfo: nodeInfo("n1", func(i *ua.NodeInfo) {
				i.IsApproved = true
				i.IsDisruptionRequired = true
			}),
			wantFinished: false,
		},
		{
			name: "rolling update outside window is skipped",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{
					ApprovalMode: v1.DisruptionApprovalModeRollingUpdate,
					RollingUpdate: &v1.RollingUpdateDisruptionSpec{
						Windows: []v1.DisruptionWindow{{From: "00:00", To: "01:00"}},
					},
				}},
			},
			nodeInfo: nodeInfo("n1", func(i *ua.NodeInfo) {
				i.IsApproved = true
				i.IsRollingUpdate = true
			}),
			wantFinished: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := node("n1", map[string]string{ua.DisruptionRequiredAnnotation: ""})
			p, cl := newProcessor(t, n)

			finished, err := p.ApproveDisruptions(context.Background(), tt.ng, []ua.NodeInfo{tt.nodeInfo})
			if err != nil {
				t.Fatalf("ApproveDisruptions: %v", err)
			}
			if finished != tt.wantFinished {
				t.Fatalf("finished = %v, want %v", finished, tt.wantFinished)
			}
			if tt.wantAnno != "" {
				ann := getNodeAnnotations(t, cl, "n1")
				if _, ok := ann[tt.wantAnno]; !ok {
					t.Fatalf("expected annotation %q to be set, got %+v", tt.wantAnno, ann)
				}
			}
		})
	}
}

func TestApproveDisruptions_RollingUpdateDeletesInstance(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	t.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")

	instance := &unstructured.Unstructured{}
	instance.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "Instance"})
	instance.SetName("n1")

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
	p := Processor{Kube: kubeclient.Client{Client: cl}, Recorder: record.NewFakeRecorder(10)}

	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{
			ApprovalMode: v1.DisruptionApprovalModeRollingUpdate,
		}},
	}
	info := nodeInfo("n1", func(i *ua.NodeInfo) {
		i.IsApproved = true
		i.IsRollingUpdate = true
	})

	finished, err := p.ApproveDisruptions(context.Background(), ng, []ua.NodeInfo{info})
	if err != nil {
		t.Fatalf("ApproveDisruptions: %v", err)
	}
	if !finished {
		t.Fatal("expected finished = true for rolling update")
	}
	remaining := &unstructured.Unstructured{}
	remaining.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "Instance"})
	err = cl.Get(context.Background(), types.NamespacedName{Name: "n1"}, remaining)
	if err == nil {
		t.Fatal("expected instance to be deleted")
	}
}

// Exercises the fallback loop's skip conditions: with capacity for 2, a ready node
// (skipped by IsReady), a ready non-waiting node (skipped by !IsWaitingForApproval),
// and a not-ready waiting node that the fallback actually approves. Because not all
// nodes are ready, the ready-batch is skipped and the fallback loop drives approval.
func TestApproveUpdates_FallbackSkipsReadyAndNonWaiting(t *testing.T) {
	maxConcurrent2 := intstr.FromInt32(2)
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{Update: &v1.UpdateSpec{MaxConcurrent: &maxConcurrent2}},
	}
	nodes := []ua.NodeInfo{
		nodeInfo("n1", func(i *ua.NodeInfo) { i.IsWaitingForApproval = true; i.IsReady = true }),
		nodeInfo("n2", func(i *ua.NodeInfo) { i.IsReady = true }),
		nodeInfo("n3", func(i *ua.NodeInfo) { i.IsWaitingForApproval = true; i.IsReady = false }),
	}
	objs := []client.Object{
		node("n1", map[string]string{ua.WaitingForApprovalAnnotation: ""}),
		node("n2", map[string]string{}),
		node("n3", map[string]string{ua.WaitingForApprovalAnnotation: ""}),
	}
	p, cl := newProcessor(t, objs...)

	finished, err := p.ApproveUpdates(context.Background(), ng, nodes)
	if err != nil {
		t.Fatalf("ApproveUpdates: %v", err)
	}
	if !finished {
		t.Fatal("expected finished = true")
	}
	// n3 is the only not-ready waiting node: approved through the fallback loop.
	if _, ok := getNodeAnnotations(t, cl, "n3")[ua.ApprovedAnnotation]; !ok {
		t.Fatal("expected n3 approved via fallback")
	}
	// n1 is ready but the ready-batch was skipped (not all nodes ready), and the
	// fallback loop skips ready nodes, so n1 stays unapproved.
	if _, ok := getNodeAnnotations(t, cl, "n1")[ua.ApprovedAnnotation]; ok {
		t.Fatal("expected n1 to remain unapproved (ready node skipped by fallback)")
	}
}

func TestApproveUpdates_NoEligibleNodesReturnsFalse(t *testing.T) {
	// Capacity exists and a node claims waiting, but it is ready while another node
	// is not ready: the ready-batch is skipped because not all nodes are ready, and
	// the fallback loop only approves not-ready waiting nodes. The single waiting node
	// here is ready, so neither loop selects it, leaving approvedNodes empty.
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}
	nodes := []ua.NodeInfo{
		nodeInfo("n1", func(i *ua.NodeInfo) { i.IsWaitingForApproval = true; i.IsReady = true }),
		nodeInfo("n2", func(i *ua.NodeInfo) { i.IsReady = false }),
	}
	objs := []client.Object{
		node("n1", map[string]string{ua.WaitingForApprovalAnnotation: ""}),
		node("n2", map[string]string{}),
	}
	p, cl := newProcessor(t, objs...)

	finished, err := p.ApproveUpdates(context.Background(), ng, nodes)
	if err != nil {
		t.Fatalf("ApproveUpdates: %v", err)
	}
	if finished {
		t.Fatal("expected finished = false when no node is eligible")
	}
	if _, ok := getNodeAnnotations(t, cl, "n1")[ua.ApprovedAnnotation]; ok {
		t.Fatal("expected n1 to remain unapproved")
	}
}

func TestApproveUpdates(t *testing.T) {
	maxConcurrent2 := intstr.FromInt32(2)

	tests := []struct {
		name         string
		ng           *v1.NodeGroup
		nodes        []ua.NodeInfo
		wantFinished bool
		wantApproved []string
	}{
		{
			name: "no waiting nodes does nothing",
			ng:   &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}},
			nodes: []ua.NodeInfo{
				nodeInfo("n1", func(i *ua.NodeInfo) { i.IsReady = true }),
			},
			wantFinished: false,
		},
		{
			name: "already at concurrency limit does nothing",
			ng:   &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}},
			nodes: []ua.NodeInfo{
				nodeInfo("n1", func(i *ua.NodeInfo) { i.IsApproved = true; i.IsReady = true }),
				nodeInfo("n2", func(i *ua.NodeInfo) { i.IsWaitingForApproval = true; i.IsReady = true }),
			},
			wantFinished: false,
		},
		{
			name: "single ready waiting node gets approved",
			ng:   &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}},
			nodes: []ua.NodeInfo{
				nodeInfo("n1", func(i *ua.NodeInfo) { i.IsWaitingForApproval = true; i.IsReady = true }),
			},
			wantFinished: true,
			wantApproved: []string{"n1"},
		},
		{
			name: "concurrency 2 approves up to 2 ready waiting nodes",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec:       v1.NodeGroupSpec{Update: &v1.UpdateSpec{MaxConcurrent: &maxConcurrent2}},
			},
			nodes: []ua.NodeInfo{
				nodeInfo("n1", func(i *ua.NodeInfo) { i.IsWaitingForApproval = true; i.IsReady = true }),
				nodeInfo("n2", func(i *ua.NodeInfo) { i.IsWaitingForApproval = true; i.IsReady = true }),
				nodeInfo("n3", func(i *ua.NodeInfo) { i.IsWaitingForApproval = true; i.IsReady = true }),
			},
			wantFinished: true,
			wantApproved: []string{"n1", "n2"},
		},
		{
			name: "not-ready waiting node is approved via fallback when not all ready",
			ng:   &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}},
			nodes: []ua.NodeInfo{
				nodeInfo("n1", func(i *ua.NodeInfo) { i.IsWaitingForApproval = true; i.IsReady = false }),
			},
			wantFinished: true,
			wantApproved: []string{"n1"},
		},
		{
			name: "cloud ephemeral scaling up skips ready-batch, falls back to not-ready node",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral},
				Status:     v1.NodeGroupStatus{Desired: 5, Ready: 3},
			},
			nodes: []ua.NodeInfo{
				nodeInfo("n1", func(i *ua.NodeInfo) { i.IsWaitingForApproval = true; i.IsReady = false }),
			},
			wantFinished: true,
			wantApproved: []string{"n1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := make([]client.Object, 0, len(tt.nodes))
			for _, ni := range tt.nodes {
				ann := map[string]string{}
				if ni.IsWaitingForApproval {
					ann[ua.WaitingForApprovalAnnotation] = ""
				}
				if ni.IsApproved {
					ann[ua.ApprovedAnnotation] = ""
				}
				objs = append(objs, node(ni.Name, ann))
			}
			p, cl := newProcessor(t, objs...)

			finished, err := p.ApproveUpdates(context.Background(), tt.ng, tt.nodes)
			if err != nil {
				t.Fatalf("ApproveUpdates: %v", err)
			}
			if finished != tt.wantFinished {
				t.Fatalf("finished = %v, want %v", finished, tt.wantFinished)
			}
			for _, name := range tt.wantApproved {
				ann := getNodeAnnotations(t, cl, name)
				if _, ok := ann[ua.ApprovedAnnotation]; !ok {
					t.Fatalf("expected node %s to be approved, got %+v", name, ann)
				}
				if _, ok := ann[ua.WaitingForApprovalAnnotation]; ok {
					t.Fatalf("expected node %s waiting annotation removed, got %+v", name, ann)
				}
			}
		})
	}
}

func TestNeedDrainNode(t *testing.T) {
	p := Processor{DeckhouseNodeName: "deckhouse-node"}

	tests := []struct {
		name string
		node *ua.NodeInfo
		ng   *v1.NodeGroup
		want bool
	}{
		{
			name: "single control-plane node is not drained",
			node: &ua.NodeInfo{Name: "master-0"},
			ng:   &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "master"}, Status: v1.NodeGroupStatus{Nodes: 1}},
			want: false,
		},
		{
			name: "deckhouse node in small group is not drained",
			node: &ua.NodeInfo{Name: "deckhouse-node"},
			ng:   &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Status: v1.NodeGroupStatus{Ready: 1}},
			want: false,
		},
		{
			name: "explicit DrainBeforeApproval false",
			node: &ua.NodeInfo{Name: "n1"},
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Status:     v1.NodeGroupStatus{Ready: 3},
				Spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{
					Automatic: &v1.AutomaticDisruptionSpec{DrainBeforeApproval: ptr(false)},
				}},
			},
			want: false,
		},
		{
			name: "explicit DrainBeforeApproval true",
			node: &ua.NodeInfo{Name: "n1"},
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Status:     v1.NodeGroupStatus{Ready: 3},
				Spec: v1.NodeGroupSpec{Disruptions: &v1.DisruptionsSpec{
					Automatic: &v1.AutomaticDisruptionSpec{DrainBeforeApproval: ptr(true)},
				}},
			},
			want: true,
		},
		{
			name: "default drains when nothing specified",
			node: &ua.NodeInfo{Name: "n1"},
			ng:   &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Status: v1.NodeGroupStatus{Ready: 3}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.NeedDrainNode(context.Background(), tt.node, tt.ng); got != tt.want {
				t.Fatalf("NeedDrainNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
