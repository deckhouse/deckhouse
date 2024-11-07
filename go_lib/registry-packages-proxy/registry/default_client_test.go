// Copyright 2024 Flant JSC
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

package registry

import (
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/require"
)

func TestRegistrySchemeCalculation(t *testing.T) {

	var tests = []struct {
		name                  string
		config                ClientConfig
		path                  string
		strictValidationError bool
		schemeError           bool
	}{
		{
			name: "registry on internet domain name, on standard port, protocol https",
			config: ClientConfig{
				Repository: "registry.test.com",
				Scheme:     "https",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on internet domain name, on standard port, protocol https, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "registry.test.com",
				Scheme:     "https",
			},
			path:                  "",
			strictValidationError: true,
		},
		{
			name: "registry on internet domain name, on standard port, protocol http",
			config: ClientConfig{
				Repository: "registry.test.com",
				Scheme:     "http",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on internet domain name, on standard port, protocol http, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "registry.test.com",
				Scheme:     "http",
			},
			path:                  "",
			strictValidationError: true,
		},

		{
			name: "registry on internet domain name, on non-standard port, protocol https",
			config: ClientConfig{
				Repository: "registry.test.com:5000",
				Scheme:     "https",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on internet domain name, on non-standard port, protocol https, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "registry.test.com:5000",
				Scheme:     "https",
			},
			path:                  "",
			strictValidationError: true,
		},
		{
			name: "registry on internet domain name, on non-standard port, protocol http",
			config: ClientConfig{
				Repository: "registry.test.com:5000",
				Scheme:     "http",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on internet domain name, on non-standard port, protocol http, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "registry.test.com:5000",
				Scheme:     "http",
			},
			path:                  "",
			strictValidationError: true,
		},

		{
			name: "registry on special domain name, on standard port, protocol https",
			config: ClientConfig{
				Repository: "registry.internal",
				Scheme:     "https",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on special domain name, on standard port, protocol https, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "registry.internal",
				Scheme:     "https",
			},
			path:                  "",
			strictValidationError: true,
		},
		{
			name: "registry on special domain name, on standard port, protocol http",
			config: ClientConfig{
				Repository: "registry.internal",
				Scheme:     "http",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on special domain name, on standard port, protocol http, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "registry.internal",
				Scheme:     "http",
			},
			path:                  "",
			strictValidationError: true,
		},

		{
			name: "registry on special domain name, on non-standard port, protocol https",
			config: ClientConfig{
				Repository: "registry.internal:5000",
				Scheme:     "https",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on special domain name, on non-standard port, protocol https, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "registry.internal:5000",
				Scheme:     "https",
			},
			path:                  "",
			strictValidationError: true,
		},
		{
			name: "registry on special domain name, on non-standard port, protocol http",
			config: ClientConfig{
				Repository: "registry.internal:5000",
				Scheme:     "http",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on special domain name, on non-standard port, protocol http, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "registry.internal:5000",
				Scheme:     "http",
			},
			path:                  "",
			strictValidationError: true,
		},

		{
			name: "registry on public ip address, on standard port, protocol https",
			config: ClientConfig{
				Repository: "8.8.8.8",
				Scheme:     "https",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on public ip address, on standard port, protocol https, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "8.8.8.8",
				Scheme:     "https",
			},
			path:                  "",
			strictValidationError: true,
		},
		{
			name: "registry on public ip address, on standard port, protocol http",
			config: ClientConfig{
				Repository: "8.8.8.8",
				Scheme:     "http",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on public ip address, on standard port, protocol http, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "8.8.8.8",
				Scheme:     "http",
			},
			path:                  "",
			strictValidationError: true,
		},

		{
			name: "registry on public ip address, on non-standard port, protocol https",
			config: ClientConfig{
				Repository: "8.8.8.8:5000",
				Scheme:     "https",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on public ip address, on non-standard port, protocol https, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "8.8.8.8:5000",
				Scheme:     "https",
			},
			path:                  "",
			strictValidationError: true,
		},
		{
			name: "registry on public ip address, on non-standard port, protocol http",
			config: ClientConfig{
				Repository: "8.8.8.8:5000",
				Scheme:     "http",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on public ip address, on non-standard port, protocol http, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "8.8.8.8:5000",
				Scheme:     "http",
			},
			path:                  "",
			strictValidationError: true,
		},

		{
			name: "registry on private ip address, on standard port, protocol https, should fail due to scheme error",
			config: ClientConfig{
				Repository: "192.168.1.1",
				Scheme:     "https",
			},
			path:        "/deckhouse",
			schemeError: true,
		},
		{
			name: "registry on private ip address, on standard port, protocol https, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "192.168.1.1",
				Scheme:     "https",
			},
			path:                  "",
			strictValidationError: true,
		},
		{
			name: "registry on private ip address, on standard port, protocol http",
			config: ClientConfig{
				Repository: "192.168.1.1",
				Scheme:     "http",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on private ip address, on standard port, protocol http, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "192.168.1.1",
				Scheme:     "http",
			},
			path:                  "",
			strictValidationError: true,
		},

		{
			name: "registry on private ip address, on non-standard port, protocol https, should fail due to scheme error",
			config: ClientConfig{
				Repository: "192.168.1.1:5000",
				Scheme:     "https",
			},
			path:        "/deckhouse",
			schemeError: true,
		},
		{
			name: "registry on private ip address, on non-standard port, protocol https, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "192.168.1.1:5000",
				Scheme:     "https",
			},
			path:                  "",
			strictValidationError: true,
		},
		{
			name: "registry on private ip address, on non-standard port, protocol http",
			config: ClientConfig{
				Repository: "192.168.1.1:5000",
				Scheme:     "http",
			},
			path: "/deckhouse",
		},
		{
			name: "registry on private ip address, on non-standard port, protocol http, path is empty, should fail due to strict validation error",
			config: ClientConfig{
				Repository: "192.168.1.1:5000",
				Scheme:     "http",
			},
			path:                  "",
			strictValidationError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := test.config.Repository
			if test.path != "" {
				repo = fmt.Sprintf("%s/%s", repo, test.path)
			}

			nameOpts := newNameOptions(test.config.Scheme)
			repository, err := name.NewRepository(repo, nameOpts...)
			if test.strictValidationError {
				t.Log(err)
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if test.schemeError {
				t.Logf("Expected scheme:%s, actual scheme: %s", test.config.Scheme, repository.Scheme())
				require.NotEqual(t, repository.Scheme(), test.config.Scheme)
				return
			}
			require.Equal(t, repository.Scheme(), test.config.Scheme)
		})
	}
}
