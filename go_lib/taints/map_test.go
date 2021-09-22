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
