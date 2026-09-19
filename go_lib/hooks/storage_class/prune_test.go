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

package storage_class

import (
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// convert renders an object in the cluster the way a module renders its desired classes.
func testConvertStorageClass(sc storagev1.StorageClass) testStorageClass {
	return testStorageClass{Name: sc.Name, Type: sc.Parameters["type"]}
}

func testExistingStorageClass(name, classType string) storagev1.StorageClass {
	return storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Parameters: map[string]string{"type": classType},
	}
}

func TestNewModifiedPruneIfFunc(t *testing.T) {
	t.Parallel()

	shouldPrune := NewModifiedPrunePredictor(testConvertStorageClass)

	tests := []struct {
		name    string
		actual  storagev1.StorageClass
		desired *testStorageClass
		want    bool
	}{
		{
			name:    "an unchanged class stays",
			actual:  testExistingStorageClass("fast", "ssd"),
			desired: &testStorageClass{Name: "fast", Type: "ssd"},
		},
		{
			name:    "a class with changed parameters goes",
			actual:  testExistingStorageClass("fast", "hdd"),
			desired: &testStorageClass{Name: "fast", Type: "ssd"},
			want:    true,
		},
		{
			// Deleting it would take away storage the operator may still be using; it is only
			// gone from the desired list, not from the cluster.
			name:    "a class the module no longer wants stays",
			actual:  testExistingStorageClass("legacy", "hdd"),
			desired: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldPrune(tt.actual, tt.desired); got != tt.want {
				t.Errorf("shouldPrune() = %v, want %v", got, tt.want)
			}
		})
	}
}
