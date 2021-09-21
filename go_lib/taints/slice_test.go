/*
Copyright 2021 Flant JSC

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
