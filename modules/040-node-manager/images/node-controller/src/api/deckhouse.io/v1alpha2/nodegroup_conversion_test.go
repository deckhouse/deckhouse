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

package v1alpha2

import (
	"testing"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestNodeGroupConversion_PreservesSeccompDefault(t *testing.T) {
	src := &NodeGroup{
		Spec: NodeGroupSpec{
			NodeType: NodeTypeCloud,
			Kubelet: &KubeletSpec{
				SeccompDefault: boolPtr(true),
			},
		},
	}

	dst := &v1.NodeGroup{}
	if err := src.ConvertTo(dst); err != nil {
		t.Fatalf("convert to v1 failed: %v", err)
	}

	if dst.Spec.Kubelet == nil || dst.Spec.Kubelet.SeccompDefault == nil || !*dst.Spec.Kubelet.SeccompDefault {
		t.Fatalf("seccompDefault was not converted to v1")
	}

	back := &NodeGroup{}
	if err := back.ConvertFrom(dst); err != nil {
		t.Fatalf("convert from v1 failed: %v", err)
	}

	if back.Spec.Kubelet == nil || back.Spec.Kubelet.SeccompDefault == nil || !*back.Spec.Kubelet.SeccompDefault {
		t.Fatalf("seccompDefault was not preserved after round-trip conversion")
	}
}

// A write issued at v1alpha2 round-trips through the hub, so a systemType the
// conversion drops is stored as Mutable — flipping an immutable group's
// MachineDeployments to bashible bootstrap and re-creating every machine.
func TestNodeGroupConversion_PreservesSystemType_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   SystemType
	}{
		{name: "immutable survives the round-trip", in: SystemTypeImmutable},
		{name: "mutable survives the round-trip", in: SystemTypeMutable},
		{name: "unset stays unset", in: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := &NodeGroup{Spec: NodeGroupSpec{NodeType: NodeTypeCloud, SystemType: tc.in}}

			hub := &v1.NodeGroup{}
			if err := src.ConvertTo(hub); err != nil {
				t.Fatalf("convert to v1 failed: %v", err)
			}
			if hub.Spec.SystemType != v1.SystemType(tc.in) {
				t.Fatalf("systemType in v1 = %q, want %q", hub.Spec.SystemType, tc.in)
			}

			back := &NodeGroup{}
			if err := back.ConvertFrom(hub); err != nil {
				t.Fatalf("convert from v1 failed: %v", err)
			}
			if back.Spec.SystemType != tc.in {
				t.Fatalf("systemType after the round-trip = %q, want %q", back.Spec.SystemType, tc.in)
			}
		})
	}
}
