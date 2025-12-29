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
	type input struct {
		username string
		password string
		host     string
	}

	type expected struct {
		config string
		err    bool
	}

	tests := []struct {
		name     string
		input    input
		expected expected
	}{
		// Successful cases
		{
			name: "valid credentials",
			input: input{
				username: "foo",
				password: "bar",
				host:     "registry.io",
			},
			expected: expected{
				config: fmt.Sprintf(
					`{"auths":{"registry.io":{"username":"foo","password":"bar","auth":"%s"}}}`,
					base64.StdEncoding.EncodeToString([]byte("foo:bar")),
				),
			},
		},
		{
			name: "host with https scheme",
			input: input{
				username: "user",
				password: "1234",
				host:     "https://registry.io",
			},
			expected: expected{
				config: fmt.Sprintf(
					`{"auths":{"registry.io":{"username":"user","password":"1234","auth":"%s"}}}`,
					base64.StdEncoding.EncodeToString([]byte("user:1234")),
				),
			},
		},
		{
			name: "host with http scheme and port",
			input: input{
				username: "test",
				password: "pass",
				host:     "http://registry.io:5000",
			},
			expected: expected{
				config: fmt.Sprintf(
					`{"auths":{"registry.io:5000":{"username":"test","password":"pass","auth":"%s"}}}`,
					base64.StdEncoding.EncodeToString([]byte("test:pass")),
				),
			},
		},
		{
			name: "host with trailing slash",
			input: input{
				username: "test",
				password: "123",
				host:     "https://registry.io/",
			},
			expected: expected{
				config: fmt.Sprintf(
					`{"auths":{"registry.io":{"username":"test","password":"123","auth":"%s"}}}`,
					base64.StdEncoding.EncodeToString([]byte("test:123")),
				),
			},
		},
		{
			name: "empty username and password",
			input: input{
				username: "",
				password: "",
				host:     "https://registry.io",
			},
			expected: expected{
				config: `{"auths":{"registry.io":{}}}`,
			},
		},
		{
			name: "only username provided",
			input: input{
				username: "token",
				password: "",
				host:     "registry.io",
			},
			expected: expected{
				config: `{"auths":{"registry.io":{"username":"token"}}}`,
			},
		},
		{
			name: "only password provided",
			input: input{
				username: "",
				password: "secret",
				host:     "registry.io",
			},
			expected: expected{
				config: `{"auths":{"registry.io":{"password":"secret"}}}`,
			},
		},
		// Error cases
		{
			name: "invalid host URL",
			input: input{
				username: "x",
				password: "y",
				host:     "#bad:url",
			},
			expected: expected{
				err: true,
			},
		},
		{
			name: "empty host",
			input: input{
				username: "user",
				password: "pass",
				host:     "",
			},
			expected: expected{
				err: true,
			},
		},
		{
			name: "host without domain",
			input: input{
				username: "user",
				password: "pass",
				host:     "://",
			},
			expected: expected{
				err: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := DockerCfgFromCreds(tt.input.username, tt.input.password, tt.input.host)

			if tt.expected.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.JSONEq(t, tt.expected.config, string(cfg))
			}
		})
	}
}

func TestCredsFromDockerCfg(t *testing.T) {
	type input struct {
		config string
		host   string
	}

	type expected struct {
		username string
		password string
		err      bool
	}

	tests := []struct {
		name     string
		input    input
		expected expected
	}{
		// Successful cases: different authentication formats
		{
			name: "username and password fields",
			input: input{
				config: `{"auths":{"registry.io":{"username":"admin","password":"s3cr3t"}}}`,
				host:   "registry.io",
			},
			expected: expected{
				username: "admin",
				password: "s3cr3t",
			},
		},
		{
			name: "auth field only",
			input: input{
				config: fmt.Sprintf(
					`{"auths":{"registry.io":{"auth":"%s"}}}`,
					base64.StdEncoding.EncodeToString([]byte("foo:bar")),
				),
				host: "registry.io",
			},
			expected: expected{
				username: "foo",
				password: "bar",
			},
		},
		{
			name: "auth field overrides username/password",
			input: input{
				config: fmt.Sprintf(
					`{"auths":{"registry.io":{"username":"admin","password":"s3cr3t","auth":"%s"}}}`,
					base64.StdEncoding.EncodeToString([]byte("foo:bar")),
				),
				host: "registry.io",
			},
			expected: expected{
				username: "foo",
				password: "bar",
			},
		},
		// Successful cases: different auth base64 formats
		{
			name: "base64 auth with padding",
			input: input{
				config: `{"auths":{"registry.io":{"auth": "dXNlcjpwYXNzd29yZA=="}}}`, // user:password
				host:   "registry.io",
			},
			expected: expected{
				username: "user",
				password: "password",
			},
		},
		{
			name: "base64 auth without padding",
			input: input{
				config: `{"auths":{"registry.io":{"auth": "dXNlcjpwYXNz"}}}`, // user:pass
				host:   "registry.io",
			},
			expected: expected{
				username: "user",
				password: "pass",
			},
		},
		// Successful cases: empty values
		{
			name: "empty credentials with colon in auth",
			input: input{
				config: fmt.Sprintf(
					`{"auths":{"registry.io":{"username":"","password":"","auth":"%s"}}}`,
					base64.StdEncoding.EncodeToString([]byte(":")),
				),
				host: "registry.io",
			},
			expected: expected{
				username: "",
				password: "",
			},
		},
		{
			name: "empty auth field",
			input: input{
				config: `{"auths":{"registry.io":{"username":"","password":"","auth":""}}}`,
				host:   "registry.io",
			},
			expected: expected{
				username: "",
				password: "",
			},
		},
		{
			name: "empty credentials object",
			input: input{
				config: `{"auths":{"registry.io":{}}}`,
				host:   "registry.io",
			},
			expected: expected{
				username: "",
				password: "",
			},
		},
		{
			name: "empty auths",
			input: input{
				config: `{"auths":{}}`,
				host:   "registry.io",
			},
			expected: expected{
				username: "",
				password: "",
			},
		},
		{
			name: "empty config",
			input: input{
				config: `{}`,
				host:   "registry.io",
			},
			expected: expected{
				username: "",
				password: "",
			},
		},
		{
			name: "empty string",
			input: input{
				config: ``,
				host:   "registry.io",
			},
			expected: expected{
				username: "",
				password: "",
			},
		},
		// Successful cases: host lookup
		{
			name: "multiple hosts in config",
			input: input{
				config: `{"auths":{"devregistry.io":{"username":"dev","password":"devPassword"}, "testregistry.io":{"username":"test","password":"testPassword"}}}`,
				host:   "devregistry.io",
			},
			expected: expected{
				username: "dev",
				password: "devPassword",
			},
		},
		{
			name: "missing hosts in config",
			input: input{
				config: `{"auths":{"another.io":{"username":"x","password":"y"}}}`,
				host:   "not-found.io",
			},
			expected: expected{
				username: "",
				password: "",
			},
		},
		// Successful cases: host normalization
		{
			name: "host with port",
			input: input{
				config: `{"auths":{"registry.io:5000":{"username":"portuser","password":"portpass"}}}`,
				host:   "registry.io:5000",
			},
			expected: expected{
				username: "portuser",
				password: "portpass",
			},
		},
		// Error cases
		{
			name: "invalid JSON",
			input: input{
				config: `not-even-json`,
				host:   "registry.io",
			},
			expected: expected{
				err: true,
			},
		},
		{
			name: "malformed base64 in auth",
			input: input{
				config: `{"auths":{"registry.io":{"auth":"!!!invalid"}}}`,
				host:   "registry.io",
			},
			expected: expected{
				err: true,
			},
		},
		{
			name: "auth field without colon",
			input: input{
				config: fmt.Sprintf(
					`{"auths":{"registry.io":{"auth":"%s"}}}`,
					base64.StdEncoding.EncodeToString([]byte("user")),
				),
				host: "registry.io",
			},
			expected: expected{
				err: true,
			},
		},
		{
			name: "invalid host in request",
			input: input{
				config: `{"auths":{"registry.io":{"username":"user","password":"pass"}}}`,
				host:   "#bad:url",
			},
			expected: expected{
				err: true,
			},
		},
		{
			name: "invalid host in config",
			input: input{
				config: `{"auths":{"#bad:url":{"username":"user","password":"pass"}}}`,
				host:   "registry.io",
			},
			expected: expected{
				err: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			user, pass, err := CredsFromDockerCfg([]byte(tt.input.config), tt.input.host)

			if tt.expected.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected.username, user)
				require.Equal(t, tt.expected.password, pass)
			}
		})
	}
}

func TestNormalizeHost(t *testing.T) {
	type expected struct {
		host string
		err  bool
	}

	tests := []struct {
		name     string
		host     string
		expected expected
	}{
		// Successful cases
		{
			name: "plain domain",
			host: "docker.io",
			expected: expected{
				host: "docker.io",
			},
		},
		{
			name: "with www prefix",
			host: "www.docker.io",
			expected: expected{
				host: "www.docker.io",
			},
		},
		{
			name: "https scheme",
			host: "https://docker.io",
			expected: expected{
				host: "docker.io",
			},
		},
		{
			name: "http scheme",
			host: "http://example.com",
			expected: expected{
				host: "example.com",
			},
		},
		{
			name: "http with port",
			host: "http://example.com:5000",
			expected: expected{
				host: "example.com:5000",
			},
		},
		{
			name: "https with port",
			host: "https://example.com:443",
			expected: expected{
				host: "example.com:443",
			},
		},
		{
			name: "with path and query",
			host: "https://example.com:5000/v2/path?query=value",
			expected: expected{
				host: "example.com:5000",
			},
		},
		{
			name: "trailing slash",
			host: "https://docker.io/",
			expected: expected{
				host: "docker.io",
			},
		},
		{
			name: "IP address",
			host: "192.168.1.1",
			expected: expected{
				host: "192.168.1.1",
			},
		},
		{
			name: "IP address with port",
			host: "192.168.1.1:5000",
			expected: expected{
				host: "192.168.1.1:5000",
			},
		},
		{
			name: "localhost",
			host: "localhost",
			expected: expected{
				host: "localhost",
			},
		},
		{
			name: "localhost with port",
			host: "localhost:5000",
			expected: expected{
				host: "localhost:5000",
			},
		},
		// Error cases
		{
			name: "empty input",
			host: "",
			expected: expected{
				err: true,
			},
		},
		{
			name: "malformed URL",
			host: "#bad:url",
			expected: expected{
				err: true,
			},
		},
		{
			name: "only scheme",
			host: "http://",
			expected: expected{
				err: true,
			},
		},
		{
			name: "host without domain",
			host: "://",
			expected: expected{
				err: true,
			},
		},
		{
			name: "port without domain",
			host: ":9000",
			expected: expected{
				err: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			host, err := normalizeHost(tt.host)

			if tt.expected.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected.host, host)
			}
		})
	}
}
