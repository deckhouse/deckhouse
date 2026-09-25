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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// dfOutput builds what `df -Pk` prints for a filesystem of the given size.
//
// The two units are deliberate and are the ones the checks use: a disk is sold and reported in
// decimal GB, and free space is compared in GiB, which is what an operator sees in `df -h`.
// dfOutput renders what `df -Pk` prints for a filesystem of totalGB decimal gigabytes with
// freeGiB gibibytes free.
//
// The conversion is spelled out because getting it wrong is the bug this file exists to catch:
// df counts 1024-byte blocks, so a decimal gigabyte is 1e9/1024 of them, not a million. The
// fixture used to make the same mistake as the code it was testing, which is why a filesystem
// reported 2.3% smaller than it is went unnoticed until a node with exactly the documented disk
// was refused.
func dfOutput(totalGB, freeGiB int) string {
	totalKB := totalGB * 1_000_000_000 / 1024
	freeKB := freeGiB * 1024 * 1024
	return "Filesystem     1024-blocks     Used Available Capacity Mounted on\n" +
		fmt.Sprintf("/dev/vda1 %d %d %d 22%% /\n", totalKB, totalKB-freeKB, freeKB)
}

func TestNodeDiskSpace(t *testing.T) {
	tests := []struct {
		name       string
		node       *fakeNode
		wantDetail string
		wantErr    string
	}{
		{
			name:       "a disk above the floor",
			node:       newFakeNode().on("df -Pk /var/lib").prints(dfOutput(100, 60)),
			wantDetail: "has 100 GB at /var/lib",
		},
		{
			// The case a real bootstrap was refused on: a 50 GiB disk, whose root filesystem
			// measures about 50 GB once it is partitioned and formatted. It has the disk the
			// documentation asks for, and the check said it was 49 GB and stopped the install.
			name:       "the disk the documentation asks for",
			node:       newFakeNode().on("df -Pk /var/lib").prints(dfOutput(50, 40)),
			wantDetail: "has 50 GB at /var/lib",
		},
		{
			// Formatting takes a couple of percent and many images carve off a boot partition,
			// so the floor sits below the disk requirement rather than on it.
			name:       "a disk of the right size with a boot partition taken off it",
			node:       newFakeNode().on("df -Pk /var/lib").prints(dfOutput(46, 30)),
			wantDetail: "has 46 GB at /var/lib",
		},
		{
			// The smallest a master is given on our own platforms: a DVP root disk of 40 GiB,
			// which measures about 42 GB. The floor used to be 45 and refused it, and the
			// Commander path it bootstraps through has no command line to skip the check on.
			name:       "the root disk a DVP master is created with",
			node:       newFakeNode().on("df -Pk /var/lib").prints(dfOutput(42, 30)),
			wantDetail: "has 42 GB at /var/lib",
		},
		{
			name:    "a disk below the floor",
			node:    newFakeNode().on("df -Pk /var/lib").prints(dfOutput(30, 25)),
			wantErr: "the filesystem is 30 GB",
		},
		{
			name:    "df is not there",
			node:    newFakeNode().on("df -Pk /var/lib").fails(errors.New("bash: df: command not found")),
			wantErr: "cannot measure the disk on",
		},
		{
			name:    "df printed something else",
			node:    newFakeNode().on("df -Pk /var/lib").prints("df: /var/lib: No such file or directory"),
			wantErr: "the output of df could not be read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := NodeDiskSpaceCheck{NodeInterface: FixedNodeInterface(tt.node)}
			detail, err := check.Run(t.Context())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, detail, tt.wantDetail)
		})
	}
}

// TestStaticFreeDiskSpace is the static-only half: a cloud master arrives on a disk dhctl asked
// the provider for and it is empty, a static node is a machine that has been doing something else.
func TestStaticFreeDiskSpace(t *testing.T) {
	tests := []struct {
		name    string
		node    *fakeNode
		wantErr string
	}{
		{
			name: "enough free",
			node: newFakeNode().on("df -Pk /var/lib").prints(dfOutput(100, 40)),
		},
		{
			name: "exactly at the floor",
			node: newFakeNode().on("df -Pk /var/lib").prints(dfOutput(100, 20)),
		},
		{
			// ENOSPC inside bashible — in tar, in the package manager, in containerd or in etcd —
			// lands in the retry storm, which repeats it until the bundle is killed.
			name:    "a nearly full disk",
			node:    newFakeNode().on("df -Pk /var/lib").prints(dfOutput(100, 5)),
			wantErr: "the filesystem has 5 GiB free",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := StaticFreeDiskSpaceCheck{NodeInterface: FixedNodeInterface(tt.node)}
			detail, err := check.Run(t.Context())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)

				var failure *preflight.Failure
				require.ErrorAs(t, err, &failure)
				assert.Contains(t, failure.Expected, "at least 20 GiB")
				return
			}
			require.NoError(t, err)
			assert.Contains(t, detail, "free at /var/lib")
		})
	}
}

// etcd sits on a disk of its own on nearly every cloud provider, and that disk is what
// cloud-kube-data-device asks about. The floor was derived from "50 GB, etcd included", so it
// refused masters where etcd was never going to be on this filesystem: an AWS master is created on
// the AMI's own 20 GiB root — diskSizeGb has no default in the schema — with a separate 20 GiB
// etcd disk, and was refused at 19 GB while working. Bought live on ec2-user@3.65.71.55.
func TestNodeDiskSpaceWithASeparateEtcdDisk(t *testing.T) {
	const awsRoot = 19

	t.Run("the root of an AWS master, etcd elsewhere", func(t *testing.T) {
		check := NodeDiskSpaceCheck{
			NodeInterface:      FixedNodeInterface(newFakeNode().on("df -Pk /var/lib").prints(dfOutput(awsRoot, 12))),
			KubeDataDevicePath: func() string { return "/dev/xvdf" },
		}

		detail, err := check.Run(t.Context())

		// The size itself is not the point and the fixture's KiB round trip shifts it by a
		// gigabyte; that this size passes at all is.
		require.NoError(t, err)
		assert.Contains(t, detail, "GB at /var/lib")
	})

	// The same filesystem without that disk: etcd would land here, and 19 GB is not enough.
	t.Run("the same root with nowhere else for etcd", func(t *testing.T) {
		check := NodeDiskSpaceCheck{
			NodeInterface: FixedNodeInterface(newFakeNode().on("df -Pk /var/lib").prints(dfOutput(awsRoot, 12))),
		}

		_, err := check.Run(t.Context())

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Expected, "the documentation asks for a 50 GB disk")
	})

	// A separate disk is not a licence for any disk at all.
	t.Run("a root too small even for the images", func(t *testing.T) {
		check := NodeDiskSpaceCheck{
			NodeInterface:      FixedNodeInterface(newFakeNode().on("df -Pk /var/lib").prints(dfOutput(8, 4))),
			KubeDataDevicePath: func() string { return "/dev/xvdf" },
		}

		_, err := check.Run(t.Context())

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Expected, "separate disk for Kubernetes data")
	})

	// An empty value is what a layout with no such disk reports, and it must read as "no disk".
	t.Run("the provider reports no data disk", func(t *testing.T) {
		check := NodeDiskSpaceCheck{
			NodeInterface:      FixedNodeInterface(newFakeNode().on("df -Pk /var/lib").prints(dfOutput(awsRoot, 12))),
			KubeDataDevicePath: func() string { return "" },
		}

		_, err := check.Run(t.Context())

		require.Error(t, err)
	})
}
