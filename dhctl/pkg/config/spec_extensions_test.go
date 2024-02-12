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

package config

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRulesExtension_sshPrivateKey(t *testing.T) {
	newStore := newSchemaStore([]string{"/tmp"})

	err := newStore.upload([]byte(`
kind: TestKind
apiVersions:
- apiVersion: test
  openAPISpec:
    type: object
    additionalProperties: false
    required: [key]
    x-rules: [sshPrivateKey]
    properties:
      kind:
        type: string
      apiVersion:
        type: string
      key:
        type: string
      passphrase:
        type: string
`))
	require.NoError(t, err)

	configFunc := func(config, keyPath string) string {
		return fmt.Sprintf(config, strings.Join(strings.Split(readFile(t, keyPath), "\n"), "\n  "))
	}

	tests := map[string]struct {
		content     string
		errContains string
	}{
		"ok without passphrase": {
			content: configFunc(`
kind: TestKind
apiVersion: test
key: |
  %s`,
				"./mocks/id_rsa",
			),
		},
		"fail without passphrase": {
			content: configFunc(`
kind: TestKind
apiVersion: test
key: |
  %s`,
				"./mocks/id_invalid_rsa",
			),
			errContains: "structure error: length too large",
		},
		"ok with passphrase": {
			content: configFunc(`
kind: TestKind
apiVersion: test
key: |
  %s
passphrase: test`,
				"./mocks/id_passphrase_rsa",
			),
		},
		"fail with no passphrase": {
			content: configFunc(`
kind: TestKind
apiVersion: test
key: |
  %s
`,
				"./mocks/id_passphrase_invalid_rsa",
			),
			errContains: "this private key is passphrase protected",
		},
		"fail with passphrase": {
			content: configFunc(`
kind: TestKind
apiVersion: test
passphrase: test
key: |
  %s
`,
				"./mocks/id_passphrase_invalid_rsa",
			),
			errContains: "structure error: tags don't match",
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			content := []byte(tt.content)

			_, err := newStore.ValidateWithOpts(&content, ValidateOptions{ValidateExtensions: true})
			if tt.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func readFile(t *testing.T, path string) string {
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
