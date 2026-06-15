/*
Copyright 2026 Flant JSC

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

package bashible

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/initsecret"
)

func TestContextBootstrapProxy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   ContextBootstrapProxy
		wantErr bool
	}{
		// Valid
		{
			name: "valid with all fields",
			input: ContextBootstrapProxy{
				Host:     "example.com",
				Path:     "/path",
				Scheme:   "https",
				Username: "user",
				Password: "pass",
				CA:       "---cert---",
				TTL:      "5m",
			},
			wantErr: false,
		},
		{
			name: "valid minimal proxy",
			input: ContextBootstrapProxy{
				Host:   "example.com",
				Path:   "/path",
				Scheme: "https",
			},
			wantErr: false,
		},
		// Invalid
		{
			name: "missing host",
			input: ContextBootstrapProxy{
				Path:   "/path",
				Scheme: "https",
			},
			wantErr: true,
		},
		{
			name: "missing path",
			input: ContextBootstrapProxy{
				Host:   "example.com",
				Scheme: "https",
			},
			wantErr: true,
		},
		{
			name: "missing scheme",
			input: ContextBootstrapProxy{
				Host: "example.com",
				Path: "/path",
			},
			wantErr: true,
		},
		{
			name: "username without password",
			input: ContextBootstrapProxy{
				Host:     "example.com",
				Path:     "/path",
				Scheme:   "https",
				Username: "user",
			},
			wantErr: true,
		},
		{
			name: "password without username",
			input: ContextBootstrapProxy{
				Host:     "example.com",
				Path:     "/path",
				Scheme:   "https",
				Password: "pass",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if err != nil {
				if e, ok := err.(validation.InternalError); ok {
					assert.Fail(t, "Internal validation error: %w", e.InternalError())
				}
			}

			if tt.wantErr {
				assert.Error(t, err, "Expected errors but got none")
			} else {
				assert.NoError(t, err, "Expected no errors but got some")
			}
		})
	}
}

func TestContextBootstrap_Validate(t *testing.T) {
	initConfig := initsecret.Config{
		CA: initsecret.CertKey{
			Cert: "---cert---",
			Key:  "---key---",
		},
		ROUser: initsecret.User{
			Name:         "ro_name",
			Password:     "ro_password",
			PasswordHash: "ro_password_hash",
		},
		RWUser: initsecret.User{
			Name:         "rw_name",
			Password:     "rw_password",
			PasswordHash: "rw_password_hash",
		},
	}

	proxyConfig := ContextBootstrapProxy{
		Host:   "example.com",
		Path:   "/path",
		Scheme: "https",
	}

	tests := []struct {
		name    string
		input   ContextBootstrap
		wantErr bool
	}{
		// Valid
		{
			name: "valid with all fields",
			input: ContextBootstrap{
				Init:  initConfig,
				Proxy: &proxyConfig,
			},
			wantErr: false,
		},
		{
			name: "valid without proxy",
			input: ContextBootstrap{
				Init: initConfig,
			},
			wantErr: false,
		},
		// Invalid
		{
			name: "missing init",
			input: ContextBootstrap{
				Proxy: &proxyConfig,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if err != nil {
				if e, ok := err.(validation.InternalError); ok {
					assert.Fail(t, "Internal validation error: %w", e.InternalError())
				}
			}

			if tt.wantErr {
				assert.Error(t, err, "Expected errors but got none")
			} else {
				assert.NoError(t, err, "Expected no errors but got some")
			}
		})
	}
}

func TestContextBootstrapProxy_ToMap(t *testing.T) {
	tests := []struct {
		name     string
		input    ContextBootstrapProxy
		expected map[string]any
	}{
		{
			name: "with all fields",
			input: ContextBootstrapProxy{
				Host:     "example.com",
				Path:     "/path",
				Scheme:   "https",
				Username: "user",
				Password: "pass",
				CA:       "---cert---",
				TTL:      "5m",
			},
			expected: map[string]any{
				"host":     "example.com",
				"path":     "/path",
				"scheme":   "https",
				"username": "user",
				"password": "pass",
				"ca":       "---cert---",
				"ttl":      "5m",
			},
		},
		{
			name: "with minimal proxy",
			input: ContextBootstrapProxy{
				Host:   "example.com",
				Path:   "/path",
				Scheme: "https",
			},
			expected: map[string]any{
				"host":   "example.com",
				"path":   "/path",
				"scheme": "https",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.ToMap()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContextBootstrap_ToMap(t *testing.T) {
	initConfig := initsecret.Config{
		CA: initsecret.CertKey{
			Cert: "---cert---",
			Key:  "---key---",
		},
		ROUser: initsecret.User{
			Name:         "ro_name",
			Password:     "ro_password",
			PasswordHash: "ro_password_hash",
		},
		RWUser: initsecret.User{
			Name:         "rw_name",
			Password:     "rw_password",
			PasswordHash: "rw_password_hash",
		},
	}

	proxyConfig := ContextBootstrapProxy{
		Host:   "example.com",
		Path:   "/path",
		Scheme: "https",
	}

	initConfigMap := initConfig.ToMap()
	proxyConfigMap := proxyConfig.ToMap()

	tests := []struct {
		name     string
		input    ContextBootstrap
		expected map[string]any
	}{
		{
			name: "with all fields",
			input: ContextBootstrap{
				Init:  initConfig,
				Proxy: &proxyConfig,
			},
			expected: map[string]any{
				"init":  initConfigMap,
				"proxy": proxyConfigMap,
			},
		},
		{
			name: "without proxy",
			input: ContextBootstrap{
				Init: initConfig,
			},
			expected: map[string]any{
				"init": initConfigMap,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.ToMap()
			assert.Equal(t, tt.expected, result)
		})
	}
}
