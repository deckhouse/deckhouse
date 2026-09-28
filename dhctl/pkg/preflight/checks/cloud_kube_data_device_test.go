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
		assert.Contains(t, failure.Observed, "no unused disk to fall back to")
		assert.Contains(t, failure.Fix, "the cloud attached it")
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

// Azure reports the LUN of the attachment and GCP the disk's device_name, which reaches the node
// as a serial. Neither is a path, so `test -b` on it failed on every cluster of those two
// providers, attached disk or not — the check was red whatever the state of the machine. The
// shapes and the lookup order mirror the bashible steps that mount the disk.
func TestCloudKubeDataDeviceResolvesWhatIsNotAPath(t *testing.T) {
	t.Run("an Azure LUN through the current udev rules", func(t *testing.T) {
		node := newFakeNode().
			on("sh -c ls -1 /dev/disk/azure/data/by-lun/10 2>/dev/null | head -n1; ls -1 /dev/disk/azure/data-lun10 /dev/disk/azure/scsi*/lun10 2>/dev/null | head -n1; for o in 2 1 0; do ls -1 /dev/disk/by-path/*nvme-$((10+o)) 2>/dev/null | head -n1; done").
			prints("/dev/disk/azure/scsi1/lun10\n").
			on("test -b /dev/disk/azure/scsi1/lun10").succeeds().
			on("readlink -f /dev/disk/azure/scsi1/lun10").prints("/dev/sdc\n")

		detail, err := CloudKubeDataDeviceCheck{
			DevicePath:    func() string { return "10" },
			NodeInterface: FixedNodeInterface(node),
		}.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "/dev/sdc")
	})

	t.Run("a GCP device name, matched on the serial", func(t *testing.T) {
		node := newFakeNode().
			on("sh -c lsblk -lo name,serial | grep -F -- 'kubernetes-data-0' | head -n1 | cut -d' ' -f1").
			prints("sdb\n").
			on("test -b /dev/sdb").succeeds().
			on("readlink -f /dev/sdb").prints("/dev/sdb\n")

		detail, err := CloudKubeDataDeviceCheck{
			DevicePath:    func() string { return "kubernetes-data-0" },
			NodeInterface: FixedNodeInterface(node),
		}.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "/dev/sdb")
	})

	// Nothing on the node matches: that is the failure the check exists for, and it now names what
	// the provider reported rather than a path nobody wrote.
	t.Run("the disk is not attached", func(t *testing.T) {
		node := newFakeNode().
			on("sh -c lsblk -lo name,serial | grep -F -- 'kubernetes-data-0' | head -n1 | cut -d' ' -f1").prints("")

		_, err := CloudKubeDataDeviceCheck{
			DevicePath:    func() string { return "kubernetes-data-0" },
			NodeInterface: FixedNodeInterface(node),
		}.Run(t.Context())

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Checked, `"kubernetes-data-0"`)
	})
}

// AWS reports the attachment name it asked for, and a Nitro instance presents the disk under a
// name the kernel chose: /dev/xvdf against /dev/nvme1n1. bashible's step 005 handles that by
// falling back to the one unused disk on the machine, so a healthy master was being reported as
// having no data disk. Bought live on ec2-user@3.65.71.55.
func TestCloudKubeDataDeviceFallsBackTheWayBashibleDoes(t *testing.T) {
	const lsblk = "lsblk -Pno PATH,TYPE,MOUNTPOINT,FSTYPE"

	rootDisk := `PATH="/dev/nvme0n1" TYPE="disk" MOUNTPOINT="" FSTYPE=""` + "\n" +
		`PATH="/dev/nvme0n1p1" TYPE="part" MOUNTPOINT="/" FSTYPE="xfs"` + "\n"

	t.Run("the reported name is not the one the kernel chose", func(t *testing.T) {
		node := newFakeNode().
			on("test -b /dev/xvdf").exits(1).
			on(lsblk).prints(rootDisk + `PATH="/dev/nvme1n1" TYPE="disk" MOUNTPOINT="" FSTYPE=""` + "\n").
			on("readlink -f /dev/nvme1n1").prints("/dev/nvme1n1\n")

		detail, err := CloudKubeDataDeviceCheck{
			DevicePath:    func() string { return "/dev/xvdf" },
			NodeInterface: FixedNodeInterface(node),
		}.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "/dev/nvme1n1")
		assert.Contains(t, detail, `"/dev/xvdf" does not exist on the node`,
			"a passing line has to say the device was not the one the provider named")
	})

	// The failure the check exists for: step 005 refuses to choose, minutes later, inside the
	// bashible retry storm.
	t.Run("several unused disks", func(t *testing.T) {
		node := newFakeNode().
			on("test -b /dev/xvdf").exits(1).
			on(lsblk).prints(rootDisk +
			`PATH="/dev/nvme1n1" TYPE="disk" MOUNTPOINT="" FSTYPE=""` + "\n" +
			`PATH="/dev/nvme2n1" TYPE="disk" MOUNTPOINT="" FSTYPE=""` + "\n")

		_, err := CloudKubeDataDeviceCheck{
			DevicePath:    func() string { return "/dev/xvdf" },
			NodeInterface: FixedNodeInterface(node),
		}.Run(t.Context())

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "cannot choose")
		assert.Contains(t, failure.Observed, "/dev/nvme2n1")
	})

	t.Run("nothing to fall back to", func(t *testing.T) {
		node := newFakeNode().
			on("test -b /dev/xvdf").exits(1).
			on(lsblk).prints(rootDisk)

		_, err := CloudKubeDataDeviceCheck{
			DevicePath:    func() string { return "/dev/xvdf" },
			NodeInterface: FixedNodeInterface(node),
		}.Run(t.Context())

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "no unused disk to fall back to")
	})

	// A disk with partitions on it is somebody's, even when the disk itself carries no filesystem.
	t.Run("the only spare disk is partitioned", func(t *testing.T) {
		node := newFakeNode().
			on("test -b /dev/xvdf").exits(1).
			on(lsblk).prints(rootDisk +
			`PATH="/dev/nvme1n1" TYPE="disk" MOUNTPOINT="" FSTYPE=""` + "\n" +
			`PATH="/dev/nvme1n1p1" TYPE="part" MOUNTPOINT="/data" FSTYPE="ext4"` + "\n")

		_, err := CloudKubeDataDeviceCheck{
			DevicePath:    func() string { return "/dev/xvdf" },
			NodeInterface: FixedNodeInterface(node),
		}.Run(t.Context())

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "no unused disk to fall back to")
	})
}
