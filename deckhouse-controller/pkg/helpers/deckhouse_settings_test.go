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

package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeckhouseSettings_ExperimentalModuleAllowed(t *testing.T) {
	tests := []struct {
		name     string
		allowAll bool
		allowed  []string
		module   string
		want     bool
	}{
		{
			name:   "blocked by default",
			module: "foo",
			want:   false,
		},
		{
			name:     "allowed when all experimental modules are allowed",
			allowAll: true,
			module:   "foo",
			want:     true,
		},
		{
			name:    "allowed when listed in the allowlist",
			allowed: []string{"foo", "bar"},
			module:  "foo",
			want:    true,
		},
		{
			name:    "blocked when a different module is allowlisted",
			allowed: []string{"bar"},
			module:  "foo",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &DeckhouseSettings{
				AllowExperimentalModules:   tt.allowAll,
				AllowedExperimentalModules: tt.allowed,
			}

			assert.Equal(t, tt.want, settings.ExperimentalModuleAllowed(tt.module))
		})
	}
}
