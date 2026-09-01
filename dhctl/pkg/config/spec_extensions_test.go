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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

func TestRulesExtension_sshPrivateKey(t *testing.T) {
	newStore := newSchemaStore(&options.New().Global, []string{"/tmp"})

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
			errContains: "invalid ssh key: x509: decryption password incorrect",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := newStore.Validate(new([]byte(tt.content)), ValidateOptionValidateExtensions(true))
			if tt.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

// TestRulesExtension_sshPublicKey pins the half of x-rules that standalone bootstrap-phase
// base-infra used to escape. Every cloud provider puts the rule on the sshPublicKey of its
// ClusterConfiguration (sshKey on GCP), so once base-infra loads the config the way a full
// bootstrap loads it, a placeholder there stops the command instead of reaching the cloud API.
func TestRulesExtension_sshPublicKey(t *testing.T) {
	newStore := newSchemaStore(&options.New().Global, []string{"/tmp"})

	err := newStore.upload([]byte(`
kind: TestKind
apiVersions:
- apiVersion: test
  openAPISpec:
    type: object
    additionalProperties: false
    required: [sshPublicKey]
    properties:
      kind:
        type: string
      apiVersion:
        type: string
      sshPublicKey:
        type: string
        x-rules: [sshPublicKey]
`))
	require.NoError(t, err)

	tests := map[string]struct {
		key         string
		errContains string
	}{
		"ok, openssh format": {
			key: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC/HKZEdkiyMhe/Eztf9Ly2HgIf7ceBZrX0dM8by064C8R8Ei1S+EqdQDFsv/LIiEB9IrI5q6vQbx1HCr+bQN6Tm3rHrl00aAT4ce2R4QpxXAlCNBmjYlkc00giMQ4J5atIsVysvS3kDJgtoHSXi+NPwwv4wlfo2Q8rRzH6XT2gt/9l4qrdK5kMjJS3JVVHqYousl+pFHKR4ywtWXBUIRM2+zi68dbOLGql25+CEZILRopjoaxddsvYmrcQVJKUsbcFUgGMaZBPNAdmuFtm1OAWQCAJWFpkZzpPAgD/qhnIXPDx1MUE6YC7SDRu4pGfR8BEEcrwDR/igOIAMgudEAzv test@example.com",
		},
		"placeholder is refused": {
			key:         "<SSH_PUBLIC_KEY>",
			errContains: "failed to decode base64 string",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			content := fmt.Sprintf("kind: TestKind\napiVersion: test\nsshPublicKey: %q\n", tt.key)

			_, err := newStore.Validate(new([]byte(content)), ValidateOptionValidateExtensions(true))
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
