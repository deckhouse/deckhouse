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

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
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
		name                  string
		input                 Config
		expectedModuleEnabled bool
	}{
		{
			name: "mode direct",
			input: ConfigBuilder(
				WithModeDirect(),
			),
			// Direct is a managed mode, non-legacy → module enabled.
			expectedModuleEnabled: true,
		},
		{
			name: "mode proxy",
			input: ConfigBuilder(
				WithModeProxy(),
			),
			// Proxy is a managed mode, non-legacy → module enabled.
			expectedModuleEnabled: true,
		},
		{
			name: "mode unmanaged",
			input: ConfigBuilder(
				WithModeUnmanaged(),
			),
			// Unmanaged leaves the registry infrastructure untouched even in
			// non-legacy mode → module NOT enabled.
			expectedModuleEnabled: false,
		},
		{
			name: "mode unmanaged && legacy",
			input: ConfigBuilder(
				WithLegacyMode(),
				WithModeUnmanaged(),
			),
			// Legacy + Unmanaged → module NOT enabled.
			expectedModuleEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("BashibleContext -> registry module enabled for managed non-legacy modes", func(t *testing.T) {
				ctx, err := tt.input.
					Manifest().
					BashibleContext(GeneratePKI)

				require.NoError(t, err)
				require.Equal(t, tt.expectedModuleEnabled, ctx.RegistryModuleEnable)
			})

			t.Run("RegistryBashibleConfigSecretData -> exists when not in legacy mode", func(t *testing.T) {
				exists, _, err := tt.input.
					Manifest().
					RegistryBashibleConfigSecretData(GeneratePKI)

				require.NoError(t, err)

				// Secret exists for all non-legacy modes (Direct/Proxy/Unmanaged/Local),
				// not just managed modes — the config describes what BashibleConfig
				// produced.
				legacyMode := tt.input.LegacyMode
				require.Equal(t, !legacyMode, exists)
			})
		})
	}
}

// TestBashibleContext_LocalMode asserts Local (air-gap) mode uses HostWithPath as
// ImagesBase and seeds containerd with TWO mirrors: cache (preferred) then seed
// (fallback), both https with the module CA and without any path rewrites.
func TestBashibleContext_LocalMode(t *testing.T) {
	cfg := ConfigBuilder(WithModeLocal())
	ctx, err := cfg.Manifest().BashibleContext(GeneratePKI)
	require.NoError(t, err)

	// C1: ImagesBase must be HostWithPath so bashible resolves digests correctly.
	require.Equal(t, constant.HostWithPath, ctx.ImagesBase,
		"Local mode ImagesBase must be constant.HostWithPath (%s), got %s",
		constant.HostWithPath, ctx.ImagesBase)

	// C2: Hosts must contain two mirrors — cache first, seed fallback.
	entry, ok := ctx.Hosts[constant.Host]
	require.True(t, ok, "ctx.Hosts must have key constant.Host (%s)", constant.Host)
	require.Len(t, entry.Mirrors, 2, "Local mode must have exactly 2 mirrors (cache then seed)")

	// Mirror 0: in-cluster cache (preferred once up).
	cache := entry.Mirrors[0]
	require.Equal(t, "registry-cache.d8-system.svc:5001", cache.Host,
		"mirror0 must be the in-cluster cache")
	require.Equal(t, "https", cache.Scheme, "cache mirror scheme must be https")
	require.NotEmpty(t, cache.CA, "cache mirror must carry the module CA")
	require.Empty(t, cache.Rewrites,
		"cache mirror must have NO rewrites: cache serves rooted at system/deckhouse")

	// Mirror 1: on-node seed process (available from first boot).
	seed := entry.Mirrors[1]
	require.Equal(t, "127.0.0.1:5010", seed.Host,
		"mirror1 must be the on-node seed")
	require.Equal(t, "https", seed.Scheme, "seed mirror scheme must be https")
	require.NotEmpty(t, seed.CA, "seed mirror must carry the module CA")
	require.Empty(t, seed.Rewrites,
		"seed mirror must have NO rewrites: seed serves rooted at system/deckhouse")

	// I1: Managed mode → module must be enabled and Bootstrap must be set.
	require.True(t, ctx.RegistryModuleEnable, "Local mode must enable registry module")
	require.NotNil(t, ctx.Bootstrap, "Local mode must set Bootstrap.Init")
}

// TestBashibleContext_DirectMode asserts that Direct (connected) mode does NOT
// clobber the hosts produced by ToContext(): it must keep the upstream mirror
// with the ^system/deckhouse -> <upstream-path> rewrite intact.
func TestBashibleContext_DirectMode(t *testing.T) {
	// Use a non-default upstream path so the rewrite target is distinguishable.
	cfg := ConfigBuilder(
		WithModeDirect(),
		WithImagesRepo("r.example.com/corp/images"),
		WithCredentials("user", "pass"),
		WithSchemeHTTPS(),
	)
	ctx, err := cfg.Manifest().BashibleContext(GeneratePKI)
	require.NoError(t, err)

	// C1: ImagesBase must be constant.HostWithPath (set by BashibleConfig/ToContext).
	require.Equal(t, constant.HostWithPath, ctx.ImagesBase,
		"Direct mode ImagesBase must be constant.HostWithPath (%s)", constant.HostWithPath)

	// C2: ctx.Hosts must NOT have been replaced by BootstrapSeedHosts; it must
	// contain the upstream mirror with the path rewrite from ToContext().
	entry, ok := ctx.Hosts[constant.Host]
	require.True(t, ok, "ctx.Hosts must have key constant.Host (%s)", constant.Host)
	require.Len(t, entry.Mirrors, 1)

	mirror := entry.Mirrors[0]
	require.Equal(t, "r.example.com", mirror.Host, "Direct mode mirror host must be the upstream registry host")

	// The rewrite From=^system/deckhouse must be present (produced by
	// toDirectBashibleHosts via ToContext, not clobbered by BashibleContext).
	require.NotEmpty(t, mirror.Rewrites, "Direct mode mirror must have the ^system/deckhouse rewrite")
	require.Equal(t, constant.PathRegexp, mirror.Rewrites[0].From,
		"Direct mode rewrite From must be constant.PathRegexp (%s)", constant.PathRegexp)

	// I1: Managed mode → module enabled and Bootstrap set.
	require.True(t, ctx.RegistryModuleEnable, "Direct mode must enable registry module")
	require.NotNil(t, ctx.Bootstrap, "Direct mode must set Bootstrap.Init")
}

// TestBashibleContext_UnmanagedMode asserts I1: Unmanaged mode must NOT set
// RegistryModuleEnable or Bootstrap even in non-legacy mode.
func TestBashibleContext_UnmanagedMode(t *testing.T) {
	cfg := ConfigBuilder(
		WithModeUnmanaged(),
		WithImagesRepo("r.example.com/test"),
		WithSchemeHTTPS(),
	)
	ctx, err := cfg.Manifest().BashibleContext(GeneratePKI)
	require.NoError(t, err)

	require.False(t, ctx.RegistryModuleEnable,
		"Unmanaged mode must NOT enable registry module")
	require.Nil(t, ctx.Bootstrap,
		"Unmanaged mode must NOT set Bootstrap")
}
