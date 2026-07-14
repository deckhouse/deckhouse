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
