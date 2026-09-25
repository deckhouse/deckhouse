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

package checks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// containerdV2MetaConfig is a cluster that asks for the runtime with requirements.
func containerdV2MetaConfig(t *testing.T) *config.MetaConfig {
	t.Helper()
	encoded, err := json.Marshal(containerdV2CRI)
	require.NoError(t, err)
	return &config.MetaConfig{ClusterConfig: map[string]json.RawMessage{"defaultCRI": encoded}}
}

// supportedNode answers the four questions the way a node that can run containerd v2 does.
func supportedNode() *fakeNode {
	return newFakeNode().
		on("uname -r").prints("6.1.0-13-amd64").
		on("systemctl --version").prints("systemd 252 (252.22-1~deb12u1)\n+PAM +AUDIT").
		on("stat -fc %T /sys/fs/cgroup").prints("cgroup2fs").
		on("cat /proc/filesystems").prints("nodev\tsysfs\n\terofs\n\text4")
}

// TestNodeCRIRequirementsRun covers what bashible refuses at step 000, from inside the retry
// storm, on a node that cannot be fixed without a reboot — and on most distributions, without a
// different kernel.
func TestNodeCRIRequirementsRun(t *testing.T) {
	tests := []struct {
		name    string
		node    *fakeNode
		wantErr string
	}{
		{
			name: "a node that supports it",
			node: supportedNode(),
		},
		{
			name:    "a kernel below the floor",
			node:    supportedNode().on("uname -r").prints("4.18.0-513.el8.x86_64"),
			wantErr: "kernel 4.18.0-513.el8.x86_64 is older than 5.8",
		},
		{
			name:    "systemd below the floor",
			node:    supportedNode().on("systemctl --version").prints("systemd 219 (219)"),
			wantErr: "systemd 219 is older than 244",
		},
		{
			name:    "the node still boots on cgroup v1",
			node:    supportedNode().on("stat -fc %T /sys/fs/cgroup").prints("tmpfs"),
			wantErr: `cgroup v2 is not in use (/sys/fs/cgroup is "tmpfs")`,
		},
		{
			// Neither built in nor loadable: modprobe -n is the second chance, and the default
			// answer of the fake is "not there".
			name:    "no erofs",
			node:    supportedNode().on("cat /proc/filesystems").prints("nodev\tsysfs\n\text4"),
			wantErr: "erofs kernel module is not available",
		},
		{
			// The two ranges the bashible check names: containerd v2 stores images on EROFS.
			name:    "a kernel inside the CVE range",
			node:    supportedNode().on("uname -r").prints("6.12.20-generic"),
			wantErr: "CVE-2025-37999",
		},
		{
			// Every unmet requirement at once, because they are found together and a node is
			// upgraded once.
			name: "several at once",
			node: supportedNode().
				on("uname -r").prints("4.18.0-513.el8.x86_64").
				on("systemctl --version").prints("systemd 219 (219)"),
			wantErr: "systemd 219",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := NodeCRIRequirementsCheck{
				MetaConfig:    containerdV2MetaConfig(t),
				NodeInterface: FixedNodeInterface(tt.node),
			}

			detail, err := check.Run(t.Context())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, detail, "supports ContainerdV2")
		})
	}
}

// TestNodeCRIRequirementsAcceptsALoadableErofs: a module that is not built in but can be loaded
// is just as good, and refusing such a node would be wrong on every distribution that ships erofs
// as a module.
func TestNodeCRIRequirementsAcceptsALoadableErofs(t *testing.T) {
	node := supportedNode().
		on("cat /proc/filesystems").prints("nodev\tsysfs\n\text4").
		on("modprobe -n erofs").succeeds()

	check := NodeCRIRequirementsCheck{
		MetaConfig:    containerdV2MetaConfig(t),
		NodeInterface: FixedNodeInterface(node),
	}

	detail, err := check.Run(t.Context())
	require.NoError(t, err)
	assert.Contains(t, detail, "erofs")
}
