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

