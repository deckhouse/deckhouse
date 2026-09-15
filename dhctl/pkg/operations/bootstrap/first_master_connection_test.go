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

	"github.com/stretchr/testify/assert"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
)

// TestMasterConnectionString: this line is printed for the operator's convenience, and printing it
// used to cost a live SSH connection — the provider starts the client it hands back. A master that
// had not finished booting, or an --ssh-user the image does not have, therefore failed the
// FirstMaster phase after fifty attempts two seconds apart, two minutes before the preflight that
// exists to explain it ran at all.
//
// Nothing in the string needs a connection, so none of these cases may open one.
func TestMasterConnectionString(t *testing.T) {
	port := func(p int) *int { return &p }

	tests := []struct {
		name   string
		config *sshconfig.ConnectionConfig
		want   string
	}{
		{
			name:   "no configuration at all",
			config: nil,
			want:   "ssh 10.0.0.5",
		},
		{
			name: "a user and the default port",
			config: &sshconfig.ConnectionConfig{
				Config: &sshconfig.Config{User: "ubuntu", Port: port(22)},
			},
			want: "ssh ubuntu@10.0.0.5",
		},
		{
			name: "a port that is not the default",
			config: &sshconfig.ConnectionConfig{
				Config: &sshconfig.Config{User: "ubuntu", Port: port(2222)},
			},
			want: "ssh ubuntu@10.0.0.5 -p 2222",
		},
		{
			name: "through a bastion",
			config: &sshconfig.ConnectionConfig{
				Config: &sshconfig.Config{
					User:        "ubuntu",
					Port:        port(22),
					BastionHost: "bastion.example.com",
					BastionUser: "jump",
					BastionPort: port(2200),
				},
			},
			want: "ssh -J jump@bastion.example.com:2200 ubuntu@10.0.0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, masterConnectionString(tt.config, "10.0.0.5"))
		})
	}
}
