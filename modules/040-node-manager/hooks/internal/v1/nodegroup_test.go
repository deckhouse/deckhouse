package v1

import "testing"

func TestKubeletIsEmpty_WithSeccompDefault(t *testing.T) {
	k := Kubelet{}
	if !k.IsEmpty() {
		t.Fatalf("expected empty kubelet config")
	}

	k.SeccompDefault = true
	if k.IsEmpty() {
		t.Fatalf("expected non-empty kubelet config when seccompDefault is set")
	}
}

