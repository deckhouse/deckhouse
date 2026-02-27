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
type proxySettingsOption func(*ProxySettings)

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
func proxySettingsBuilder(opts ...proxySettingsOption) *ProxySettings {
	settings := ProxySettings{
		RegistrySettings: RegistrySettings{
			ImagesRepo: "test:80/a/b/c/d",
			Scheme:     constant.SchemeHTTPS,
			CA:         "-----BEGIN CERTIFICATE-----",
			Username:   "test-user",
			Password:   "test-password",
			CheckMode:  constant.CheckModeDefault,
		},
		TTL: "5m",
	}

	for _, opt := range opts {
		opt(&settings)
	}

	return &settings
}

func TestDeckhouseSettings_Merge(t *testing.T) {
	tests := []struct {
		name     string
		input    DeckhouseSettings
		expected DeckhouseSettings
	}{
		{
			name: "empty mode",
			input: DeckhouseSettings{},
			expected: DeckhouseSettings{
				Mode: constant.ModeDirect,
				Direct: &RegistrySettings{
					ImagesRepo: constant.DefaultImagesRepo,
					Scheme:     constant.DefaultScheme,
				},
			},
		},
		{
			name: "mode direct",
			input: DeckhouseSettings{
				Mode: constant.ModeDirect,
			},
			expected: DeckhouseSettings{
				Mode: constant.ModeDirect,
				Direct: &RegistrySettings{
					ImagesRepo: constant.DefaultImagesRepo,
					Scheme:     constant.DefaultScheme,
				},
			},
		},
		{
			name: "mode direct no overrides",
			input: DeckhouseSettings{
				Mode:   constant.ModeDirect,
				Direct: registrySettingsBuilder(),
			},
			expected: DeckhouseSettings{
				Mode:   constant.ModeDirect,
				Direct: registrySettingsBuilder(),
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
					ImagesRepo: constant.DefaultImagesRepo,
					Scheme:     constant.DefaultScheme,
				},
			},
		},
		{
			name: "mode unmanaged no overrides",
			input: DeckhouseSettings{
				Mode:      constant.ModeUnmanaged,
				Unmanaged: registrySettingsBuilder(),
			},
			expected: DeckhouseSettings{
				Mode:      constant.ModeUnmanaged,
				Unmanaged: registrySettingsBuilder(),
			},
		},
		{
			name: "mode proxy",
			input: DeckhouseSettings{
				Mode: constant.ModeProxy,
			},
			expected: DeckhouseSettings{
				Mode: constant.ModeProxy,
				Proxy: &ProxySettings{
					RegistrySettings: RegistrySettings{
						ImagesRepo: constant.DefaultImagesRepo,
						Scheme:     constant.DefaultScheme,
					},
				},
			},
		},
		{
			name: "mode proxy no overrides",
			input: DeckhouseSettings{
				Mode:  constant.ModeProxy,
				Proxy: proxySettingsBuilder(),
			},
			expected: DeckhouseSettings{
				Mode:  constant.ModeProxy,
				Proxy: proxySettingsBuilder(),
			},
		},
		{
			name: "mode local",
			input: DeckhouseSettings{
				Mode: constant.ModeLocal,
			},
			expected: DeckhouseSettings{
				Mode: constant.ModeLocal,
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
			settings := New(tt.input.Mode).
				Merge(&tt.input)

			require.EqualValues(t, tt.expected, settings)
		})
	}

	t.Run("merge with nil other returns copy of base", func(t *testing.T) {
		base := New(constant.ModeDirect)
		merged := base.Merge(nil)
		require.EqualValues(t, base, merged)
	})
}

func TestRegistrySettings_Merge(t *testing.T) {
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
				ImagesRepo: constant.DefaultImagesRepo,
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
				Scheme:     constant.DefaultScheme,
			},
		},
		{
			name:  "default ImagesRepo and Scheme",
			input: nil,
			expected: RegistrySettings{
				ImagesRepo: constant.DefaultImagesRepo,
				Scheme:     constant.DefaultScheme,
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
			settings := NewRegistrySettings().
				Merge(tt.input)

			require.Equal(t, tt.expected, settings)
		})
	}

	t.Run("merge with nil other returns copy of base", func(t *testing.T) {
		base := NewRegistrySettings()
		merged := base.Merge(nil)
		require.Equal(t, base, merged)
	})
}

func TestProxySettings_Merge(t *testing.T) {
	tests := []struct {
		name     string
		input    *ProxySettings
		expected ProxySettings
	}{
		{
			name: "default ImagesRepo",
			input: &ProxySettings{
				RegistrySettings: RegistrySettings{
					ImagesRepo: "",
					Scheme:     "HTTPS",
				},
			},
			expected: ProxySettings{
				RegistrySettings: RegistrySettings{
					ImagesRepo: constant.DefaultImagesRepo,
					Scheme:     "HTTPS",
				},
			},
		},
		{
			name: "default Scheme",
			input: &ProxySettings{
				RegistrySettings: RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     "",
				},
			},
			expected: ProxySettings{
				RegistrySettings: RegistrySettings{
					ImagesRepo: "registry.example.com",
					Scheme:     constant.DefaultScheme,
				},
			},
		},
		{
			name:  "default ImagesRepo and Scheme",
			input: nil,
			expected: ProxySettings{
				RegistrySettings: RegistrySettings{
					ImagesRepo: constant.DefaultImagesRepo,
					Scheme:     constant.DefaultScheme,
				},
			},
		},
		{
			name:     "full",
			input:    proxySettingsBuilder(),
			expected: *proxySettingsBuilder(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := ProxySettings{
				RegistrySettings: NewRegistrySettings(),
			}
			settings = settings.Merge(tt.input)

			require.Equal(t, tt.expected, settings)
		})
	}

	t.Run("merge with nil other returns copy of base", func(t *testing.T) {
		base := ProxySettings{
			RegistrySettings: NewRegistrySettings(),
			TTL:              "5m",
		}

		merged := base.Merge(nil)
		require.Equal(t, base, merged)
	})
}

func TestDeckhouseSettings_ToMap(t *testing.T) {
	tests := []struct {
		name   string
		input  DeckhouseSettings
		output map[string]any
	}{
		{
			name: "mode direct",
			input: DeckhouseSettings{
				Mode:   constant.ModeDirect,
				Direct: registrySettingsBuilder(),
			},
			output: map[string]any{
				"mode": "Direct",
				"direct": map[string]any{
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
			name: "mode unmanaged",
			input: DeckhouseSettings{
				Mode:      constant.ModeUnmanaged,
				Unmanaged: registrySettingsBuilder(),
			},
			output: map[string]any{
				"mode": "Unmanaged",
				"unmanaged": map[string]any{
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
			name: "mode proxy",
			input: DeckhouseSettings{
				Mode:  constant.ModeProxy,
				Proxy: proxySettingsBuilder(),
			},
			output: map[string]any{
				"mode": "Proxy",
				"proxy": map[string]any{
					"imagesRepo": "test:80/a/b/c/d",
					"scheme":     "HTTPS",
					"username":   "test-user",
					"password":   "test-password",
					"ca":         "-----BEGIN CERTIFICATE-----",
					"checkMode":  "Default",
					"ttl":        "5m",
				},
			},
		},
		{
			name: "mode local",
			input: DeckhouseSettings{
				Mode: constant.ModeLocal,
			},
			output: map[string]any{
				"mode": "Local",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.EqualValues(t, tt.output, tt.input.ToMap())
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

func TestProxySettings_ToMap(t *testing.T) {
	tests := []struct {
		name   string
		input  ProxySettings
		output map[string]any
	}{
		{
			name:  "all fields",
			input: *proxySettingsBuilder(),
			output: map[string]any{
				"imagesRepo": "test:80/a/b/c/d",
				"scheme":     "HTTPS",
				"username":   "test-user",
				"password":   "test-password",
				"ca":         "-----BEGIN CERTIFICATE-----",
				"checkMode":  "Default",
				"ttl":        "5m",
			},
		},
		{
			name: "optional fields",
			input: *proxySettingsBuilder(
				func(rs *ProxySettings) {
					rs.Username = ""
					rs.Password = ""
					rs.CA = ""
					rs.CheckMode = ""
					rs.TTL = ""
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
				Mode:   constant.ModeDirect,
				Direct: registrySettingsBuilder(),
			},
			output: output{
				err: false,
			},
		},
		{
			name: "valid unmanaged mode",
			input: DeckhouseSettings{
				Mode:      constant.ModeUnmanaged,
				Unmanaged: registrySettingsBuilder(),
			},
			output: output{
				err: false,
			},
		},
		{
			name: "valid proxy mode",
			input: DeckhouseSettings{
				Mode:  constant.ModeProxy,
				Proxy: proxySettingsBuilder(),
			},
			output: output{
				err: false,
			},
		},
		{
			name: "valid local mode",
			input: DeckhouseSettings{
				Mode: constant.ModeLocal,
			},
			output: output{
				err: false,
			},
		},
		// Invalid cases - Mode
		{
			name: "invalid mode",
			input: DeckhouseSettings{
				Mode: "invalid-mode",
			},
			output: output{
				err:    true,
				errMsg: "Unknown registry mode",
			},
		},
		{
			name: "empty mode",
			input: DeckhouseSettings{
				Mode: "",
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
				Mode: constant.ModeDirect,
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
				errMsg: "Section 'direct' must be empty",
			},
		},
		// Invalid cases - Unmanaged mode validation
		{
			name: "unmanaged mode without unmanaged settings",
			input: DeckhouseSettings{
				Mode: constant.ModeUnmanaged,
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
				errMsg: "Section 'unmanaged' must be empty",
			},
		},
		// Invalid cases - Proxy mode validation
		{
			name: "proxy mode without proxy settings",
			input: DeckhouseSettings{
				Mode: constant.ModeProxy,
			},
			output: output{
				err:    true,
				errMsg: "proxy: is required",
			},
		},
		{
			name: "non-proxy mode with proxy settings",
			input: DeckhouseSettings{
				Mode:   constant.ModeDirect,
				Direct: registrySettingsBuilder(),
				Proxy:  proxySettingsBuilder(),
			},
			output: output{
				err:    true,
				errMsg: "Section 'proxy' must be empty",
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
			name: "valid settings with full images repo (host:port/path)",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.ImagesRepo = "registry-test.io:8080/a/b/c/d" },
			),
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
		{
			name: "images repo with trailing slash after path",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.ImagesRepo = "test:80/a/b/c/d/" },
			),
			output: output{
				err:    true,
				errMsg: "does not match the regexp pattern",
			},
		},
		{
			name: "images repo with only host, port and trailing slash",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.ImagesRepo = "test:80/" },
			),
			output: output{
				err:    true,
				errMsg: "does not match the regexp pattern",
			},
		},
		{
			name: "images repo with multiple consecutive slashes",
			input: registrySettingsBuilder(
				func(s *RegistrySettings) { s.ImagesRepo = "test:80////a" },
			),
			output: output{
				err:    true,
				errMsg: "does not match the regexp pattern",
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

func TestProxySettings_Validate(t *testing.T) {
	type output struct {
		err    bool
		errMsg string
	}

	tests := []struct {
		name   string
		input  *ProxySettings
		output output
	}{
		// Valid cases
		{
			name:  "valid settings with all fields",
			input: proxySettingsBuilder(),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with full images repo (host:port/path)",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.ImagesRepo = "registry-test.io:8080/a/b/c/d" },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings without credentials",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.Username = "" },
				func(s *ProxySettings) { s.Password = "" },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with HTTP scheme and no CA",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.Scheme = constant.SchemeHTTP },
				func(s *ProxySettings) { s.CA = "" },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with license only",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.Username = "" },
				func(s *ProxySettings) { s.Password = "" },
				func(s *ProxySettings) { s.License = "test-license" },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with relaxed check mode",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.CheckMode = constant.CheckModeRelax },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "empty check mode is valid",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.CheckMode = "" },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with ttl 5m",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.TTL = "5m" },
			),
			output: output{
				err: false,
			},
		},
		{
			name: "valid settings with empty ttl",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.TTL = "" },
			),
			output: output{
				err: false,
			},
		},

		// Invalid cases - ImagesRepo
		{
			name: "empty images repo",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.ImagesRepo = "" },
			),
			output: output{
				err:    true,
				errMsg: "Field 'imagesRepo' is required",
			},
		},
		{
			name: "images repo with trailing slash after path",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.ImagesRepo = "test:80/a/b/c/d/" },
			),
			output: output{
				err:    true,
				errMsg: "does not match the regexp pattern",
			},
		},
		{
			name: "images repo with only host, port and trailing slash",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.ImagesRepo = "test:80/" },
			),
			output: output{
				err:    true,
				errMsg: "does not match the regexp pattern",
			},
		},
		{
			name: "images repo with multiple consecutive slashes",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.ImagesRepo = "test:80////a" },
			),
			output: output{
				err:    true,
				errMsg: "does not match the regexp pattern",
			},
		},

		// Invalid cases - Scheme
		{
			name: "invalid scheme",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.Scheme = "ftp" },
			),
			output: output{
				err:    true,
				errMsg: "Invalid scheme",
			},
		},
		{
			name: "empty scheme",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.Scheme = "" },
			),
			output: output{
				err:    true,
				errMsg: "Invalid scheme",
			},
		},

		// Invalid cases - Credentials
		{
			name: "password without username",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.Username = "" },
				func(s *ProxySettings) { s.Password = "test-password" },
			),
			output: output{
				err:    true,
				errMsg: "Username is required when password is provided",
			},
		},
		{
			name: "username without password",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.Username = "test-user" },
				func(s *ProxySettings) { s.Password = "" },
			),
			output: output{
				err:    true,
				errMsg: "Password is required when username is provided",
			},
		},

		// Invalid cases - License
		{
			name: "license with credentials",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.License = "test-license" },
			),
			output: output{
				err:    true,
				errMsg: "License field must be empty when using credentials (username/password)",
			},
		},
		{
			name: "license with username only",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.Password = "" },
				func(s *ProxySettings) { s.License = "test-license" },
			),
			output: output{
				err:    true,
				errMsg: "License field must be empty when using credentials (username/password)",
			},
		},
		{
			name: "license with password only",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.Username = "" },
				func(s *ProxySettings) { s.License = "test-license" },
			),
			output: output{
				err:    true,
				errMsg: "License field must be empty when using credentials (username/password)",
			},
		},

		// Invalid cases - CA
		{
			name: "CA with HTTP scheme",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.Scheme = constant.SchemeHTTP },
			),
			output: output{
				err:    true,
				errMsg: "CA is not allowed when scheme is 'HTTP'",
			},
		},

		// Invalid cases - CheckMode
		{
			name: "invalid check mode",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.CheckMode = "invalid-mode" },
			),
			output: output{
				err:    true,
				errMsg: "unknown registry check mode",
			},
		},

		// Invalid cases - TTL
		{
			name: "invalid TTL regexp",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.TTL = "invalid-TTL" },
			),
			output: output{
				err:    true,
				errMsg: "does not match required pattern",
			},
		},
		{
			name: "invalid TTL duration",
			input: proxySettingsBuilder(
				func(s *ProxySettings) { s.TTL = "4m59s" },
			),
			output: output{
				err:    true,
				errMsg: "must be at least",
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

func TestDeckhouseSettings_DeepCopy(t *testing.T) {
	t.Run("should create a deep copy of DeckhouseSettings", func(t *testing.T) {
		original := &DeckhouseSettings{
			Mode:      constant.ModeDirect,
			Direct:    registrySettingsBuilder(),
			Unmanaged: registrySettingsBuilder(),
			Proxy:     proxySettingsBuilder(),
		}

		copied := original.DeepCopy()
		require.NotNil(t, copied)
		require.NotSame(t, original, copied)
		require.EqualValues(t, original, copied)
	})

	t.Run("should handle nil receiver", func(t *testing.T) {
		var nilSettings *DeckhouseSettings
		copied := nilSettings.DeepCopy()
		require.Nil(t, copied)
	})
}

func TestRegistrySettings_DeepCopy(t *testing.T) {
	t.Run("should create a deep copy of RegistrySettings", func(t *testing.T) {
		var original *RegistrySettings = registrySettingsBuilder()

		copied := original.DeepCopy()
		require.NotNil(t, copied)
		require.NotSame(t, original, copied)
		require.EqualValues(t, original, copied)
	})

	t.Run("should handle nil receiver", func(t *testing.T) {
		var nilSettings *RegistrySettings
		copied := nilSettings.DeepCopy()
		require.Nil(t, copied)
	})
}

func TestProxySettings_DeepCopy(t *testing.T) {
	t.Run("should create a deep copy of ProxySettings", func(t *testing.T) {
		var original *ProxySettings = proxySettingsBuilder()

		copied := original.DeepCopy()
		require.NotNil(t, copied)
		require.NotSame(t, original, copied)
		require.EqualValues(t, original, copied)
	})

	t.Run("should handle nil receiver", func(t *testing.T) {
		var nilSettings *ProxySettings
		copied := nilSettings.DeepCopy()
		require.Nil(t, copied)
	})
}
