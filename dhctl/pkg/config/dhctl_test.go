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
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"
)

func TestLoadDHCTLConfigSchema(t *testing.T) {
	const schemasDir = "./../../../candi/openapi/dhctl"

	t.Run("commander mode: false", func(t *testing.T) {
		newStore := newSchemaStore([]string{schemasDir}, LoadOptions{CommanderMode: false})

		require.Nil(t, newStore.Get(&SchemaIndex{
			Kind:    "SSHConfig",
			Version: "dhctl.deckhouse.io/v1",
		}))
		require.Nil(t, newStore.Get(&SchemaIndex{
			Kind:    "SSHHost",
			Version: "dhctl.deckhouse.io/v1",
		}))
	})

	t.Run("commander mode: true", func(t *testing.T) {
		newStore := newSchemaStore([]string{schemasDir}, LoadOptions{CommanderMode: true})

		require.NotEmpty(t, newStore.Get(&SchemaIndex{
			Kind:    "SSHConfig",
			Version: "dhctl.deckhouse.io/v1",
		}))
		require.NotEmpty(t, newStore.Get(&SchemaIndex{
			Kind:    "SSHHost",
			Version: "dhctl.deckhouse.io/v1",
		}))
	})

}

func TestParseSSHConfig(t *testing.T) {
	const schemasDir = "./../../../candi/openapi/dhctl"
	newStore := newSchemaStore([]string{schemasDir}, LoadOptions{CommanderMode: true})

	tests := map[string]struct {
		config   string
		expected *DHCTLConfig
		wantErr  bool
	}{
		"valid config": {
			config: validSSHConfig,
			expected: &DHCTLConfig{
				SSHConfig: &SSHConfig{
					SSHUser: "ubuntu",
					SSHPort: pointer.Int32(22),
					SSHAgentPrivateKeys: []SSHAgentPrivateKey{
						{
							Key:        "-----BEGIN RSA PRIVATE KEY-----\nsome-key\n-----END RSA PRIVATE KEY-----\n",
							Passphrase: "",
						},
						{
							Key:        "-----BEGIN RSA PRIVATE KEY-----\nsome-key\n-----END RSA PRIVATE KEY-----\n",
							Passphrase: "spicyburrito",
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
			wantErr: false,
		},
		"invalid config: no user": {
			config:  invalidSSHConfigNoUser,
			wantErr: true,
		},
		"invalid config: no agent private keys": {
			config:  invalidSSHConfigNoKeys,
			wantErr: true,
		},
		"invalid config: no hosts": {
			config:  invalidSSHConfigNoHosts,
			wantErr: true,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			config, err := ParseDHCTLConfig(tt.config, newStore, DHCTLConfigOptions{CommanderMode: true})
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, config)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, config)
			}
		})
	}
}

var validSSHConfig = `
apiVersion: dhctl.deckhouse.io/v1
kind: SSHConfig
sshUser: ubuntu
sshPort: 22
sshAgentPrivateKeys:
- key: |
    -----BEGIN RSA PRIVATE KEY-----
    some-key
    -----END RSA PRIVATE KEY-----
- key: |
    -----BEGIN RSA PRIVATE KEY-----
    some-key
    -----END RSA PRIVATE KEY-----
  passphrase: spicyburrito
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
    -----BEGIN RSA PRIVATE KEY-----
    some-key
    -----END RSA PRIVATE KEY-----
- key: |
    -----BEGIN RSA PRIVATE KEY-----
    some-key
    -----END RSA PRIVATE KEY-----
  passphrase: spicyburrito
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
    -----BEGIN RSA PRIVATE KEY-----
    some-key
    -----END RSA PRIVATE KEY-----
- key: |
    -----BEGIN RSA PRIVATE KEY-----
    some-key
    -----END RSA PRIVATE KEY-----
  passphrase: spicyburrito
`
