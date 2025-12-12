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

package moduleconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

type registrySettingsOption func(*RegistrySettings)

func registrySettingsBuilder(opts ...registrySettingsOption) *RegistrySettings {
	settings := RegistrySettings{
		ImagesRepo: "test:80/a/b/c/d",
		Scheme:     constant.SchemeHTTPS,
		CA:         "-----BEGIN CERTIFICATE-----",
		Username:   "test-user",
		Password:   "test-password",
		CheckMode:  constant.CheckModeDefault,
	}

	for _, opt := range opts {
		opt(&settings)
	}

	return &settings
}

func TestDeckhouseSettings_ToMap(t *testing.T) {
	registrySettings := registrySettingsBuilder()
	registrySettingsMap := map[string]any{
		"imagesRepo": "test:80/a/b/c/d",
		"scheme":     "HTTPS",
		"username":   "test-user",
		"password":   "test-password",
		"ca":         "-----BEGIN CERTIFICATE-----",
		"checkMode":  "Default",
	}

	tests := []struct {
		name   string
		input  DeckhouseSettings
		output map[string]any
	}{
		{
			name: "mode direct",
			input: DeckhouseSettings{
				Mode:   constant.ModeDirect,
				Direct: registrySettings,
			},
			output: map[string]any{
				"mode":   "Direct",
				"direct": registrySettingsMap,
			},
		},
		{
			name: "mode unmanaged",
			input: DeckhouseSettings{
				Mode:      constant.ModeUnmanaged,
				Unmanaged: registrySettings,
			},
			output: map[string]any{
				"mode":      "Unmanaged",
				"unmanaged": registrySettingsMap,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.EqualValues(t, tt.output, tt.input.ToMap())
		})
	}
}

func TestDeckhouseSettings_ApplySettings(t *testing.T) {
	tests := []struct {
		name     string
		input    DeckhouseSettings
		expected DeckhouseSettings
	}{
		{
			name: "mode direct",
			input: DeckhouseSettings{
				Mode: constant.ModeDirect,
			},
			expected: DeckhouseSettings{
				Mode: constant.ModeDirect,
				Direct: &RegistrySettings{
					ImagesRepo: constant.CEImagesRepo,
					Scheme:     constant.CEScheme,
				},
			},
		},
		{
			name: "mode unmanaged",
			input: DeckhouseSettings{
				Mode: constant.ModeUnmanaged,
			},
			expected: DeckhouseSettings{
				Mode: constant.ModeUnmanaged,
				Unmanaged: &RegistrySettings{
					ImagesRepo: constant.CEImagesRepo,
					Scheme:     constant.CEScheme,
				},
			},
		},
		{
			name: "mode unknown",
			input: DeckhouseSettings{
				Mode: "Unknown",
			},
			expected: DeckhouseSettings{
				Mode: "Unknown",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deckhouseSettings := DeckhouseSettings{}
			deckhouseSettings.ApplySettings(tt.input)
			require.EqualValues(t, tt.expected, deckhouseSettings)
		})
	}
}

func TestRegistrySettings_ApplySettings(t *testing.T) {
	tests := []struct {
		name     string
		input    *RegistrySettings
		expected RegistrySettings
	}{
		{
			name: "default ImagesRepo",
			input: &RegistrySettings{
				ImagesRepo: "",
				Scheme:     "HTTPS",
			},
			expected: RegistrySettings{
				ImagesRepo: constant.CEImagesRepo,
				Scheme:     "HTTPS",
			},
		},
		{
			name: "default Scheme",
			input: &RegistrySettings{
				ImagesRepo: "registry.example.com",
				Scheme:     "",
			},
			expected: RegistrySettings{
				ImagesRepo: "registry.example.com",
				Scheme:     constant.CEScheme,
			},
		},
		{
			name:  "default ImagesRepo and Scheme",
			input: nil,
			expected: RegistrySettings{
				ImagesRepo: constant.CEImagesRepo,
				Scheme:     constant.CEScheme,
			},
		},
		{
			name: "trim ImagesRepo",
			input: &RegistrySettings{
				ImagesRepo: "registry.example.com/",
				Scheme:     "HTTPS",
			},
			expected: RegistrySettings{
				ImagesRepo: "registry.example.com",
				Scheme:     "HTTPS",
			},
		},
		{
			name:     "full",
			input:    registrySettingsBuilder(),
			expected: *registrySettingsBuilder(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var registrySettings RegistrySettings
			registrySettings.ApplySettings(tt.input)

			require.Equal(t, tt.expected, registrySettings)
		})
	}
}

func TestDeckhouseSettings_Validate(t *testing.T) {
	type output struct {
		err    bool
		errMsg string
	}

	tests := []struct {
		name   string
		input  DeckhouseSettings
		output output
	}{
		// Valid cases
		{
			name: "valid direct mode",
			input: DeckhouseSettings{
				Mode:      constant.ModeDirect,
				Direct:    registrySettingsBuilder(),
				Unmanaged: nil,
			},
			output: output{
				err: false,
			},
		},
		{
			name: "valid unmanaged mode",
			input: DeckhouseSettings{
				Mode:      constant.ModeUnmanaged,
				Direct:    nil,
				Unmanaged: registrySettingsBuilder(),
			},
			output: output{
				err: false,
			},
		},
		// Invalid cases - Mode
		{
			name: "invalid mode",
			input: DeckhouseSettings{
				Mode:      "invalid-mode",
				Direct:    nil,
				Unmanaged: nil,
			},
			output: output{
				err:    true,
				errMsg: "Unknown registry mode",
			},
		},
		{
			name: "empty mode",
			input: DeckhouseSettings{
				Mode:      "",
				Direct:    nil,
				Unmanaged: nil,
			},
			output: output{
				err:    true,
				errMsg: "Unknown registry mode",
			},
		},
		// Invalid cases - Direct mode validation
		{
			name: "direct mode without direct settings",
			input: DeckhouseSettings{
				Mode:      constant.ModeDirect,
				Direct:    nil,
				Unmanaged: nil,
			},
			output: output{
				err:    true,
				errMsg: "direct: is required",
			},
		},
		{
			name: "non-direct mode with direct settings",
			input: DeckhouseSettings{
				Mode:      constant.ModeUnmanaged,
				Direct:    registrySettingsBuilder(),
				Unmanaged: registrySettingsBuilder(),
			},
			output: output{
				err:    true,
				errMsg: "Field 'direct' must be empty",
			},
		},
		// Invalid cases - Unmanaged mode validation
		{
			name: "unmanaged mode without unmanaged settings",
			input: DeckhouseSettings{
				Mode:      constant.ModeUnmanaged,
				Direct:    nil,
				Unmanaged: nil,
			},
			output: output{
				err:    true,
				errMsg: "unmanaged: is required",
			},
		},
		{
			name: "non-unmanaged mode with unmanaged settings",
			input: DeckhouseSettings{
				Mode:      constant.ModeDirect,
				Direct:    registrySettingsBuilder(),
				Unmanaged: registrySettingsBuilder(),
			},
			output: output{
				err:    true,
				errMsg: "Field 'unmanaged' must be empty",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if tt.output.err {
				require.Error(t, err)
				if tt.output.errMsg != "" {
					require.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRegistrySettings_ToMap(t *testing.T) {
	tests := []struct {
		name   string
		input  RegistrySettings
		output map[string]any
	}{
		{
			name:  "all fields",
			input: *registrySettingsBuilder(),
			output: map[string]any{
				"imagesRepo": "test:80/a/b/c/d",
				"scheme":     "HTTPS",
				"username":   "test-user",
				"password":   "test-password",
				"ca":         "-----BEGIN CERTIFICATE-----",
				"checkMode":  "Default",
			},
		},
		{
			name: "optional fields",
			input: *registrySettingsBuilder(
				func(rs *RegistrySettings) {
					rs.Username = ""
					rs.Password = ""
					rs.CA = ""
					rs.CheckMode = ""
				},
			),
			output: map[string]any{
				"imagesRepo": "test:80/a/b/c/d",
				"scheme":     "HTTPS",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.EqualValues(t, tt.output, tt.input.ToMap())
		})
	}
}

func TestRegistrySettings_Validate(t *testing.T) {
	type output struct {
		err    bool
		errMsg string
	}

	tests := []struct {
		name   string
		input  *RegistrySettings
		output output
	}{
		// Valid cases
		{
			name:  "valid settings with all fields",
			input: registrySettingsBuilder(),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings without credentials",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.Username = "" },
				func(s *RegistrySettings) { s.Password = "" },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with HTTP scheme and no CA",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.Scheme = constant.SchemeHTTP },
				func(s *RegistrySettings) { s.CA = "" },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with license only",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.Username = "" },
				func(s *RegistrySettings) { s.Password = "" },
				func(s *RegistrySettings) { s.License = "test-license" },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with relaxed check mode",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.CheckMode = constant.CheckModeRelax },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "empty check mode is valid",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.CheckMode = "" },
			),
			output: output{
				err: false,
			},
		},

		// Invalid cases - ImagesRepo
		{
			name: "empty images repo",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.ImagesRepo = "" },
			),
			output: output{
				err:    true,
				errMsg: "Field 'imagesRepo' is required",
			},
		},

		// Invalid cases - Scheme
		{
			name: "invalid scheme",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.Scheme = "ftp" },
			),
			output: output{
				err:    true,
				errMsg: "Invalid scheme",
			},
		},
		{
			name: "empty scheme",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.Scheme = "" },
			),
			output: output{
				err:    true,
				errMsg: "Invalid scheme",
			},
		},

		// Invalid cases - Credentials
		{
			name: "password without username",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.Username = "" },
				func(s *RegistrySettings) { s.Password = "test-password" },
			),
			output: output{
				err:    true,
				errMsg: "Username is required when password is provided",
			},
		},
		{
			name: "username without password",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.Username = "test-user" },
				func(s *RegistrySettings) { s.Password = "" },
			),
			output: output{
				err:    true,
				errMsg: "Password is required when username is provided",
			},
		},

		// Invalid cases - License
		{
			name: "license with credentials",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.License = "test-license" },
			),
			output: output{
				err:    true,
				errMsg: "License field must be empty when using credentials (username/password)",
			},
		},
		{
			name: "license with username only",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.Password = "" },
				func(s *RegistrySettings) { s.License = "test-license" },
			),
			output: output{
				err:    true,
				errMsg: "License field must be empty when using credentials (username/password)",
			},
		},
		{
			name: "license with password only",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.Username = "" },
				func(s *RegistrySettings) { s.License = "test-license" },
			),
			output: output{
				err:    true,
				errMsg: "License field must be empty when using credentials (username/password)",
			},
		},

		// Invalid cases - CA
		{
			name: "CA with HTTP scheme",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.Scheme = constant.SchemeHTTP },
			),
			output: output{
				err:    true,
				errMsg: "CA is not allowed when scheme is 'HTTP'",
			},
		},

		// Invalid cases - CheckMode
		{
			name: "invalid check mode",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.CheckMode = "invalid-mode" },
			),
			output: output{
				err:    true,
				errMsg: "unknown registry check mode",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if tt.output.err {
				require.Error(t, err)
				if tt.output.errMsg != "" {
					require.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
