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
)

func TestManifestsNoError(t *testing.T) {
	tests := []struct {
		name  string
		input Config
	}{
		{
			name: "mode direct",
			input: ConfigBuilder(
				WithModeDirect(),
			),
		},
		{
			name: "mode unmanaged",
			input: ConfigBuilder(
				WithModeUnmanaged(),
			),
		},
		{
			name: "mode unmanaged && legacy",
			input: ConfigBuilder(
				WithLegacyMode(),
				WithModeUnmanaged(),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("DeckhouseRegistrySecretData", func(t *testing.T) {
				_, err := tt.input.
					Manifest().
					DeckhouseRegistrySecretData(GeneratePKI)

				require.NoError(t, err)
			})

			t.Run("RegistryBashibleConfigSecretData", func(t *testing.T) {
				_, _, err := tt.input.
					Manifest().
					RegistryBashibleConfigSecretData()

				require.NoError(t, err)
			})

			t.Run("KubeadmTplCtx", func(t *testing.T) {
				_ = tt.input.
					Manifest().
					KubeadmTplCtx()
			})

			t.Run("BashibleTplCtx", func(t *testing.T) {
				_, err := tt.input.
					Manifest().
					BashibleContext(GeneratePKI)

				require.NoError(t, err)
			})
		})
	}
}

func TestManifestsLegacyMode(t *testing.T) {
	tests := []struct {
		name       string
		input      Config
		legacyMode bool
	}{
		{
			name: "mode direct",
			input: ConfigBuilder(
				WithModeDirect(),
			),
			legacyMode: false,
		},
		{
			name: "mode unmanaged",
			input: ConfigBuilder(
				WithModeUnmanaged(),
			),
			legacyMode: false,
		},
		{
			name: "mode unmanaged && legacy",
			input: ConfigBuilder(
				WithLegacyMode(),
				WithModeUnmanaged(),
			),
			legacyMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("BashibleTplCtx -> registry module enabled when not in legacy mode", func(t *testing.T) {
				ctx, err := tt.input.
					Manifest().
					BashibleContext(GeneratePKI)

				require.NoError(t, err)

				expectedModuleEnabled := !tt.legacyMode
				require.Equal(t, expectedModuleEnabled, ctx.RegistryModuleEnable)
			})

			t.Run("RegistryBashibleConfigSecretData -> exists when not in legacy mode", func(t *testing.T) {
				exists, _, err := tt.input.
					Manifest().
					RegistryBashibleConfigSecretData()

				require.NoError(t, err)

				expectedExists := !tt.legacyMode
				require.Equal(t, expectedExists, exists)
			})
		})
	}
}
