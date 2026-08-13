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

// contexts are keyed by NodeGroup name; the {os}. prefix is stripped by TransformName beforehand
func TestContextKeys(t *testing.T) {
	tests := []struct {
		name         string
		arg          string
		wantBundle   string
		wantBashible string
	}{
		{"nodegroup name", "master-flomaster", "bundle-master-flomaster", "bashible-master-flomaster"},
		{"nodegroup with a dot", "ubuntu-lts.master", "bundle-ubuntu-lts.master", "bashible-ubuntu-lts.master"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetNodegroupContextKey(tt.arg)
			if err != nil {
				t.Fatalf("GetNodegroupContextKey() error = %v", err)
			}
			if got != tt.wantBundle {
				t.Errorf("GetNodegroupContextKey() = %v, want %v", got, tt.wantBundle)
			}

			got, err = GetBashibleContextKey(tt.arg)
			if err != nil {
				t.Fatalf("GetBashibleContextKey() error = %v", err)
			}
			if got != tt.wantBashible {
				t.Errorf("GetBashibleContextKey() = %v, want %v", got, tt.wantBashible)
			}

			got, err = GetBootstrapContextKey(tt.arg)
			if err != nil {
				t.Fatalf("GetBootstrapContextKey() error = %v", err)
			}
			if got != tt.wantBashible {
				t.Errorf("GetBootstrapContextKey() = %v, want %v", got, tt.wantBashible)
			}
		})
	}
}

func TestTransformName(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{"bundle-prefixed name loses the bundle", "ubuntu-lts.master-flomaster", "master-flomaster"},
		{"plain nodegroup name is kept", "master-flomaster", "master-flomaster"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformName(tt.arg)
			if err != nil {
				t.Fatalf("TransformName() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("TransformName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseName(t *testing.T) {
	os, target, err := ParseName("ubuntu-lts.master-flomaster")
	if err != nil {
		t.Fatalf("ParseName() error = %v", err)
	}
	if os != "ubuntu-lts" || target != "master-flomaster" {
		t.Errorf("ParseName() = %v, %v, want ubuntu-lts, master-flomaster", os, target)
	}

	if _, _, err := ParseName("ubuntu-lts-master-flomaster"); err == nil {
		t.Errorf("ParseName() without a dot: error = nil, want an error")
	}
}
