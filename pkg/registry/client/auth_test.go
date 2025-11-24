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

package client

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "registry without scheme",
			input:    "registry.deckhouse.io/deckhouse/fe",
			expected: "//registry.deckhouse.io/deckhouse/fe",
		},
		{
			name:     "registry with port",
			input:    "registry.deckhouse.io:5123/deckhouse/fe",
			expected: "//registry.deckhouse.io:5123/deckhouse/fe",
		},
		{
			name:     "IP address",
			input:    "192.168.1.1/deckhouse/fe",
			expected: "//192.168.1.1/deckhouse/fe",
		},
		{
			name:     "IP address with port",
			input:    "192.168.1.1:8080/deckhouse/fe",
			expected: "//192.168.1.1:8080/deckhouse/fe",
		},
		{
			name:     "IPv6 address",
			input:    "2001:db8:3333:4444:5555:6666:7777:8888/deckhouse/fe",
			expected: "//2001:db8:3333:4444:5555:6666:7777:8888/deckhouse/fe",
		},
		{
			name:     "IPv6 address with port",
			input:    "[2001:db8::1]:8080/deckhouse/fe",
			expected: "//[2001:db8::1]:8080/deckhouse/fe",
		},
		{
			name:     "IP with port",
			input:    "192.168.1.1:5123/deckhouse/fe",
			expected: "//192.168.1.1:5123/deckhouse/fe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := parse(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, u.String())
		})
	}
}

func TestReadAuthConfig(t *testing.T) {
	t.Run("host match", func(t *testing.T) {
		auths := `{
	"auths": {
		"registry.example.com:8032/modules": {
			"auth": "dXNlcjpi",
			"email": "user@example.com"
		}
	}
}`
		cfg := base64.StdEncoding.EncodeToString([]byte(auths))
		authConfig, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.NoError(t, err)
		assert.Equal(t, "user", authConfig.Username)
		assert.Equal(t, "b", authConfig.Password)
	})

	t.Run("path mismatch but host match", func(t *testing.T) {
		auths := `{
	"auths": {
		"registry.example.com:8032/foo/bar": {
			"auth": "dXNlcjpi",
			"email": "user@example.com"
		}
	}
}`
		cfg := base64.StdEncoding.EncodeToString([]byte(auths))
		authConfig, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.NoError(t, err)
		assert.Equal(t, "user", authConfig.Username)
		assert.Equal(t, "b", authConfig.Password)
	})

	t.Run("host mismatch", func(t *testing.T) {
		auths := `{
	"auths": {
		"registry.invalid.com:8032/modules": {
			"auth": "YTpi"
		}
	}
}`
		cfg := base64.StdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `"registry.example.com:8032/modules" credentials not found in the dockerCfg`)
	})

	t.Run("port mismatch", func(t *testing.T) {
		auths := `{
	"auths": {
		"registry.example.com:8033/foobar": {
			"auth": "YTpi"
		}
	}
}`
		cfg := base64.StdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `"registry.example.com:8032/modules" credentials not found in the dockerCfg`)
	})

	t.Run("multiple registries - correct host match", func(t *testing.T) {
		auths := `{
	"auths": {
		"registry.invalid.com:8032/modules": {
			"auth": "aW52YWxpZDppbnZhbGlk"
		},
		"registry.example.com:8032/modules": {
			"auth": "dmFsaWQ6dmFsaWQ=",
			"email": "valid@example.com"
		},
		"another.registry.com": {
			"auth": "YW5vdGhlcjphbm90aGVy"
		}
	}
}`
		cfg := base64.StdEncoding.EncodeToString([]byte(auths))
		authConfig, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.NoError(t, err)
		assert.Equal(t, "valid", authConfig.Username)
		assert.Equal(t, "valid", authConfig.Password)
	})

	t.Run("invalid base64 - fallback to plain text", func(t *testing.T) {
		auths := `{
	"auths": {
		"registry.example.com:8032/modules": {
			"auth": "dXNlcjpi",
			"email": "user@example.com"
		}
	}
}`
		// Pass plain JSON instead of base64
		authConfig, err := readAuthConfig("registry.example.com:8032/modules", auths)
		assert.NoError(t, err)
		assert.Equal(t, "user", authConfig.Username)
		assert.Equal(t, "b", authConfig.Password)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		cfg := base64.StdEncoding.EncodeToString([]byte(`{"auths": {invalid json`))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal docker config")
	})

	t.Run("missing auths field", func(t *testing.T) {
		auths := `{
	"other_field": "value"
}`
		cfg := base64.StdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `"registry.example.com:8032/modules" credentials not found in the dockerCfg`)
	})

	t.Run("empty auths", func(t *testing.T) {
		auths := `{
	"auths": {}
}`
		cfg := base64.StdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `"registry.example.com:8032/modules" credentials not found in the dockerCfg`)
	})

	t.Run("malformed auth entry", func(t *testing.T) {
		auths := `{
	"auths": {
		"registry.example.com:8032/modules": "not an object"
	}
}`
		cfg := base64.StdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal")
	})

	t.Run("registry with scheme in config", func(t *testing.T) {
		auths := `{
	"auths": {
		"https://registry.example.com:8032/v2/": {
			"auth": "dXNlcjpi",
			"email": "user@example.com"
		}
	}
}`
		cfg := base64.StdEncoding.EncodeToString([]byte(auths))
		authConfig, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.NoError(t, err)
		assert.Equal(t, "user", authConfig.Username)
		assert.Equal(t, "b", authConfig.Password)
	})

	t.Run("IPv6 registry", func(t *testing.T) {
		auths := `{
	"auths": {
		"[2001:db8::1]:8080": {
			"auth": "dXNlcjpwYXNz",
			"email": "ipv6@example.com"
		}
	}
}`
		cfg := base64.StdEncoding.EncodeToString([]byte(auths))
		authConfig, err := readAuthConfig("[2001:db8::1]:8080/repo", cfg)
		assert.NoError(t, err)
		assert.Equal(t, "user", authConfig.Username)
		assert.Equal(t, "pass", authConfig.Password)
	})
}
