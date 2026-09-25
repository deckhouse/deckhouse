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
// This is the floor for the shape where etcd has nowhere else to go, which is a static cluster and
// the few clouds that attach no separate disk; see nodeFilesystemWithSeparateEtcdFloorGB for the
// other one.
//
// It is deliberately below the documented disk rather than level with it. It started at 45 — 50
// minus formatting — and refused masters our own platforms hand out: a DVP master gets a 40 GiB
// root, which measures about 42 GB. Those clusters work, and nearly every e2e run reaches dhctl
// through Commander, where there is no command line to put a skip flag on — so a floor that
// refuses them refuses the product, with no way around it. 40 keeps a little of that margin while
// still catching the mistake this check exists for, a node given 30 GB or less.
const nodeFilesystemFloorGB = 40

// nodeFilesystemWithSeparateEtcdFloorGB is the floor when the cluster has a disk of its own for
// Kubernetes data, which is nearly every cloud provider.
//
// The documented 50 GB is what the master machine needs; this check measures one filesystem. When
// etcd lives on the disk mounted at /mnt/kubernetes-data — which cloud-kube-data-device is the
// check for — the root filesystem holds packages, container images and containerd's state, and
// none of etcd. Measuring that against a floor derived from "50 GB, etcd included" asks for
// something the documentation never asked for on this shape: an AWS master is created on the AMI's
// own 20 GiB root (diskSizeGb has no default in the schema) with a separate 20 GiB etcd disk, and
// was refused at 19 GB while working perfectly.
//
// 15 clears the smallest root our own platforms hand a master and still refuses a machine that
// cannot hold the control-plane images.
const nodeFilesystemWithSeparateEtcdFloorGB = 15

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
	// KubeDataDevicePath reports the disk the provider attached for Kubernetes data, read when
	// the check runs because it is an output of the infrastructure. A non-empty value means etcd
	// will not be on the filesystem being measured, and the floor moves accordingly. nil on a
	// static cluster, where there is no such disk.
	KubeDataDevicePath func() string
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

	floor, separateEtcd := c.floor()
	if totalGB < floor {
		// A disk does not grow between two attempts of the same check.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the filesystem holding %s on %s", nodeStatePath, host),
			Observed: fmt.Sprintf("the filesystem is %d GB", totalGB),
			Expected: c.expectation(floor, separateEtcd),
			Fix:      "give the node a larger disk, or mount a larger filesystem at " + nodeStatePath,
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

func NodeDiskSpace(nodeInterface NodeInterfaceFunc, kubeDataDevicePath func() string) preflight.Check {
	check := NodeDiskSpaceCheck{NodeInterface: nodeInterface, KubeDataDevicePath: kubeDataDevicePath}
	return preflight.Check{
		Name:        NodeDiskSpaceCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}

// floor is how big the filesystem has to be, and whether etcd was taken out of the reckoning.
//
// etcd is on a disk of its own on nearly every cloud provider, and that disk is exactly what
// cloud-kube-data-device asks about: a value there means /var/lib will not hold the cluster state.
func (c NodeDiskSpaceCheck) floor() (int, bool) {
	if c.KubeDataDevicePath == nil || strings.TrimSpace(c.KubeDataDevicePath()) == "" {
		return nodeFilesystemFloorGB, false
	}
	return nodeFilesystemWithSeparateEtcdFloorGB, true
}

// expectation says what would have passed, and why the number is what it is — without which a
// reader who knows the documentation asks for 50 GB has no way to make sense of a 15 GB floor.
func (c NodeDiskSpaceCheck) expectation(floor int, separateEtcd bool) string {
	if separateEtcd {
		return fmt.Sprintf("at least %d GB of filesystem at %s; the cluster state goes on the separate disk "+
			"for Kubernetes data, so it is not counted here", floor, nodeStatePath)
	}
	return fmt.Sprintf("at least %d GB of filesystem at %s; the documentation asks for a %d GB disk",
		floor, nodeStatePath, minimumRequiredRootDiskSizeGB)
}
