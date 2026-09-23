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

// The path a provider reports: a stable symlink, which the kernel resolves to a device name.
const (
	dataDeviceLink = "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_0bfa67a1"
	dataDeviceReal = "/dev/sdc"
)

// TestCloudKubeDataDevice covers what bashible does at step 005 when the path is not a device: it
// falls back to autodetecting an unused disk, and either finds several or finds none — inside the
// retry storm, on a master that has already been created.
func TestCloudKubeDataDevice(t *testing.T) {
	t.Run("the disk is attached and empty", func(t *testing.T) {
		node := newFakeNode().
			on("test -b " + dataDeviceLink).succeeds().
			on("readlink -f " + dataDeviceLink).prints(dataDeviceReal).
			on("lsblk -no FSTYPE " + dataDeviceReal).prints("")

		check := CloudKubeDataDeviceCheck{
			DevicePath:    func() string { return dataDeviceLink },
			NodeInterface: FixedNodeInterface(node),
		}

		detail, err := check.Run(t.Context())
		require.NoError(t, err)
		assert.Contains(t, detail, dataDeviceReal+" is attached")
		assert.Contains(t, detail, "is empty")
	})

	t.Run("the disk already carries a filesystem", func(t *testing.T) {
		// A resumed bootstrap finds its own, so this is not a failure — but a disk with someone
		// else's data on it looks exactly the same, and is worth saying out loud.
		node := newFakeNode().
			on("test -b " + dataDeviceLink).succeeds().
			on("readlink -f " + dataDeviceLink).prints(dataDeviceReal).
			on("lsblk -no FSTYPE " + dataDeviceReal).prints("ext4")

		check := CloudKubeDataDeviceCheck{
			DevicePath:    func() string { return dataDeviceLink },
			NodeInterface: FixedNodeInterface(node),
		}

		detail, err := check.Run(t.Context())
		require.NoError(t, err)
		assert.Contains(t, detail, "already carries a filesystem (ext4)")
	})

	t.Run("the path is not a block device", func(t *testing.T) {
		// The default answer of the fake is a non-zero exit, which is what `test -b` does for a
		// path that is not there.
		check := CloudKubeDataDeviceCheck{
			DevicePath:    func() string { return dataDeviceLink },
			NodeInterface: FixedNodeInterface(newFakeNode()),
		}

		_, err := check.Run(t.Context())

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "not a block device on the node")
		assert.Contains(t, failure.Fix, "the disk is attached to the master node")
	})

	t.Run("no separate disk in this layout", func(t *testing.T) {
		check := CloudKubeDataDeviceCheck{
			DevicePath:    func() string { return "" },
			NodeInterface: FixedNodeInterface(newFakeNode()),
		}

		_, err := check.Run(t.Context())
		assert.ErrorIs(t, err, preflight.ErrNotApplicable)
	})

	t.Run("nothing reports a path at all", func(t *testing.T) {
		check := CloudKubeDataDeviceCheck{NodeInterface: FixedNodeInterface(newFakeNode())}

		_, err := check.Run(t.Context())
		assert.ErrorIs(t, err, preflight.ErrNotApplicable)
	})
}
