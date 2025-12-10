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
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
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
		{
			name: "mode unmanaged && legacy ",
			input: TestConfigBuilder(
				WithModeUnmanaged(),
				WithLegacyMode(),
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

func TestManifestsDirectMode(t *testing.T) {
	pki, err := GeneratePKI()
	require.NoError(t, err)
	getPKI := func() (PKI, error) {
		return pki, nil
	}

	t.Run("Direct mode", func(t *testing.T) {
		config := TestConfigBuilder(
			WithModeDirect(),
			WithImagesRepo("r.example.com/test"),
			WithCredentials("test-user", "test-password"),
			WithSchemeHTTPS(),
			WithCA("-----BEGIN CERTIFICATE-----"),
		)

		t.Run("modeSettings", func(t *testing.T) {
			expect := ModeSettings{
				Mode: constant.ModeDirect,
				Remote: Data{
					ImagesRepo: "r.example.com/test",
					Scheme:     "HTTPS",
					CA:         "-----BEGIN CERTIFICATE-----",
					Username:   "test-user",
					Password:   "test-password",
				},
			}
			require.EqualValues(t, config.Settings, expect)
		})

		t.Run("modeModel", func(t *testing.T) {
			expect := ModeModel{
				Mode:                constant.ModeDirect,
				InClusterImagesRepo: constant.HostWithPath,
				RemoteImagesRepo:    "r.example.com/test",
				RemoteData: Data{
					ImagesRepo: "r.example.com/test",
					Scheme:     "HTTPS",
					CA:         "-----BEGIN CERTIFICATE-----",
					Username:   "test-user",
					Password:   "test-password",
				},
			}
			require.EqualValues(t, config.Settings.ToModel(), expect)
		})

		t.Run("InclusterData", func(t *testing.T) {
			actual, err := config.Settings.ToModel().InClusterData(getPKI)
			require.NoError(t, err)

			expect := Data{
				ImagesRepo: constant.HostWithPath,
				CA:         pki.CA.Cert,
				Scheme:     "HTTPS",
				Username:   "test-user",
				Password:   "test-password",
			}
			require.EqualValues(t, actual, expect)
		})

		t.Run("BashibleConfig", func(t *testing.T) {
			actual, err := config.Settings.ToModel().BashibleConfig()
			require.NoError(t, err)
			require.NotEmpty(t, actual.Version)
			actual.Version = ""

			expect := bashible.Config{
				Mode:           constant.ModeDirect,
				Version:        "",
				ImagesBase:     constant.HostWithPath,
				ProxyEndpoints: nil,
				Hosts: map[string]bashible.ConfigHosts{
					constant.Host: {
						Mirrors: []bashible.ConfigMirrorHost{
							{
								Host:   "r.example.com",
								Scheme: "https",
								CA:     "-----BEGIN CERTIFICATE-----",
								Auth: bashible.ConfigAuth{
									Username: "test-user",
									Password: "test-password",
								},
								Rewrites: []bashible.ConfigRewrite{
									{
										From: constant.PathRegexp,
										To:   "test",
									},
								},
							},
						},
					},
				},
			}
			require.EqualValues(t, actual, expect)
		})
	})
}

func TestManifestsUnmanagedMode(t *testing.T) {
	pki, err := GeneratePKI()
	require.NoError(t, err)
	getPKI := func() (PKI, error) {
		return pki, nil
	}

	t.Run("Unmanaged mode", func(t *testing.T) {
		config := TestConfigBuilder(
			WithModeUnmanaged(),
			WithImagesRepo("r.example.com/test"),
			WithCredentials("test-user", "test-password"),
			WithSchemeHTTPS(),
			WithCA("-----BEGIN CERTIFICATE-----"),
		)

		t.Run("modeSettings", func(t *testing.T) {
			expect := ModeSettings{
				Mode: constant.ModeUnmanaged,
				Remote: Data{
					ImagesRepo: "r.example.com/test",
					Scheme:     "HTTPS",
					CA:         "-----BEGIN CERTIFICATE-----",
					Username:   "test-user",
					Password:   "test-password",
				},
			}
			require.EqualValues(t, config.Settings, expect)
		})

		t.Run("modeModel", func(t *testing.T) {
			expect := ModeModel{
				Mode:                constant.ModeUnmanaged,
				InClusterImagesRepo: "r.example.com/test",
				RemoteImagesRepo:    "r.example.com/test",
				RemoteData: Data{
					ImagesRepo: "r.example.com/test",
					Scheme:     "HTTPS",
					CA:         "-----BEGIN CERTIFICATE-----",
					Username:   "test-user",
					Password:   "test-password",
				},
			}
			require.EqualValues(t, config.Settings.ToModel(), expect)
		})

		t.Run("InclusterData", func(t *testing.T) {
			actual, err := config.Settings.ToModel().InClusterData(getPKI)
			require.NoError(t, err)

			expect := Data{
				ImagesRepo: "r.example.com/test",
				Scheme:     "HTTPS",
				CA:         "-----BEGIN CERTIFICATE-----",
				Username:   "test-user",
				Password:   "test-password",
			}
			require.EqualValues(t, actual, expect)
		})

		t.Run("BashibleConfig", func(t *testing.T) {
			actual, err := config.Settings.ToModel().BashibleConfig()
			require.NoError(t, err)
			require.NotEmpty(t, actual.Version)
			actual.Version = ""

			expect := bashible.Config{
				Mode:           constant.ModeUnmanaged,
				Version:        "",
				ImagesBase:     "r.example.com/test",
				ProxyEndpoints: nil,
				Hosts: map[string]bashible.ConfigHosts{
					"r.example.com": {
						Mirrors: []bashible.ConfigMirrorHost{
							{
								Host:   "r.example.com",
								Scheme: "https",
								CA:     "-----BEGIN CERTIFICATE-----",
								Auth: bashible.ConfigAuth{
									Username: "test-user",
									Password: "test-password",
								},
							},
						},
					},
				},
			}
			require.EqualValues(t, actual, expect)
		})
	})
}
