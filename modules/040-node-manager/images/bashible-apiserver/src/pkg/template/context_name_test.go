/*
Copyright 2024 Flant JSC

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

package template

import (
	"testing"
)

// Contexts are keyed by NodeGroup name; the {os}. prefix, when a request carries one, is stripped
// by TransformName before the key is built.
func TestGetNodegroupContextKey(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{"nodegroup name", "master-flomaster", "bundle-master-flomaster"},
		{"name that still carries the bundle", "ubuntu-lts.master", "bundle-ubuntu-lts.master"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetNodegroupContextKey(tt.arg)
			if err != nil {
				t.Fatalf("GetNodegroupContextKey() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("GetNodegroupContextKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
