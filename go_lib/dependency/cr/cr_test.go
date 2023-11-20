/*
Copyright 2023 Flant JSC

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

package cr

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	tests := []string{
		"registry.deckhouse.io/deckhouse/fe",
		"registry.deckhouse.io:5123/deckhouse/fe",
		"192.168.1.1/deckhouse/fe",
		"192.168.1.1:8080/deckhouse/fe",
		"2001:db8:3333:4444:5555:6666:7777:8888/deckhouse/fe",
		"[2001:db8::1]:8080/deckhouse/fe",
		"192.168.1.1:5123/deckhouse/fe",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			u, err := parse(tt)
			if err != nil {
				t.Errorf("got error: %s", err)
			}
			if u.String() != "//"+tt {
				t.Errorf("got: %s, wanted: %s", u, tt)
			}
		})
	}
}

func TestReadAuthConfig(t *testing.T) {
	t.Run("host match", func(t *testing.T) {
		auths := `
{
	"auths": {
		"registry.example.com:8032/modules": {
			"auth": "YTpiCg=="
		}
	}
}
`
		cfg := base64.RawStdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.NoError(t, err)
	})

	t.Run("path mismatch", func(t *testing.T) {
		auths := `
{
	"auths": {
		"registry.example.com:8032/foo/bar": {
			"auth": "YTpiCg=="
		}
	}
}
`
		cfg := base64.RawStdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.NoError(t, err)
	})

	t.Run("host mismatch", func(t *testing.T) {
		auths := `
{
	"auths": {
		"registry.invalid.com:8032/modules": {
			"auth": "YTpiCg=="
		}
	}
}
`
		cfg := base64.RawStdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.Error(t, err)
	})

	t.Run("port mismatch", func(t *testing.T) {
		auths := `
{
	"auths": {
		"registry.example.com:8033/foobar": {
			"auth": "YTpiCg=="
		}
	}
}
`
		cfg := base64.RawStdEncoding.EncodeToString([]byte(auths))
		_, err := readAuthConfig("registry.example.com:8032/modules", cfg)
		assert.Error(t, err)
	})
}
