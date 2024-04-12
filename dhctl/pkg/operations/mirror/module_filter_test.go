/*
 * Copyright 2024 Flant JSC
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *     http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mirror

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModuleFilter_Match(t *testing.T) {
	type args struct {
		mod Module
	}
	tests := []struct {
		name string
		f    ModuleFilter
		args args
		want bool
	}{
		{
			name: "Empty filter matches anything",
			f:    map[string][]string{},
			args: args{mod: Module{Name: "test", RegistryPath: "registry.example.com/dh", Releases: []string{"v1.2.3"}}},
			want: true,
		},
		{
			name: "Happy path, module doesn't match",
			f:    map[string][]string{"deckhouse-admin": {"v1.2.0", "v1.2.1", "v1.2.2", "v1.2.3"}},
			args: args{mod: Module{Name: "test", RegistryPath: "registry.example.com/dh", Releases: []string{"v1.2.3"}}},
			want: false,
		},
		{
			name: "Happy path, module matches",
			f:    map[string][]string{"deckhouse-admin": {"v1.2.0", "v1.2.1", "v1.2.2", "v1.2.3"}},
			args: args{mod: Module{Name: "deckhouse-admin", RegistryPath: "registry.example.com/dh", Releases: []string{"v1.2.3"}}},
			want: true,
		},
		{
			name: "Happy path, module unknown to filter",
			f:    map[string][]string{"deckhouse-admin": {"v1.2.0", "v1.2.1", "v1.2.2", "v1.2.3"}},
			args: args{mod: Module{Name: "op-monitoring", RegistryPath: "registry.example.com/dh", Releases: []string{"v1.2.3"}}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.Match(tt.args.mod); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseModuleFilterString(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want ModuleFilter
	}{
		{
			name: "Empty filter expression",
			args: args{str: ""},
			want: nil,
		},
		{
			name: "One filter expression",
			args: args{str: "moduleName:v12.34.56"},
			want: ModuleFilter{"moduleName": {"v12.34.56"}},
		},
		{
			name: "Multiple filter expression for one module",
			args: args{str: "moduleName:v12.34.56;moduleName:v0.0.1;"},
			want: ModuleFilter{"moduleName": {"v12.34.56", "v0.0.1"}},
		},
		{
			name: "Multiple filter expression for different modules",
			args: args{str: "module1:v12.34.56;module2:v0.0.1;"},
			want: ModuleFilter{"module1": {"v12.34.56"}, "module2": {"v0.0.1"}},
		},
		{
			name: "Multiple filter expression for different modules with many versions and bad spacing",
			args: args{str: " ; module1: v12.34.56; module1 :v1.1.1; module2:v0.0.1; module2 : v2.3.2;"},
			want: ModuleFilter{"module1": {"v12.34.56", "v1.1.1"}, "module2": {"v0.0.1", "v2.3.2"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseModuleFilterString(tt.args.str)
			require.Equal(t, tt.want, got)
		})
	}
}
