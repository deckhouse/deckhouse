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

package v1alpha1

import (
	"testing"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestSpecConversion_PreservesSeccompDefault_ToV1(t *testing.T) {
	in := &NodeGroupSpec{
		NodeType: NodeTypeCloud,
		Kubelet: &KubeletSpec{
			SeccompDefault: boolPtr(true),
		},
	}

	out := &v1.NodeGroupSpec{}
	if err := ConvertV1alpha1NodeGroupSpecToV1NodeGroupSpec(in, out, nil); err != nil {
		t.Fatalf("conversion to v1 failed: %v", err)
	}

	if out.Kubelet == nil || out.Kubelet.SeccompDefault == nil || !*out.Kubelet.SeccompDefault {
		t.Fatalf("seccompDefault was not converted to v1")
	}
}

func TestSpecConversion_PreservesSeccompDefault_FromV1(t *testing.T) {
	in := &v1.NodeGroupSpec{
		NodeType: v1.NodeTypeCloudEphemeral,
		Kubelet: &v1.KubeletSpec{
			SeccompDefault: boolPtr(true),
		},
	}

	out := &NodeGroupSpec{}
	if err := ConvertV1NodeGroupSpecToV1alpha1NodeGroupSpec(in, out, nil); err != nil {
		t.Fatalf("conversion from v1 failed: %v", err)
	}

	if out.Kubelet == nil || out.Kubelet.SeccompDefault == nil || !*out.Kubelet.SeccompDefault {
		t.Fatalf("seccompDefault was not converted from v1")
	}
}

// A write issued at v1alpha1 round-trips through the hub, so a systemType the
// conversion drops is stored as Mutable — which flips an immutable group's
// MachineDeployments back to the bashible bootstrap and re-creates every machine
// in the group.
func TestSpecConversion_PreservesSystemType_RoundTrip(t *testing.T) {
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
			src := &NodeGroupSpec{NodeType: NodeTypeCloud, SystemType: tc.in}

			hub := &v1.NodeGroupSpec{}
			if err := ConvertV1alpha1NodeGroupSpecToV1NodeGroupSpec(src, hub, nil); err != nil {
				t.Fatalf("conversion to v1 failed: %v", err)
			}
			if hub.SystemType != v1.SystemType(tc.in) {
				t.Fatalf("systemType in v1 = %q, want %q", hub.SystemType, tc.in)
			}

			back := &NodeGroupSpec{}
			if err := ConvertV1NodeGroupSpecToV1alpha1NodeGroupSpec(hub, back, nil); err != nil {
				t.Fatalf("conversion from v1 failed: %v", err)
			}
			if back.SystemType != tc.in {
				t.Fatalf("systemType after the round-trip = %q, want %q", back.SystemType, tc.in)
			}
		})
	}
}
