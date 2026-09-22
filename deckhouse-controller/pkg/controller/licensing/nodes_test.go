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
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
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

func taintValue(key, value string) corev1.Taint {
	return corev1.Taint{Key: key, Value: value, Effect: corev1.TaintEffectNoSchedule}
}

// N1 to N11 of specification 15.1: which nodes are free and what the licensable
// ones add up to.
func TestClassify(t *testing.T) {
	never := func(string) (bool, error) { return false, nil }
	always := func(string) (bool, error) { return true, nil }

	cases := []struct {
		name     string
		nodes    []corev1.Node
		userPods func(string) (bool, error)
		free     []string
		servers  int64
		vcpu     int64
		cores    int64
	}{
		{
			name:     "N1 control-plane without user pods",
			nodes:    []corev1.Node{node("master", "8", true, taint(taintControlPlane))},
			userPods: never,
			free:     []string{"master"},
		},
		{
			name:     "N2 control-plane running a user pod",
			nodes:    []corev1.Node{node("master", "8", true, taint(taintControlPlane))},
			userPods: always,
			servers:  1, vcpu: 8, cores: 4,
		},
		{
			name:     "the legacy master taint is recognised too",
			nodes:    []corev1.Node{node("master", "8", true, taint(taintMaster))},
			userPods: never,
			free:     []string{"master"},
		},
		{
			name:     "N3 dedicated system",
			nodes:    []corev1.Node{node("system", "16", true, taintValue(taintDedicated, "system"))},
			userPods: never,
			free:     []string{"system"},
		},
		{
			name:     "N4 dedicated monitoring",
			nodes:    []corev1.Node{node("monitoring", "16", true, taintValue(taintDedicated, "monitoring"))},
			userPods: never,
			free:     []string{"monitoring"},
		},
		{
			name:     "N5 dedicated frontend is licensed: it carries customer traffic",
			nodes:    []corev1.Node{node("front", "16", true, taintValue(taintDedicated, "frontend"))},
			userPods: never,
			servers:  1, vcpu: 16, cores: 8,
		},
		{
			name:     "a dedicated node with no value at all is licensed",
			nodes:    []corev1.Node{node("other", "16", true, taint(taintDedicated))},
			userPods: never,
			servers:  1, vcpu: 16, cores: 8,
		},
		{
			name:     "N6 dedicated system running a user pod",
			nodes:    []corev1.Node{node("system", "16", true, taintValue(taintDedicated, "system"))},
			userPods: always,
			servers:  1, vcpu: 16, cores: 8,
		},
		{
			name:     "an arbitrary taint does not reserve a worker",
			nodes:    []corev1.Node{node("worker", "4", true, taint("example.com/gpu"))},
			userPods: never,
			servers:  1, vcpu: 4, cores: 2,
		},
		{
			name:     "N8 a NotReady worker still counts, capacity is what is licensed",
			nodes:    []corev1.Node{node("worker", "4", false)},
			userPods: never,
			servers:  1, vcpu: 4, cores: 2,
		},
		{
			name:     "N9 fractional capacity rounds up",
			nodes:    []corev1.Node{node("worker", "3500m", true)},
			userPods: never,
			servers:  1, vcpu: 4, cores: 2,
		},
		{
			name:     "N11 no licensable nodes at all",
			nodes:    []corev1.Node{node("master", "8", true, taint(taintControlPlane))},
			userPods: never,
			free:     []string{"master"},
		},
		{
			name: "every metric comes from the same node set",
			nodes: []corev1.Node{
				node("master", "8", true, taint(taintControlPlane)),
				node("worker-1", "4", true),
				node("worker-2", "2", true),
			},
			userPods: never,
			free:     []string{"master"},
			servers:  2, vcpu: 6, cores: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed, err := classify(tc.nodes, tc.userPods)
			if err != nil {
				t.Fatalf("classify: %v", err)
			}

			var free []string
			for _, n := range observed {
				if n.Free {
					if n.Reason == "" {
						t.Fatalf("free node %s carries no reason", n.Name)
					}
					free = append(free, n.Name)
				}
			}
			if !reflect.DeepEqual(free, tc.free) {
				t.Fatalf("free = %v, want %v", free, tc.free)
			}

			got := licensing.Consumption(licensable(observed))
			want := map[string]int64{
				licensing.MetricServers: tc.servers,
				licensing.MetricVCPU:    tc.vcpu,
				licensing.MetricCores:   tc.cores,
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("consumption = %v, want %v", got, want)
			}
		})
	}
}

// N10: cores are taken on the sum, not per node.
func TestCoresAreTakenOnTheSum(t *testing.T) {
	nodes := make([]corev1.Node, 0, 24)
	for i := range 24 {
		nodes = append(nodes, node(fmt.Sprintf("worker-%02d", i), "3", true))
	}

	observed, err := classify(nodes, func(string) (bool, error) { return false, nil })
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	got := licensing.Consumption(licensable(observed))
	if got[licensing.MetricVCPU] != 72 || got[licensing.MetricCores] != 36 {
		t.Fatalf("consumption = %v, want 72 vCPU and 36 cores, not 48", got)
	}
}

// N12: an unreadable pod list aborts the observation rather than quietly
// dropping the node, which would understate consumption for good.
func TestClassifyFailsWhenPodsAreUnreadable(t *testing.T) {
	boom := errors.New("apiserver is unhappy")
	nodes := []corev1.Node{
		node("worker", "4", true),
		node("system", "16", true, taintValue(taintDedicated, "system")),
	}

	if _, err := classify(nodes, func(string) (bool, error) { return false, boom }); !errors.Is(err, boom) {
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
			name: "a pod in an application namespace is user workload",
			pod:  corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api"}},
			want: true,
		},
		{
			name: "a pod in a d8 namespace is platform workload",
			pod:  corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "d8-system", Name: "deckhouse"}},
			want: false,
		},
		{
			name: "a pod in a kube namespace is platform workload",
			pod:  corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "kube-proxy"}},
			want: false,
		},
		{
			name: "a completed job pod is not running workload",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "import"},
				Status:     corev1.PodStatus{Phase: corev1.PodSucceeded},
			},
			want: false,
		},
		{
			name: "a failed pod is not running workload",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "import"},
				Status:     corev1.PodStatus{Phase: corev1.PodFailed},
			},
			want: false,
		},
		{
			name: "a running pod in an application namespace is user workload",
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
