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

package helpers

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerCfgFromCreds(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		password  string
		host      string
		wantCfg   string
		wantError bool
	}{
		{
			name:     "valid credentials",
			username: "foo",
			password: "bar",
			host:     "registry.io",
			wantCfg: fmt.Sprintf(
				`{"auths":{"registry.io":{"username":"foo","password":"bar","auth":"%s"}}}`,
				base64.StdEncoding.EncodeToString([]byte("foo:bar")),
			),
		},
		{
			name:     "host with scheme",
			username: "user",
			password: "1234",
			host:     "https://registry.io",
			wantCfg: fmt.Sprintf(
				`{"auths":{"registry.io":{"username":"user","password":"1234","auth":"%s"}}}`,
				base64.StdEncoding.EncodeToString([]byte("user:1234")),
			),
		},
		{
			name:     "host with trailing slash",
			username: "test",
			password: "123",
			host:     "https://registry.io/",
			wantCfg: fmt.Sprintf(
				`{"auths":{"registry.io":{"username":"test","password":"123","auth":"%s"}}}`,
				base64.StdEncoding.EncodeToString([]byte("test:123")),
			),
		},
		{
			name:     "empty credentials",
			username: "",
			password: "",
			host:     "https://registry.io",
			wantCfg: fmt.Sprintf(
				`{"auths":{"registry.io":{"auth":"%s"}}}`,
				base64.StdEncoding.EncodeToString([]byte(":")),
			),
		},
		{
			name:      "invalid host",
			username:  "x",
			password:  "y",
			host:      "#bad:url",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := DockerCfgFromCreds(tt.username, tt.password, tt.host)

			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.JSONEq(t, tt.wantCfg, string(cfg))
			}
		})
	}
}

func TestCredsFromDockerCfg(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		host      string
		wantUser  string
		wantPass  string
		wantError bool
	}{
		{
			name:     "username and password present",
			config:   `{"auths":{"registry.io":{"username":"admin","password":"s3cr3t"}}}`,
			host:     "registry.io",
			wantUser: "admin",
			wantPass: "s3cr3t",
		},
		{
			name: "auth field only",
			config: fmt.Sprintf(
				`{"auths":{"registry.io":{"auth":"%s"}}}`,
				base64.StdEncoding.EncodeToString([]byte("foo:bar")),
			),
			host:     "registry.io",
			wantUser: "foo",
			wantPass: "bar",
		},
		{
			name: "auth field overrides username/password",
			config: fmt.Sprintf(
				`{"auths":{"registry.io":{"username":"admin","password":"s3cr3t","auth":"%s"}}}`,
				base64.StdEncoding.EncodeToString([]byte("foo:bar")),
			),
			host:     "registry.io",
			wantUser: "foo",
			wantPass: "bar",
		},
		{
			name:     "empty credentials object",
			config:   `{"auths":{"registry.io":{}}}`,
			host:     "registry.io",
			wantUser: "",
			wantPass: "",
		},
		{
			name: "empty fields",
			config: fmt.Sprintf(
				`{"auths":{"registry.io":{"username":"","password":"","auth":"%s"}}}`,
				base64.StdEncoding.EncodeToString([]byte(":")),
			),
			host:     "registry.io",
			wantUser: "",
			wantPass: "",
		},
		{
			name:     "missing host entry",
			config:   `{"auths":{"another.io":{"username":"x","password":"y"}}}`,
			host:     "not-found.io",
			wantUser: "",
			wantPass: "",
		},
		{
			name:      "invalid JSON",
			config:    `not-even-json`,
			host:      "registry.io",
			wantError: true,
		},
		{
			name:      "invalid base64 in auth",
			config:    `{"auths":{"registry.io":{"auth":"!!!invalid"}}}`,
			host:      "registry.io",
			wantError: true,
		},
		{
			name:     "empty credentials strings",
			config:   `{"auths":{"registry.io":{"username":"","password":"","auth":""}}}`,
			host:     "registry.io",
			wantUser: "",
			wantPass: "",
		},
		{
			name:     "missing auths section",
			config:   `{}`,
			host:     "registry.io",
			wantUser: "",
			wantPass: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			user, pass, err := CredsFromDockerCfg([]byte(tt.config), tt.host)

			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantUser, user)
				require.Equal(t, tt.wantPass, pass)
			}
		})
	}
}

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "plain host",
			input:    "docker.io",
			expected: "docker.io",
		},
		{
			name:     "https scheme",
			input:    "https://docker.io",
			expected: "docker.io",
		},
		{
			name:     "http with port",
			input:    "http://example.com:5000",
			expected: "example.com:5000",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "malformed URL",
			input:   "#bad:url",
			wantErr: true,
		},
		{
			name:     "trailing slash",
			input:    "https://docker.io/",
			expected: "docker.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := normalizeHost(tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}
