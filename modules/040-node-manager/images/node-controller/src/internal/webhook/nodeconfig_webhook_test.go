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
