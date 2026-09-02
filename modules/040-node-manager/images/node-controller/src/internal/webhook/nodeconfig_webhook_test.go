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

package webhook

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

func nodeConfigFixture() *internalv1alpha1.NodeConfig {
	return &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
		Spec: internalv1alpha1.NodeSpec{
			NodeName: "worker-0",
			Network: internalv1alpha1.Network{
				Hostname: "worker-0",
				Interfaces: []internalv1alpha1.NetworkInterface{{
					Name:      "eth0",
					Addresses: []string{"10.0.0.10/24"},
					Gateway:   "10.0.0.1",
				}},
			},
			Storage: internalv1alpha1.Storage{
				Disk: internalv1alpha1.Disk{Device: "/dev/sda"},
			},
			Kubelet: internalv1alpha1.Kubelet{MaxPods: 110},
		},
	}
}

func TestValidateNodeConfigUpdate(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(nc *internalv1alpha1.NodeConfig)
		wantDenied  bool
		wantMessage string
	}{
		{
			name: "network change is refused",
			mutate: func(nc *internalv1alpha1.NodeConfig) {
				nc.Spec.Network.Interfaces[0].Addresses = []string{"10.0.0.11/24"}
			},
			wantDenied:  true,
			wantMessage: "spec.network is written on the machine",
		},
		{
			name: "storage change is refused",
			mutate: func(nc *internalv1alpha1.NodeConfig) {
				nc.Spec.Storage.Device = "/dev/sdb"
			},
			wantDenied:  true,
			wantMessage: "spec.storage names the disk",
		},
		{
			// Wiping re-partitions the disk the node runs from; on a machine
			// with a single disk that destroys the installation media.
			name: "wiping the installed disk is refused",
			mutate: func(nc *internalv1alpha1.NodeConfig) {
				nc.Spec.Storage.Wipe = true
			},
			wantDenied:  true,
			wantMessage: "spec.storage names the disk",
		},
		{
			name: "cluster-owned field stays writable",
			mutate: func(nc *internalv1alpha1.NodeConfig) {
				nc.Spec.Kubelet.MaxPods = 250
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := nodeConfigFixture()
			updated := nodeConfigFixture()
			tt.mutate(updated)

			err := validateNodeConfigUpdate(old, updated)
			if !tt.wantDenied {
				if err != nil {
					t.Fatalf("expected the update to pass, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected the update to be refused, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("expected the error to mention %q, got %q", tt.wantMessage, err.Error())
			}
		})
	}
}

// The controller re-renders the whole spec on every pass. Two fields it always
// rewrites — the hostname, which must match the Node name, and the disk
// selector, which it keeps renderable on purpose — must stay writable, or the
// first pass after registration deadlocks the node's config for good.
func TestValidateNodeConfigUpdateLeavesTheControllerRoomToWork(t *testing.T) {
	old := &internalv1alpha1.NodeConfig{Spec: internalv1alpha1.NodeSpec{
		Network: internalv1alpha1.Network{
			Hostname:   "what-the-machine-said",
			Interfaces: []internalv1alpha1.NetworkInterface{{Name: "bond0"}},
		},
		Storage: internalv1alpha1.Storage{
			Disk: internalv1alpha1.Disk{DiskSelector: &internalv1alpha1.DiskSelector{Size: ">=20Gi"}},
		},
	}}

	rendered := old.DeepCopy()
	rendered.Spec.Network.Hostname = "master-0"
	rendered.Spec.Storage.DiskSelector = nil

	if err := validateNodeConfigUpdate(old, rendered); err != nil {
		t.Fatalf("the webhook froze a field the controller renders, which stops every later update: %v", err)
	}
}

// The window this closes: a NodeConfig reaches the cluster with no network yet
// (the NodeConfigTemplate shape, or a machine that named none) and an operator
// types addressing into it. The controller then carries that over for good and
// this webhook refuses every correction, so the node boots with no address.
func TestValidateNodeConfigUpdateRefusesANetworkTheMachineNeverMade(t *testing.T) {
	tests := []struct {
		name    string
		network internalv1alpha1.Network
	}{
		{
			name: "an address the machine never reported",
			network: internalv1alpha1.Network{
				Hostname: "worker-0",
				Interfaces: []internalv1alpha1.NetworkInterface{{
					Name: "enp1s0", Addresses: []string{"10.0.0.5/24"}, Gateway: "10.0.0.1",
				}},
			},
		},
		{
			// Same catastrophe with no address in sight: the machine's NIC is
			// eth0, so a DHCP interface under another name configures nothing.
			name: "a NIC name only the machine knows",
			network: internalv1alpha1.Network{
				Hostname:   "worker-0",
				Interfaces: []internalv1alpha1.NetworkInterface{{Name: "enp1s0", DHCP: true}},
			},
		},
		{
			name: "resolvers and routes the machine never reported",
			network: internalv1alpha1.Network{
				Hostname:   "worker-0",
				Interfaces: []internalv1alpha1.NetworkInterface{{Name: "eth0", DHCP: true}},
				DNS:        internalv1alpha1.DNS{Servers: []string{"10.0.0.53"}},
				Routes:     []internalv1alpha1.Route{{Networks: []string{"10.1.0.0/16"}, Gateway: "10.0.0.1"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := &internalv1alpha1.NodeConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
				Spec:       internalv1alpha1.NodeSpec{NodeName: "worker-0"},
			}
			typedIn := old.DeepCopy()
			typedIn.Spec.Network = tt.network

			err := validateNodeConfigUpdate(old, typedIn)
			if err == nil {
				t.Fatal("the webhook let a network the machine never made into a NodeConfig that had none")
			}
			if !strings.Contains(err.Error(), "spec.network is written on the machine") {
				t.Fatalf("expected the error to mention spec.network, got %q", err.Error())
			}
		})
	}
}

// An update that touches something else entirely must still pass on a NodeConfig
// that names no network: only the network itself is guarded here.
func TestValidateNodeConfigUpdateLeavesANetworklessConfigWritable(t *testing.T) {
	old := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
		Spec:       internalv1alpha1.NodeSpec{NodeName: "worker-0"},
	}

	updated := old.DeepCopy()
	updated.Spec.Kubelet.MaxPods = 250

	if err := validateNodeConfigUpdate(old, updated); err != nil {
		t.Fatalf("the webhook froze a config that named no network: %v", err)
	}
}

// A NodeConfig that named no network carries nothing the machine owns, and the
// controller renders eth0/DHCP into it on its first pass — the shape the
// NodeConfigTemplate hands out and the shape a DHCP machine publishes.
func TestValidateNodeConfigUpdateFillsInANetworkTheMachineNeverNamed(t *testing.T) {
	old := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
		Spec:       internalv1alpha1.NodeSpec{NodeName: "worker-0"},
	}

	rendered := old.DeepCopy()
	rendered.Spec.Network = internalv1alpha1.Network{
		Hostname:   "worker-0",
		Interfaces: []internalv1alpha1.NetworkInterface{{Name: "eth0", DHCP: true}},
	}

	if err := validateNodeConfigUpdate(old, rendered); err != nil {
		t.Fatalf("the webhook refused the render of a node that named no network, which stops every later update: %v", err)
	}
}

// controllerRender reproduces the network keepBootstrapOnlyFields writes back
// (internal/controller/nodeconfig/render.go): what the machine published, with
// the rendered hostname and, for a machine that named no NIC, the rendered eth0.
func controllerRender(published internalv1alpha1.Network, nodeName string) internalv1alpha1.Network {
	rendered := *published.DeepCopy()
	rendered.Hostname = nodeName
	if len(rendered.Interfaces) == 0 {
		rendered.Interfaces = []internalv1alpha1.NetworkInterface{{Name: "eth0", DHCP: true}}
	}
	return rendered
}

// A machine can publish resolvers, time servers or routes and leave the NIC to
// DHCP. The controller then fills the NIC in on every pass, so a carve-out that
// only knows the wholly empty network denies that node's config for good.
func TestValidateNodeConfigUpdateAcceptsTheRenderOverAnyPublishedNetwork(t *testing.T) {
	tests := []struct {
		name      string
		published internalv1alpha1.Network
	}{
		{
			name:      "nothing at all",
			published: internalv1alpha1.Network{},
		},
		{
			name:      "the hostname it booted with",
			published: internalv1alpha1.Network{Hostname: "what-the-machine-said"},
		},
		{
			name:      "resolvers and nothing else",
			published: internalv1alpha1.Network{DNS: internalv1alpha1.DNS{Servers: []string{"10.0.0.53"}, Search: []string{"lan"}}},
		},
		{
			name:      "time servers and nothing else",
			published: internalv1alpha1.Network{NTP: internalv1alpha1.NTP{Servers: []string{"10.0.0.123"}}},
		},
		{
			name:      "routes and nothing else",
			published: internalv1alpha1.Network{Routes: []internalv1alpha1.Route{{Networks: []string{"10.1.0.0/16"}, Gateway: "10.0.0.1"}}},
		},
		{
			name: "a NIC of its own, so nothing is filled in",
			published: internalv1alpha1.Network{
				DNS:        internalv1alpha1.DNS{Servers: []string{"10.0.0.53"}},
				Interfaces: []internalv1alpha1.NetworkInterface{{Name: "bond0", Addresses: []string{"192.0.2.10/24"}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := &internalv1alpha1.NodeConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
				Spec:       internalv1alpha1.NodeSpec{NodeName: "worker-0", Network: tt.published},
			}
			rendered := old.DeepCopy()
			rendered.Spec.Network = controllerRender(tt.published, "worker-0")

			if err := validateNodeConfigUpdate(old, rendered); err != nil {
				t.Fatalf("the webhook refused the controller's own render, which stops every later update: %v", err)
			}
		})
	}
}

// The carve-out may admit the rendered NIC and nothing else: on a machine that
// named none, an operator-typed NIC is carried over for good and leaves the node
// with no address after a reboot.
func TestValidateNodeConfigUpdateRefusesAnOperatorEditBesideTheRenderedNIC(t *testing.T) {
	published := internalv1alpha1.Network{DNS: internalv1alpha1.DNS{Servers: []string{"10.0.0.53"}}}

	tests := []struct {
		name  string
		typed func(n *internalv1alpha1.Network)
	}{
		{
			name:  "a NIC only the machine could name",
			typed: func(n *internalv1alpha1.Network) { n.Interfaces[0].Name = "enp1s0" },
		},
		{
			name: "an address on the rendered NIC",
			typed: func(n *internalv1alpha1.Network) {
				n.Interfaces[0].DHCP = false
				n.Interfaces[0].Addresses = []string{"10.0.0.5/24"}
			},
		},
		{
			name:  "a resolver added beside the render",
			typed: func(n *internalv1alpha1.Network) { n.DNS.Servers = append(n.DNS.Servers, "10.0.0.54") },
		},
		{
			name: "a route added beside the render",
			typed: func(n *internalv1alpha1.Network) {
				n.Routes = []internalv1alpha1.Route{{Networks: []string{"10.1.0.0/16"}, Gateway: "10.0.0.1"}}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := &internalv1alpha1.NodeConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
				Spec:       internalv1alpha1.NodeSpec{NodeName: "worker-0", Network: published},
			}
			typedIn := old.DeepCopy()
			typedIn.Spec.Network = controllerRender(published, "worker-0")
			tt.typed(&typedIn.Spec.Network)

			err := validateNodeConfigUpdate(old, typedIn)
			if err == nil {
				t.Fatal("the webhook let an operator edit through beside the render it carves out")
			}
			if !strings.Contains(err.Error(), "spec.network is written on the machine") {
				t.Fatalf("expected the error to mention spec.network, got %q", err.Error())
			}
		})
	}
}
