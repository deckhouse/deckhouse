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

package kubeclient

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"fencing-agent/internal/domain"
)

func node(name, nodeGroup string, addresses ...corev1.NodeAddress) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			UID:    types.UID("uid-" + name),
			Labels: map[string]string{domain.NodeGroupLabel: nodeGroup},
		},
		Status: corev1.NodeStatus{Addresses: addresses},
	}
}

func internal(ip string) corev1.NodeAddress {
	return corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: ip}
}

func objects(nodes ...*corev1.Node) []runtime.Object {
	out := make([]runtime.Object, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n)
	}

	return out
}

func TestListNodeGroupSelectsOwnGroupOnly(t *testing.T) {
	client := fake.NewSimpleClientset(objects(
		node("worker-1", "worker", internal("10.0.0.1")),
		node("worker-2", "worker", internal("10.0.0.2")),
		node("master-1", "master", internal("10.0.0.3")),
	)...)

	peers, err := NewNodes(client).ListNodeGroup(t.Context(), "worker")
	if err != nil {
		t.Fatalf("list node group: %v", err)
	}

	if len(peers) != 2 {
		t.Fatalf("expected 2 nodes of the worker group, got %v", peers)
	}

	for _, peer := range peers {
		if peer.Name == "master-1" {
			t.Errorf("node of another node group leaked into the result: %v", peers)
		}
	}
}

func TestListNodeGroupReportsMissingInternalIP(t *testing.T) {
	client := fake.NewSimpleClientset(objects(
		node("worker-1", "worker", corev1.NodeAddress{Type: corev1.NodeHostName, Address: "worker-1"}),
	)...)

	peers, err := NewNodes(client).ListNodeGroup(t.Context(), "worker")
	if err != nil {
		t.Fatalf("list node group: %v", err)
	}

	if len(peers) != 1 || peers[0].IP != "" {
		t.Errorf("expected a single node with an empty IP, got %v", peers)
	}
}

func TestResolveIdentityRequiresInternalIP(t *testing.T) {
	client := fake.NewSimpleClientset(objects(
		node("worker-1", "worker", corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: "1.2.3.4"}),
	)...)

	if _, err := ResolveIdentity(t.Context(), client, "worker-1"); err == nil {
		t.Error("a node without InternalIP must be rejected: it would advertise the pod IP")
	}
}

func TestResolveIdentityReturnsInternalIP(t *testing.T) {
	client := fake.NewSimpleClientset(objects(
		node("worker-1", "worker",
			corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: "1.2.3.4"},
			internal("10.0.0.1"),
		),
	)...)

	identity, err := ResolveIdentity(t.Context(), client, "worker-1")
	if err != nil {
		t.Fatalf("resolve identity: %v", err)
	}

	if identity.Name != "worker-1" || identity.UID != "uid-worker-1" || identity.IP != "10.0.0.1" {
		t.Errorf("unexpected identity %+v", identity)
	}
}
