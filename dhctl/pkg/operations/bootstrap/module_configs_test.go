// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bootstrap

import (
	"reflect"
	"testing"
)

func Test_computeModulesEnablement(t *testing.T) {
	type args struct {
		configOverrides map[string]any
	}
	tests := []struct {
		name string
		args args
		want map[string]bool
	}{
		{
			name: "no overrides",
			want: map[string]bool{},
		},
		{
			name: "success test",
			args: args{configOverrides: map[string]any{
				// Enabled and has config
				"cniCiliumEnabled": true,
				"cniCilium":        map[string]any{},

				// Enabled without config
				"upmeterEnabled": true,

				// Implicitly enabled without "*Enabled" definition
				"userAuthn": map[string]any{},

				// Disabled without config
				"prometeusEnabled": false,

				// Disabled with config
				"dummyEnabled": false,
				"dummy":        map[string]any{},
			}},
			want: map[string]bool{
				"cniCilium": true,
				"upmeter":   true,
				"userAuthn": true,
				"prometeus": false,
				"dummy":     false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeModuleEnabledStatuses(tt.args.configOverrides); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("computeModuleEnabledStatuses() = %v, want %v", got, tt.want)
			}
		})
	}
}
