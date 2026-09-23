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
	"strings"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// CloudKubeDataDeviceCheck asks whether the disk the provider says it attached for
// /mnt/kubernetes-data is actually on the master.
//
// bashible mounts it at step 005, and when the path is not a block device it falls back to
// autodetecting an unused disk — which fails with "Could not autodetect a single unused disk,
// candidates: …" when there are several, or with nothing to pick when there are none. Either way
// it happens inside the retry storm, on a master that has already been created, and etcd ends up
// on the root filesystem or nowhere.
type CloudKubeDataDeviceCheck struct {
	// DevicePath is read when the check runs: the provider reports it as an output of the
	// infrastructure it creates, so it does not exist when this suite is built.
	DevicePath    func() string
	NodeInterface NodeInterfaceFunc
}

const CloudKubeDataDeviceCheckName preflight.CheckName = "cloud-kube-data-device"

func (CloudKubeDataDeviceCheck) Description() string {
	return "the disk the provider attached for Kubernetes data is on the master"
}

func (CloudKubeDataDeviceCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (CloudKubeDataDeviceCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c CloudKubeDataDeviceCheck) Run(ctx context.Context) (string, error) {
	if c.DevicePath == nil {
		return "", preflight.NotApplicable("this cluster has no separate disk for Kubernetes data")
	}

	path := strings.TrimSpace(c.DevicePath())
	if path == "" {
		// Most layouts keep Kubernetes data on the root disk, and then there is nothing to mount.
		return "", preflight.NotApplicable("this cluster has no separate disk for Kubernetes data")
	}

	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	// -b follows symlinks, which is what the path usually is: providers report it as
	// /dev/disk/by-id/… and the kernel resolves that to /dev/sdc.
	if nodeInterface.Command("test", "-b", path).Run(ctx) != nil {
		// The disk is attached by the provider; it does not appear between two attempts.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("%s on %s", path, host),
			Observed: "the path is not a block device on the node",
			Expected: "the disk the provider reported as attached for Kubernetes data",
			Fix: "check that the master node group requests a disk for Kubernetes data. " +
				"Check in the cloud that the disk is attached to the master node",
		})
	}

	resolved := commandOutput(ctx, nodeInterface, "readlink", "-f", path)
	if resolved == "" {
		resolved = path
	}

	// Already carrying a filesystem is not a failure — a resumed bootstrap finds its own — but it
	// is worth saying, because a disk with someone else's data on it looks exactly the same.
	if fsType := commandOutput(ctx, nodeInterface, "lsblk", "-no", "FSTYPE", resolved); fsType != "" {
		return fmt.Sprintf("%s is attached to %s and already carries a filesystem (%s)", resolved, host, fsType), nil
	}

	return fmt.Sprintf("%s is attached to %s and is empty", resolved, host), nil
}

func CloudKubeDataDevice(devicePath func() string, nodeInterface NodeInterfaceFunc) preflight.Check {
	check := CloudKubeDataDeviceCheck{DevicePath: devicePath, NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        CloudKubeDataDeviceCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
