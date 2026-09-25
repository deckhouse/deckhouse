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
	"context"
	"fmt"
	"strconv"
	"strings"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// nodeStatePath is where everything Deckhouse installs and everything etcd writes ends up, so it
// is the filesystem whose size decides whether a bootstrap can finish.
const nodeStatePath = "/var/lib"

// nodeFilesystemFloorGB is how big the filesystem holding /var/lib has to be.
//
// It is not minimumRequiredRootDiskSizeGB, and the difference is the point. That constant is the
// disk a cloud master is asked for, and it is checked against the configuration, where 50 means
// 50. This one is checked against what df reports, and a 50 GB disk never presents 50 GB of
// filesystem: the partition table, ext4's inode tables and journal take on the order of two
// percent, and many images carve a boot partition off the front as well. Comparing the formatted
// size against the unformatted requirement failed a node that had exactly the disk the
// documentation asks for.
//
// 45 leaves room for both and still refuses a node given 40 GB, which is the mistake worth
// catching.
const nodeFilesystemFloorGB = 45

// minimumFreeDiskGiB is how much of the filesystem has to be free for the bootstrap to finish:
// unpacking packages, pulling images and writing the first etcd state all happen before anything
// reclaims space.
const minimumFreeDiskGiB = 20

// NodeDiskSpaceCheck measures how big the filesystem the node will be built on is.
//
// Nothing measured it: minimumRequiredRootDiskSizeGB is checked against the *configuration* of a
// cloud master and against no actual machine at all. How much of it is free is a separate
// question, asked by StaticFreeDiskSpaceCheck of a static node only.
type NodeDiskSpaceCheck struct {
	NodeInterface NodeInterfaceFunc
}

const NodeDiskSpaceCheckName preflight.CheckName = "node-disk-space"

func (NodeDiskSpaceCheck) Description() string {
	return "the node has the disk Deckhouse needs"
}

func (NodeDiskSpaceCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (NodeDiskSpaceCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c NodeDiskSpaceCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	// -P is the portable output format, so the columns are the same on every distribution;
	// -k makes them 1K blocks rather than whatever the node's default is.
	stdout, _, err := nodeInterface.Command("df", "-Pk", nodeStatePath).Output(ctx)
	if err != nil {
		return "", scriptFailure("measure the disk", nodeInterface, nil, err)
	}

	totalKB, _, ok := parseDfOutput(string(stdout))
	if !ok {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("`df -Pk %s` on %s", nodeStatePath, host),
			Observed: fmt.Sprintf("the output of df could not be read: %q", strings.TrimSpace(string(stdout))),
			Expected: "the size of the filesystem holding " + nodeStatePath,
			Fix:      "check that df is installed on the node",
		})
	}

	// df -Pk counts 1024-byte blocks, and GB here is decimal, the unit a disk is sold and
	// configured in. Dividing the blocks by a million treated a KiB as a kB and reported every
	// filesystem 2.3% smaller than it is.
	totalGB := totalKB * 1024 / 1_000_000_000

	if totalGB < nodeFilesystemFloorGB {
		// A disk does not grow between two attempts of the same check.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the filesystem holding %s on %s", nodeStatePath, host),
			Observed: fmt.Sprintf("the filesystem is %d GB", totalGB),
			Expected: fmt.Sprintf("at least %d GB of filesystem at %s (what a %d GB disk holds after "+
				"partitioning and formatting)", nodeFilesystemFloorGB, nodeStatePath, minimumRequiredRootDiskSizeGB),
			Fix: "give the node a larger disk, or mount a larger filesystem at " + nodeStatePath,
		})
	}

	return fmt.Sprintf("%s has %d GB at %s", host, totalGB, nodeStatePath), nil
}

// StaticFreeDiskSpaceCheck asks how much of that filesystem is actually free.
//
// Only of a static node. A cloud master is created from a disk dhctl asks the provider for, and
// arrives empty; a static node is a machine that has been doing something else, and the space
// left on it is the operator's business until the bootstrap runs out of it. That failure surfaces
// as ENOSPC somewhere inside bashible — in tar, in the package manager, in containerd or in etcd
// — and lands in the retry storm.
type StaticFreeDiskSpaceCheck struct {
	NodeInterface NodeInterfaceFunc
}

const StaticFreeDiskSpaceCheckName preflight.CheckName = "static-free-disk-space"

func (StaticFreeDiskSpaceCheck) Description() string {
	return "the node has enough free disk for the bootstrap"
}

func (StaticFreeDiskSpaceCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (StaticFreeDiskSpaceCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c StaticFreeDiskSpaceCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	stdout, _, err := nodeInterface.Command("df", "-Pk", nodeStatePath).Output(ctx)
	if err != nil {
		return "", scriptFailure("measure the free disk space", nodeInterface, nil, err)
	}

	_, freeKB, ok := parseDfOutput(string(stdout))
	if !ok {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("`df -Pk %s` on %s", nodeStatePath, host),
			Observed: fmt.Sprintf("the output of df could not be read: %q", strings.TrimSpace(string(stdout))),
			Expected: "the free space on the filesystem holding " + nodeStatePath,
			Fix:      "check that df is installed on the node",
		})
	}

	freeGiB := freeKB / (1024 * 1024)
	if freeGiB < minimumFreeDiskGiB {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the free space at %s on %s", nodeStatePath, host),
			Observed: fmt.Sprintf("the filesystem has %d GiB free", freeGiB),
			Expected: fmt.Sprintf("at least %d GiB free at %s", minimumFreeDiskGiB, nodeStatePath),
			Fix:      "free space on the node, or mount a larger filesystem at " + nodeStatePath,
		})
	}

	return fmt.Sprintf("%s has %d GiB free at %s", host, freeGiB, nodeStatePath), nil
}

func StaticFreeDiskSpace(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := StaticFreeDiskSpaceCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        StaticFreeDiskSpaceCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}

// parseDfOutput reads the total and available 1K blocks out of `df -Pk`. The POSIX format puts
// them in the second and fourth columns of the last line, which is the one a long device name
// would otherwise have wrapped.
func parseDfOutput(output string) (int, int, bool) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return 0, 0, false
	}

	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return 0, 0, false
	}

	total, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, false
	}
	free, err := strconv.Atoi(fields[3])
	if err != nil {
		return 0, 0, false
	}
	return total, free, true
}

func NodeDiskSpace(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := NodeDiskSpaceCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        NodeDiskSpaceCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
