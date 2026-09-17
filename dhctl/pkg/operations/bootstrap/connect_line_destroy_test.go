// Copyright 2026 Flant JSC
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

package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
)

func intPtr(i int) *int { return &i }

func TestDestroyCommand(t *testing.T) {
	tests := map[string]struct {
		cfg            *sshconfig.Config
		want           string
		wantNotContain []string
	}{
		"no config at all": {
			cfg:  nil,
			want: "dhctl destroy --ssh-host 10.0.0.5",
		},
		"default ports are left off": {
			cfg: &sshconfig.Config{
				User: "ubuntu",
				Port: intPtr(sshconfig.DefaultPort),
			},
			want: "dhctl destroy --ssh-host 10.0.0.5 --ssh-user ubuntu",
		},
		"non-default port is carried over": {
			cfg:  &sshconfig.Config{User: "ubuntu", Port: intPtr(2222)},
			want: "dhctl destroy --ssh-host 10.0.0.5 --ssh-user ubuntu --ssh-port 2222",
		},
		"bastion is carried over": {
			cfg: &sshconfig.Config{
				User:        "ubuntu",
				BastionHost: "bastion.example.com",
				BastionUser: "jump",
				BastionPort: intPtr(2200),
			},
			want: "dhctl destroy --ssh-host 10.0.0.5 --ssh-user ubuntu " +
				"--ssh-bastion-host bastion.example.com --ssh-bastion-user jump --ssh-bastion-port 2200",
		},
		"key paths are carried over": {
			cfg: &sshconfig.Config{
				User: "ubuntu",
				PrivateKeys: []sshconfig.AgentPrivateKey{
					{Key: "/root/.ssh/id_ed25519", IsPath: true},
					{Key: "/root/.ssh/other", IsPath: true},
				},
			},
			want: "dhctl destroy --ssh-host 10.0.0.5 --ssh-user ubuntu " +
				"--ssh-agent-private-keys /root/.ssh/id_ed25519 --ssh-agent-private-keys /root/.ssh/other",
		},
		// The line reaches the terminal, the debug log and the commander stream alike, and a
		// secret in any of them outlives the cluster it belonged to.
		"secrets are never rendered": {
			cfg: &sshconfig.Config{
				User:            "ubuntu",
				SudoPassword:    "sudo-secret",
				BastionHost:     "bastion.example.com",
				BastionPassword: "bastion-secret",
				PrivateKeys: []sshconfig.AgentPrivateKey{
					{Key: "-----BEGIN OPENSSH PRIVATE KEY-----inline", IsPath: false},
					{Key: "/root/.ssh/id_ed25519", Passphrase: "passphrase-secret", IsPath: true},
				},
			},
			wantNotContain: []string{
				"sudo-secret", "bastion-secret", "passphrase-secret", "BEGIN OPENSSH PRIVATE KEY",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := destroyCommand("10.0.0.5", tt.cfg)

			if tt.want != "" {
				require.Equal(t, tt.want, got)
			}
			for _, s := range tt.wantNotContain {
				require.NotContains(t, got, s)
			}
		})
	}
}

func TestFirstMasterAddressForSSH(t *testing.T) {
	require.Empty(t, firstMasterAddressForSSH(nil), "no context")
	require.Empty(t, firstMasterAddressForSSH(&bootstrapContext{}), "no masters: nothing to clean up over SSH")

	// Sorted by node name, so a multi-master bootstrap names the same one on every run.
	bctx := &bootstrapContext{masterAddressesForSSH: map[string]string{
		"cluster-master-2": "10.0.0.7",
		"cluster-master-0": "10.0.0.5",
		"cluster-master-1": "10.0.0.6",
	}}
	for range 20 {
		require.Equal(t, "10.0.0.5", firstMasterAddressForSSH(bctx))
	}
}
