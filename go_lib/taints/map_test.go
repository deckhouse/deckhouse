package taints

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

func Test_Map_Slice(t *testing.T) {
	taintsMap := map[string]v1.Taint{
		"test1": {
			Key: "test1",
		},
		"test2": {
			Key: "test2",
		},
		"test3": {
			Key: "test3",
		},
	}

	taintsArr := Map(taintsMap).Slice()
	if len(taintsArr) != 3 {
		t.Fatalf("taintsArr should have len 3. Got %d: %#v", len(taintsArr), taintsArr)
	}

	taintsMap = map[string]v1.Taint{}
	taintsArr = Map(taintsMap).Slice()
	if len(taintsArr) != 0 {
		t.Fatalf("taintsArr should have len 0. Got %d: %#v", len(taintsArr), taintsArr)
	}

	taintsMap = make(Map)
	taintsArr = Map(taintsMap).Slice()
	if len(taintsArr) != 0 {
		t.Fatalf("taintsArr should have len 0. Got %d: %#v", len(taintsArr), taintsArr)
	}

	taintsMap = nil
	taintsArr = Map(taintsMap).Slice()
	if len(taintsArr) != 0 {
		t.Fatalf("taintsArr should have len 0. Got %d: %#v", len(taintsArr), taintsArr)
	}
}
