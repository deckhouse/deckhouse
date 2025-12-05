// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package registry

import (
	"testing"

	"github.com/stretchr/testify/require"

	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"
)

func TestModeNoError(t *testing.T) {
	tests := []struct {
		name  string
		input module_config.DeckhouseSettings
	}{
		{
			name: "mode direct",
			input: TestConfigBuilder(
				WithModeDirect(),
			).DeckhouseSettings,
		},
		{
			name: "mode unmanaged",
			input: TestConfigBuilder(
				WithModeUnmanaged(),
			).DeckhouseSettings,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings, err := newModeSettings(tt.input)
			require.NoError(t, err)
			model := settings.ToModel()

			t.Run("InClusterData", func(t *testing.T) {
				_, err := model.InClusterData(GeneratePKI)
				require.NoError(t, err)
			})

			t.Run("BashibleConfig", func(t *testing.T) {
				_, err := model.BashibleConfig()
				require.NoError(t, err)
			})
		})
	}
}
