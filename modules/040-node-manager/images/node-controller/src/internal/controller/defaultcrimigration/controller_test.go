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

package defaultcrimigration

import "testing"

func TestIsObsoleteDefaultCRI(t *testing.T) {
	tests := []struct {
		name             string
		clusterConfigCRI string
		moduleConfigCRI  string
		want             bool
	}{
		{name: "neither set", clusterConfigCRI: "", moduleConfigCRI: "", want: false},
		{name: "ClusterConfiguration at default", clusterConfigCRI: "Containerd", moduleConfigCRI: "", want: false},
		{name: "ClusterConfiguration non-default, not migrated", clusterConfigCRI: "ContainerdV2", moduleConfigCRI: "", want: true},
		{name: "ClusterConfiguration non-default, ModuleConfig at default", clusterConfigCRI: "ContainerdV2", moduleConfigCRI: "Containerd", want: true},
		{name: "ClusterConfiguration non-default, migrated to ModuleConfig", clusterConfigCRI: "ContainerdV2", moduleConfigCRI: "ContainerdV2", want: false},
		{name: "NotManaged in ClusterConfiguration, not migrated", clusterConfigCRI: "NotManaged", moduleConfigCRI: "", want: true},
		{name: "only ModuleConfig set", clusterConfigCRI: "", moduleConfigCRI: "ContainerdV2", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isObsoleteDefaultCRI(tt.clusterConfigCRI, tt.moduleConfigCRI); got != tt.want {
				t.Fatalf("isObsoleteDefaultCRI(%q, %q) = %v, want %v", tt.clusterConfigCRI, tt.moduleConfigCRI, got, tt.want)
			}
		})
	}
}
