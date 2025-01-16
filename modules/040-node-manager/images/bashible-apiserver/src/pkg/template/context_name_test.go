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

func TestGetNodegroupContextKey(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    string
		wantErr bool
	}{
		{"valid", "ubuntu-lts.master-flomaster", "bundle-ubuntu-lts-master-flomaster", false},
		{"invalid", "ubuntu-lts-master-flomaster", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetNodegroupContextKey(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNodegroupContextKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetNodegroupContextKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
