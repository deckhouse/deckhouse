package taints

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

func Test_Slice_WithoutKey(t *testing.T) {
	//
	taints := []v1.Taint{
		{
			Key: "test1",
		},
		{
			Key: "test2",
		},
		{
			Key: "test3",
		},
	}

	modTaints := Slice(taints).WithoutKey("test2")

	if modTaints[0].Key != "test1" {
		t.Fatalf("taint[0] should have key='test1', got '%s'", modTaints[0].Key)
	}
	if modTaints[1].Key != "test3" {
		t.Fatalf("taint[1] should have key='test3', got '%s'", modTaints[1].Key)
	}

	taints = []v1.Taint{}
	modTaints = Slice(taints).WithoutKey("test2")
	if len(modTaints) > 0 {
		t.Fatalf("taints should have zero len. Got %#v", modTaints)
	}

	taints = nil
	modTaints = Slice(taints).WithoutKey("test2")
	if len(modTaints) > 0 {
		t.Fatalf("taints should have zero len. Got %#v", modTaints)
	}
}
