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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"
)

func TestLoadDHCTLConfigSchema(t *testing.T) {
	const schemasDir = "./../../../candi/openapi/dhctl"

	newStore := newSchemaStore([]string{schemasDir})

	require.NotEmpty(t, newStore.Get(&SchemaIndex{
		Kind:    "SSHConfig",
		Version: "dhctl.deckhouse.io/v1",
	}))
	require.NotEmpty(t, newStore.Get(&SchemaIndex{
		Kind:    "SSHHost",
		Version: "dhctl.deckhouse.io/v1",
	}))
}

func TestParseConnectionConfig(t *testing.T) {
	const schemasDir = "./../../../candi/openapi/dhctl"
	newStore := newSchemaStore([]string{schemasDir})

	configFunc := func(config, keyPath1, keyPath2 string) string {
		return fmt.Sprintf(
			config,
			strings.Join(strings.Split(readFile(t, keyPath1), "\n"), "\n    "),
			strings.Join(strings.Split(readFile(t, keyPath2), "\n"), "\n    "),
		)
	}

	tests := map[string]struct {
		config      string
		expected    *ConnectionConfig
		opts        []ValidateOption
		errContains string
	}{
		"valid config": {
			config: configFunc(
				validSSHConfig,
				"./mocks/id_rsa",
				"./mocks/id_passphrase_rsa",
			),
			expected: &ConnectionConfig{
				SSHConfig: &SSHConfig{
					SSHUser:      "ubuntu",
					SSHPort:      pointer.Int32(22),
					SSHExtraArgs: "-vvv",
					SSHAgentPrivateKeys: []SSHAgentPrivateKey{
						{
							Key:        readFile(t, "./mocks/id_rsa"),
							Passphrase: "",
						},
						{
							Key:        readFile(t, "./mocks/id_passphrase_rsa"),
							Passphrase: "test",
						},
					},
				},
				SSHHosts: []SSHHost{
					{
						Host: "158.160.112.65",
					},
					{
						Host: "static.host.test",
					},
				},
			},
			opts: []ValidateOption{ValidateOptionValidateExtensions(true)},
		},
		"invalid config: bad ssh key": {
			config: configFunc(
				validSSHConfig,
				"./mocks/id_rsa",
				"./mocks/id_invalid_rsa",
			),
			errContains: "not an encrypted key",
			opts:        []ValidateOption{ValidateOptionValidateExtensions(true)},
		},
		"invalid config: no user": {
			config: configFunc(
				invalidSSHConfigNoUser,
				"./mocks/id_rsa",
				"./mocks/id_passphrase_rsa",
			),
			errContains: "sshUser is required",
		},
		"invalid config: no agent private keys": {
			config:      invalidSSHConfigNoKeys,
			errContains: "sshAgentPrivateKeys is required",
		},
		"invalid config: empty host": {
			config: configFunc(
				invalidSSHConfigNoHosts,
				"./mocks/id_rsa",
				"./mocks/id_passphrase_rsa",
			),
			errContains: "host is required",
		},
		"invalid config: duplicated field": {
			config: configFunc(
				validSSHConfig,
				"./mocks/id_rsa",
				"./mocks/id_passphrase_rsa",
			),
			opts:        []ValidateOption{ValidateOptionStrictUnmarshal(true)},
			errContains: "already set in map",
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			config, err := ParseConnectionConfig(tt.config, newStore, tt.opts...)
			if tt.errContains == "" {
				require.NoError(t, err)
				require.Equal(t, tt.expected, config)
			} else {
				require.ErrorContains(t, err, tt.errContains)
				require.Nil(t, config)
			}
		})
	}
}

var validSSHConfig = `
---
apiVersion: dhctl.deckhouse.io/v1
kind: SSHConfig
sshUser: ubuntu
sshPort: 21 # without strict unmarshalling will be overwritten with value below
sshPort: 22
sshExtraArgs: -vvv
sshAgentPrivateKeys:
- key: |
    %s
- key: |
    %s
  passphrase: test
---
apiVersion: dhctl.deckhouse.io/v1
kind: SSHHost
host: 158.160.112.65
---
apiVersion: dhctl.deckhouse.io/v1
kind: SSHHost
host: static.host.test
`

var invalidSSHConfigNoUser = `
apiVersion: dhctl.deckhouse.io/v1
kind: SSHConfig
sshPort: 22
sshAgentPrivateKeys:
- key: |
    %s
- key: |
    %s
  passphrase: test
---
apiVersion: dhctl.deckhouse.io/v1
kind: SSHHost
host: 158.160.112.65
`

var invalidSSHConfigNoKeys = `
apiVersion: dhctl.deckhouse.io/v1
kind: SSHConfig
sshUser: ubuntu
sshPort: 22
---
apiVersion: dhctl.deckhouse.io/v1
kind: SSHHost
host: 158.160.112.65
`

var invalidSSHConfigNoHosts = `
apiVersion: dhctl.deckhouse.io/v1
kind: SSHConfig
sshUser: ubuntu
sshPort: 22
sshAgentPrivateKeys:
- key: |
    %s
- key: |
    %s
  passphrase: spicyburrito
apiVersion: dhctl.deckhouse.io/v1
kind: SSHHost
`
