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
//
// The node is the Debian master this was reported on: CONFIG_EROFS_FS=m, not loaded, and modprobe
// in /sbin, which the SSH user's PATH does not have — so a bare `modprobe -n erofs` is "command not
// found", and the check refused a node containerd v2 installs on. The fake answers it that way.
func TestNodeCRIRequirementsAcceptsALoadableErofs(t *testing.T) {
	node := supportedNode().
		on("cat /proc/filesystems").prints("nodev\tsysfs\n\text4").
		on("modprobe -n erofs").exits(127).
		on("sudo modprobe -n -q erofs").succeeds()

	check := NodeCRIRequirementsCheck{
		MetaConfig:    containerdV2MetaConfig(t),
		NodeInterface: FixedNodeInterface(node),
	}

	detail, err := check.Run(t.Context())
	require.NoError(t, err)
	assert.Contains(t, detail, "erofs")
}

// TestNodeCRIRequirementsAcceptsALoadedErofs: once loaded, the module is in /sys/module and nothing
// has to be asked of modprobe.
func TestNodeCRIRequirementsAcceptsALoadedErofs(t *testing.T) {
	node := supportedNode().
		on("cat /proc/filesystems").prints("nodev\tsysfs\n\text4").
		on("test -d /sys/module/erofs").succeeds()

	_, err := NodeCRIRequirementsCheck{
		MetaConfig:    containerdV2MetaConfig(t),
		NodeInterface: FixedNodeInterface(node),
	}.Run(t.Context())

	require.NoError(t, err)
	assert.NotContains(t, node.ran(), "sudo modprobe -n -q erofs")
}

// TestKernelHasErofsCVE is the table of is_kernel_erofs_cve_vulnerable in
// candi/bashible/common-steps/all/000_check_containerd_v2_support.sh.tpl. The check used to flag
// every 6.12.x and 6.14.x, and refused the ALT Linux master on 6.12.63 that bashible accepts.
func TestKernelHasErofsCVE(t *testing.T) {
	tests := []struct {
		kernel     string
		vulnerable bool
	}{
		{"6.1.0-13-amd64", false},
		{"6.11.9-amd64", false},
		{"6.12.0-1-amd64", true},
		{"6.12.20-generic", true},
		{"6.12.28+bpo-amd64", true},
		{"6.12.29-amd64", false},
		{"6.12.63-6.12-alt1", false},
		{"6.13.2-arch1-1", false},
		{"6.14.0-1-default", true},
		{"6.14.6-300.fc42.x86_64", true},
		{"6.14.7-200.fc42.x86_64", false},
		{"6.15.1", false},

		// Ubuntu flavours keep 6.14.0 and count the fix in the build number.
		{"6.14.0-27-generic", true},
		{"6.14.0-28-generic", false},
		{"6.14.0-1010-aws", true},
		{"6.14.0-1011-aws", false},
		{"6.14.0-1009-azure", true},
		{"6.14.0-1010-azure", false},
		{"6.14.0-1013-gcp", true},
		{"6.14.0-1014-gcp", false},
		{"6.14.0-1011-oracle", false},
		{"6.14.0-1010-oem", false},

		// Oracle Linux UEK8 keeps 6.12.0 and backports the fix.
		{"6.12.0-100.28.2.el9uek.x86_64", true},
		{"6.12.0-101.33.4.3.el9uek.x86_64", false},
		{"6.12.0-102.36.5.2.el10uek.x86_64", false},
		// UEK7 is 5.15-based and never in range.
		{"5.15.0-309.180.4.el9uek.x86_64", false},
	}

	for _, tt := range tests {
		t.Run(tt.kernel, func(t *testing.T) {
			assert.Equal(t, tt.vulnerable, kernelHasErofsCVE(tt.kernel))
		})
	}
}
