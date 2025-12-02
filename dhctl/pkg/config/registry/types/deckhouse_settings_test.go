// Copyright 2025 Flant JSC
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

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

type registrySettingsOption func(*RegistrySettings)

func validRegistrySettings(opts ...registrySettingsOption) *RegistrySettings {
	settings := RegistrySettings{
		ImagesRepo: "test:80/a/b/c/d",
		Scheme:     registry_const.SchemeHTTPS,
		CA:         "-----BEGIN CERTIFICATE-----",
		Username:   "test-user",
		Password:   "test-password",
		CheckMode:  registry_const.CheckModeDefault,
	}
	for _, opt := range opts {
		opt(&settings)
	}
	return &settings
}

func TestDeckhouseSettings_ToMap(t *testing.T) {
	tests := []struct {
		name   string
		input  DeckhouseSettings
		output map[string]interface{}
	}{
		{
			name: "direct mode to map",
			input: DeckhouseSettings{
				Mode:   registry_const.ModeDirect,
				Direct: validRegistrySettings(),
			},
			output: map[string]interface{}{
				"mode": "Direct",
				"direct": map[string]interface{}{
					"imagesRepo": "test:80/a/b/c/d",
					"scheme":     "HTTPS",
					"username":   "test-user",
					"password":   "test-password",
					"ca":         "-----BEGIN CERTIFICATE-----",
					"checkMode":  "Default",
				},
			},
		},
		{
			name: "unmanaged mode to map",
			input: DeckhouseSettings{
				Mode:      registry_const.ModeUnmanaged,
				Unmanaged: validRegistrySettings(),
			},
			output: map[string]interface{}{
				"mode": "Unmanaged",
				"unmanaged": map[string]interface{}{
					"imagesRepo": "test:80/a/b/c/d",
					"scheme":     "HTTPS",
					"username":   "test-user",
					"password":   "test-password",
					"ca":         "-----BEGIN CERTIFICATE-----",
					"checkMode":  "Default",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.input.ToMap()
			assert.NoError(t, err)
			assert.Equal(t, tt.output, result)
		})
	}
}

func TestDeckhouseSettings_CorrectWithDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    DeckhouseSettings
		expected DeckhouseSettings
	}{
		{
			name: "correct direct mode settings",
			input: DeckhouseSettings{
				Mode:   registry_const.ModeDirect,
				Direct: &RegistrySettings{},
			},
			expected: DeckhouseSettings{
				Mode: registry_const.ModeDirect,
				Direct: &RegistrySettings{
					ImagesRepo: registry_const.CEImagesRepo,
					Scheme:     registry_const.CEScheme,
				},
			},
		},
		{
			name: "correct unmanaged mode settings",
			input: DeckhouseSettings{
				Mode:      registry_const.ModeUnmanaged,
				Unmanaged: &RegistrySettings{},
			},
			expected: DeckhouseSettings{
				Mode: registry_const.ModeUnmanaged,
				Unmanaged: &RegistrySettings{
					ImagesRepo: registry_const.CEImagesRepo,
					Scheme:     registry_const.CEScheme,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := tt.input
			settings.CorrectWithDefault()
			assert.Equal(t, tt.expected, settings)
		})
	}
}

func TestRegistrySettings_CorrectWithDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    RegistrySettings
		expected RegistrySettings
	}{
		{
			name: "trim trailing slash from images repo",
			input: RegistrySettings{
				ImagesRepo: "registry.example.com/",
				Scheme:     "HTTPS",
			},
			expected: RegistrySettings{
				ImagesRepo: "registry.example.com",
				Scheme:     "HTTPS",
			},
		},
		{
			name: "empty images repo gets default",
			input: RegistrySettings{
				ImagesRepo: "",
				Scheme:     "HTTPS",
			},
			expected: RegistrySettings{
				ImagesRepo: registry_const.CEImagesRepo,
				Scheme:     "HTTPS",
			},
		},
		{
			name: "empty scheme gets default",
			input: RegistrySettings{
				ImagesRepo: "registry.example.com",
				Scheme:     "",
			},
			expected: RegistrySettings{
				ImagesRepo: "registry.example.com",
				Scheme:     registry_const.CEScheme,
			},
		},
		{
			name: "both empty get defaults",
			input: RegistrySettings{
				ImagesRepo: "",
				Scheme:     "",
			},
			expected: RegistrySettings{
				ImagesRepo: registry_const.CEImagesRepo,
				Scheme:     registry_const.CEScheme,
			},
		},
		{
			name: "no changes needed",
			input: RegistrySettings{
				ImagesRepo: "registry.example.com",
				Scheme:     "HTTP",
			},
			expected: RegistrySettings{
				ImagesRepo: "registry.example.com",
				Scheme:     "HTTP",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := tt.input
			settings.CorrectWithDefault()

			assert.Equal(t, tt.expected, settings)
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
				Mode:      registry_const.ModeDirect,
				Direct:    validRegistrySettings(),
				Unmanaged: nil,
			},
			output: output{
				err: false,
			},
		},
		{
			name: "valid unmanaged mode",
			input: DeckhouseSettings{
				Mode:      registry_const.ModeUnmanaged,
				Direct:    nil,
				Unmanaged: validRegistrySettings(),
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
				Mode:      registry_const.ModeDirect,
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
				Mode:      registry_const.ModeUnmanaged,
				Direct:    validRegistrySettings(),
				Unmanaged: validRegistrySettings(),
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
				Mode:      registry_const.ModeUnmanaged,
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
				Mode:      registry_const.ModeDirect,
				Direct:    validRegistrySettings(),
				Unmanaged: validRegistrySettings(),
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
				assert.Error(t, err)
				if tt.output.errMsg != "" {
					assert.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
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
			input: validRegistrySettings(),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings without credentials",
			input: validRegistrySettings(
				func(s *RegistrySettings) { s.Username = "" },
				func(s *RegistrySettings) { s.Password = "" },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with HTTP scheme and no CA",
			input: validRegistrySettings(
				func(s *RegistrySettings) { s.Scheme = registry_const.SchemeHTTP },
				func(s *RegistrySettings) { s.CA = "" },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with license only",
			input: validRegistrySettings(
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
			input: validRegistrySettings(
				func(s *RegistrySettings) { s.CheckMode = registry_const.CheckModeRelax },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "empty check mode is valid",
			input: validRegistrySettings(
				func(s *RegistrySettings) { s.CheckMode = "" },
			),
			output: output{
				err: false,
			},
		},

		// Invalid cases - ImagesRepo
		{
			name: "empty images repo",
			input: validRegistrySettings(
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
			input: validRegistrySettings(
				func(s *RegistrySettings) { s.Scheme = "ftp" },
			),
			output: output{
				err:    true,
				errMsg: "Invalid scheme",
			},
		},
		{
			name: "empty scheme",
			input: validRegistrySettings(
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
			input: validRegistrySettings(
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
			input: validRegistrySettings(
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
			input: validRegistrySettings(
				func(s *RegistrySettings) { s.License = "test-license" },
			),
			output: output{
				err:    true,
				errMsg: "License field must be empty when using credentials (username/password)",
			},
		},
		{
			name: "license with username only",
			input: validRegistrySettings(
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
			input: validRegistrySettings(
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
			input: validRegistrySettings(
				func(s *RegistrySettings) { s.Scheme = registry_const.SchemeHTTP },
			),
			output: output{
				err:    true,
				errMsg: "CA is not allowed when scheme is 'HTTP'",
			},
		},

		// Invalid cases - CheckMode
		{
			name: "invalid check mode",
			input: validRegistrySettings(
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
				assert.Error(t, err)
				if tt.output.errMsg != "" {
					assert.Contains(t, err.Error(), tt.output.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
