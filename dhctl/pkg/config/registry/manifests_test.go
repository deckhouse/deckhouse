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
			name: "mode proxy",
			input: ConfigBuilder(
				WithModeProxy(),
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
					RegistryBashibleConfigSecretData(GeneratePKI)

				require.NoError(t, err)
			})

			t.Run("KubeadmContext", func(t *testing.T) {
				_ = tt.input.
					Manifest().
					KubeadmContext()
			})

			t.Run("BashibleContext", func(t *testing.T) {
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
			name: "mode proxy",
			input: ConfigBuilder(
				WithModeProxy(),
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
			t.Run("BashibleContext -> registry module enabled when not in legacy mode", func(t *testing.T) {
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
					RegistryBashibleConfigSecretData(GeneratePKI)

				require.NoError(t, err)

				expectedExists := !tt.legacyMode
				require.Equal(t, expectedExists, exists)
			})
		})
	}
}

// TestTheBashibleContextTellsTheNodeItCameFromABundle pins the one field the node bootstrap uses to
// tell an installation from a bundle apart from a legacy Local one.
//
// On the node the two are identical by design — the bundle path borrows the legacy bootstrap steps to
// stand a registry up on the first master and fill it. But one of those steps hands the cluster the
// legacy implementation's state machine, and on this path there is nothing to execute it, so it has to
// stay shut. If this field stops reaching the context, that step opens again and nothing fails
// visibly: the installation finishes, and which registry implementation ends up managing the cluster
// becomes a race (see modules/038-registry/bashible_tests).
func TestTheBashibleContextTellsTheNodeItCameFromABundle(t *testing.T) {
	fromBundle := ConfigBuilder(WithModeLocal())
	fromBundle.BundleBootstrap = true

	ctx, err := fromBundle.Manifest().BashibleContext(GeneratePKI)
	require.NoError(t, err)
	require.NotNil(t, ctx.Bootstrap)
	require.True(t, ctx.Bootstrap.FromBundle)
	require.Equal(t, true, ctx.Bootstrap.ToMap()["fromBundle"],
		"the steps read the rendered map, not the struct")

	// The same mode without a bundle is a legacy Local cluster, and it must render as it always did:
	// the key absent entirely, not present-and-false.
	plain := ConfigBuilder(WithModeLocal())
	legacyLocal, err := plain.Manifest().BashibleContext(GeneratePKI)
	require.NoError(t, err)
	require.NotNil(t, legacyLocal.Bootstrap)
	require.False(t, legacyLocal.Bootstrap.FromBundle)
	require.NotContains(t, legacyLocal.Bootstrap.ToMap(), "fromBundle")
}
