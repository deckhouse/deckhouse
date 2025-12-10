/*
Copyright 2025 Flant JSC

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

package initsecret

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContextToMap(t *testing.T) {
	tests := []struct {
		name   string
		input  Config
		result map[string]any
	}{
		{
			name: "with all fields",
			input: Config{
				CA: &CertKey{
					Cert: "cert",
					Key:  "key",
				},
			},
			result: map[string]any{
				"ca": map[string]any{
					"cert": "cert",
					"key":  "key",
				},
			},
		},

		{
			name:   "without optional fields",
			input:  Config{},
			result: map[string]any{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.result, tt.input.ToMap())
		})
	}
}
