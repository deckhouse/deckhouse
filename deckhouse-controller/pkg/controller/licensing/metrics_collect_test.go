// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package licensing

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func node(name string, cores string, ready bool, taints ...corev1.Taint) corev1.Node {
	n := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{Taints: taints},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cores)},
		},
	}
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	n.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}}
	return n
}

func taint(key string) corev1.Taint {
	return corev1.Taint{Key: key, Effect: corev1.TaintEffectNoSchedule}
}

func TestCountNodes(t *testing.T) {
	never := func(string) (bool, error) { return false, nil }
	always := func(string) (bool, error) { return true, nil }

	cases := []struct {
		name      string
		nodes     []corev1.Node
		userPods  func(string) (bool, error)
		wantVCPU  int64
		wantNodes int64
	}{
		{
			name:      "a control plane node with its taint in place does not count",
			nodes:     []corev1.Node{node("master", "8", true, taint("node-role.kubernetes.io/control-plane"))},
			userPods:  never,
			wantVCPU:  0,
			wantNodes: 0,
		},
		{
			name:      "the legacy master taint is recognized too",
			nodes:     []corev1.Node{node("master", "8", true, taint("node-role.kubernetes.io/master"))},
			userPods:  never,
			wantVCPU:  0,
			wantNodes: 0,
		},
		{
			name:      "a dedicated node running a user pod counts",
			nodes:     []corev1.Node{node("system", "16", true, taint("dedicated.deckhouse.io"))},
			userPods:  always,
			wantVCPU:  16,
			wantNodes: 1,
		},
		{
			name:      "an arbitrary taint does not reserve a worker",
			nodes:     []corev1.Node{node("worker", "4", true, taint("example.com/gpu"))},
			userPods:  never,
			wantVCPU:  4,
			wantNodes: 1,
		},
		{
			name:      "a NotReady worker still counts, capacity is what is licensed",
			nodes:     []corev1.Node{node("worker", "4", false)},
			userPods:  never,
			wantVCPU:  4,
			wantNodes: 1,
		},
		{
			name:      "fractional capacity rounds up to whole cores",
			nodes:     []corev1.Node{node("worker", "3500m", true)},
			userPods:  never,
			wantVCPU:  4,
			wantNodes: 1,
		},
		{
			name: "both metrics are derived from the same node set",
			nodes: []corev1.Node{
				node("master", "8", true, taint("node-role.kubernetes.io/control-plane")),
				node("worker-1", "4", true),
				node("worker-2", "2", true),
			},
			userPods:  never,
			wantVCPU:  6,
			wantNodes: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vcpu, count, err := countNodes(tc.nodes, tc.userPods)
			if err != nil {
				t.Fatalf("countNodes: %v", err)
			}
			if vcpu != tc.wantVCPU || count != tc.wantNodes {
				t.Fatalf("vCPU/nodes = %d/%d, want %d/%d", vcpu, count, tc.wantVCPU, tc.wantNodes)
			}
		})
	}
}

// An unreadable pod list must abort the count rather than quietly drop the node:
// the missing capacity would otherwise enter the seven day average as a real
// reduction in consumption.
func TestCountNodesFailsWhenPodsAreUnreadable(t *testing.T) {
	boom := errors.New("apiserver is unhappy")
	nodes := []corev1.Node{
		node("worker", "4", true),
		node("system", "16", true, taint("dedicated.deckhouse.io")),
	}

	_, _, err := countNodes(nodes, func(string) (bool, error) { return false, boom })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestIsUserPod(t *testing.T) {
	cases := []struct {
		name string
		pod  corev1.Pod
		want bool
	}{
		{
			name: "a pod in an application namespace is user platform",
			pod:  corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api"}},
			want: true,
		},
		{
			name: "a pod in a d8 namespace is platform platform",
			pod:  corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "d8-system", Name: "deckhouse"}},
			want: false,
		},
		{
			name: "a pod in a kube namespace is platform platform",
			pod:  corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "kube-proxy"}},
			want: false,
		},
		{
			name: "a completed job pod is not running platform",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "import"},
				Status:     corev1.PodStatus{Phase: corev1.PodSucceeded},
			},
			want: false,
		},
		{
			name: "a failed pod is not running platform",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "import"},
				Status:     corev1.PodStatus{Phase: corev1.PodFailed},
			},
			want: false,
		},
		{
			name: "a running pod in an application namespace is user platform",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api"},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			},
			want: true,
		},
		{
			name: "a DaemonSet pod says nothing about the node",
			pod: corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace:       "monitoring",
				Name:            "agent",
				OwnerReferences: []metav1.OwnerReference{{Kind: "DaemonSet", Name: "agent"}},
			}},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUserPod(tc.pod); got != tc.want {
				t.Fatalf("isUserPod = %v, want %v", got, tc.want)
			}
		})
	}
}
