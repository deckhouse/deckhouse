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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// supportedOS answers as a distribution bashible can configure.
func supportedOS() *fakeNode {
	return newFakeNode().
		on("uname -r").prints("5.15.0-89-generic").
		on("command -v apt-get").prints("/usr/bin/apt-get").
		on("systemctl --version").prints("systemd 249 (249.11-0ubuntu3.12)")
}

// TestNodeOSSupported covers what bashible only warns about at step 56 and then fails on several
// steps later, with nothing connecting the failure to the distribution (issue #3169).
func TestNodeOSSupported(t *testing.T) {
	tests := []struct {
		name    string
		node    *fakeNode
		wantErr string
	}{
		{
			name: "a supported distribution",
			node: supportedOS(),
		},
		{
			name:    "a kernel below the floor",
			node:    supportedOS().on("uname -r").prints("4.18.0-513.el8.x86_64"),
			wantErr: "kernel 4.18.0-513.el8.x86_64, at least 5.8 is required",
		},
		{
			// The default answer of the fake is exit 127 for every `command -v`, so none of the
			// package managers is on PATH.
			name: "no package manager bashible knows",
			node: newFakeNode().
				on("uname -r").prints("5.15.0-89-generic").
				on("systemctl --version").prints("systemd 249 (249.11)"),
			wantErr: "none of apt-get, apt, dnf, yum, rpm, zypper is installed",
		},
		{
			name: "no systemd",
			node: newFakeNode().
				on("uname -r").prints("5.15.0-89-generic").
				on("command -v apt-get").prints("/usr/bin/apt-get"),
			wantErr: "systemd is not running on the node",
		},
		{
			name: "an rpm distribution",
			node: newFakeNode().
				on("uname -r").prints("5.14.0-362.el9.x86_64").
				on("command -v dnf").prints("/usr/bin/dnf").
				on("systemctl --version").prints("systemd 252 (252-14.el9)"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := NodeOSSupportedCheck{NodeInterface: FixedNodeInterface(tt.node)}
			detail, err := check.Run(t.Context())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, detail, "has kernel")
		})
	}
}

// TestNodeXFSFtype covers the filesystem containerd's snapshotter cannot work on, and which
// cannot be fixed in place — the option is set at mkfs time.
func TestNodeXFSFtype(t *testing.T) {
	const mounted = "/dev/vda2 on / type xfs (rw,relatime)\n/dev/vdb1 on /var type xfs (rw,relatime)"

	t.Run("no XFS at all", func(t *testing.T) {
		check := NodeXFSFtypeCheck{NodeInterface: FixedNodeInterface(newFakeNode().
			on("mount -l -t xfs").prints(""))}

		_, err := check.Run(t.Context())
		assert.ErrorIs(t, err, preflight.ErrNotApplicable)
	})

	t.Run("xfs_info is not installed", func(t *testing.T) {
		check := NodeXFSFtypeCheck{NodeInterface: FixedNodeInterface(newFakeNode().
			on("mount -l -t xfs").prints(mounted))}

		_, err := check.Run(t.Context())
		assert.ErrorIs(t, err, preflight.ErrNotApplicable)
	})

	t.Run("every filesystem has d_type", func(t *testing.T) {
		node := newFakeNode().
			on("mount -l -t xfs").prints(mounted).
			on("command -v xfs_info").prints("/usr/sbin/xfs_info").
			on("xfs_info /dev/vda2").prints("naming =version 2 bsize=4096 ascii-ci=0 ftype=1").
			on("xfs_info /dev/vdb1").prints("naming =version 2 bsize=4096 ascii-ci=0 ftype=1")

		check := NodeXFSFtypeCheck{NodeInterface: FixedNodeInterface(node)}
		detail, err := check.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "all XFS filesystems on")
	})

	t.Run("one was formatted without it", func(t *testing.T) {
		node := newFakeNode().
			on("mount -l -t xfs").prints(mounted).
			on("command -v xfs_info").prints("/usr/sbin/xfs_info").
			on("xfs_info /dev/vda2").prints("naming =version 2 bsize=4096 ascii-ci=0 ftype=1").
			on("xfs_info /dev/vdb1").prints("naming =version 2 bsize=4096 ascii-ci=0 ftype=0")

		check := NodeXFSFtypeCheck{NodeInterface: FixedNodeInterface(node)}
		_, err := check.Run(t.Context())

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "/dev/vdb1 formatted with ftype=0")
		assert.Contains(t, failure.Fix, "mkfs.xfs -n ftype=1",
			"the fix is mkfs, which the bashible message never said")
	})
}

// TestNodeResolveHostname covers the classic missing line in /etc/hosts: kubelet and kubeadm both
// look the name up, and node-hostname — which checks its shape — says nothing about it.
func TestNodeResolveHostname(t *testing.T) {
	t.Run("it resolves", func(t *testing.T) {
		node := newFakeNode().
			on("hostname").prints("master-0").
			on("getent hosts master-0").prints("10.0.0.5    master-0")

		check := NodeResolveHostnameCheck{NodeInterface: FixedNodeInterface(node)}
		detail, err := check.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, `resolves its own hostname "master-0" to 10.0.0.5`)
	})

	t.Run("it does not", func(t *testing.T) {
		// getent exits non-zero and prints nothing when the name is unknown.
		node := newFakeNode().on("hostname").prints("master-0")

		check := NodeResolveHostnameCheck{NodeInterface: FixedNodeInterface(node)}
		_, err := check.Run(t.Context())

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, `cannot resolve its own hostname "master-0"`)
		assert.Contains(t, failure.Fix, "/etc/hosts")
	})

	t.Run("there is no hostname", func(t *testing.T) {
		check := NodeResolveHostnameCheck{NodeInterface: FixedNodeInterface(newFakeNode())}
		_, err := check.Run(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "`hostname` printed nothing")
	})
}
