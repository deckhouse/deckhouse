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
	"regexp"
	"strings"

	libcon "github.com/deckhouse/lib-connection/pkg"

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

	reported := strings.TrimSpace(c.DevicePath())
	if reported == "" {
		// Most layouts keep Kubernetes data on the root disk, and then there is nothing to mount.
		return "", preflight.NotApplicable("this cluster has no separate disk for Kubernetes data")
	}

	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	path := resolveDataDevice(ctx, nodeInterface, reported)
	if path == "" {
		// The disk is attached by the provider; it does not appear between two attempts.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the Kubernetes data disk %q on %s", reported, host),
			Observed: "no block device on the node matches it",
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

// resolveDataDevice turns what the provider reported into a device path on the node.
//
// It is not always a path. Azure reports the LUN of the attachment (data_disk_attachment.lun, so
// "10"), and GCP reports the disk's device_name, which arrives as the serial of the block device;
// running `test -b` on either of those fails on every cluster, attached disk or not — which is why
// this check used to be red on both providers whatever the state of the machine.
//
// The three shapes and the order they are tried in mirror the bashible steps that mount the disk
// (modules/030-cloud-provider-azure/.../001_discover_kubernetes_data_device_path.sh.tpl and the
// GCP 000_ step beside it), so the check and the mount agree about which device is meant. Like
// those steps, it branches on the shape of the value rather than on the provider name.
//
// It returns the path it resolved to, or "" when nothing on the node matches. There is no
// "unrecognised shape" answer: a serial is any string, so the last branch accepts whatever the
// first two did not — which is also what the GCP step does with it.
func resolveDataDevice(ctx context.Context, nodeInterface libcon.Interface, reported string) string {
	switch {
	case strings.HasPrefix(reported, "/dev/"):
		// A direct path, which is what most providers report. -b follows symlinks: they hand out
		// /dev/disk/by-id/… and the kernel resolves that to /dev/sdc.
		if nodeInterface.Command("test", "-b", reported).Run(ctx) == nil {
			return reported
		}
		return ""

	case azureLUN.MatchString(reported):
		lun := azureLUN.FindStringSubmatch(reported)[1]
		// The three lookups the Azure step does, in its order: the current udev rules, the legacy
		// SCSI paths, then the NVMe namespaces, which Azure numbers LUN+2, LUN+1 or LUN.
		script := fmt.Sprintf(
			"ls -1 /dev/disk/azure/data/by-lun/%[1]s 2>/dev/null | head -n1; "+
				"ls -1 /dev/disk/azure/data-lun%[1]s /dev/disk/azure/scsi*/lun%[1]s 2>/dev/null | head -n1; "+
				"for o in 2 1 0; do ls -1 /dev/disk/by-path/*nvme-$((%[1]s+o)) 2>/dev/null | head -n1; done",
			lun)
		return firstBlockDevice(ctx, nodeInterface, script)

	default:
		// A disk serial, which is how GCP's device_name reaches the node. Matched the way the GCP
		// step matches it, on the line rather than on the field, so the two agree.
		script := fmt.Sprintf("lsblk -lo name,serial | grep -F -- %s | head -n1 | cut -d' ' -f1", shellQuote(reported))
		name := commandOutput(ctx, nodeInterface, "sh", "-c", script)
		if name == "" {
			return ""
		}
		device := "/dev/" + name
		if nodeInterface.Command("test", "-b", device).Run(ctx) != nil {
			return ""
		}
		return device
	}
}

// azureLUN matches the LUN Azure reports, written either as the bare number or with the prefix the
// bashible step also accepts.
var azureLUN = regexp.MustCompile(`^(?:lun)?([0-9]+)$`)

// firstBlockDevice runs the candidate lookups and returns the first line that names a real block
// device, so a stale symlink left in /dev/disk does not pass for an attached disk.
func firstBlockDevice(ctx context.Context, nodeInterface libcon.Interface, script string) string {
	for _, candidate := range strings.Split(commandOutput(ctx, nodeInterface, "sh", "-c", script), "\n") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if nodeInterface.Command("test", "-b", candidate).Run(ctx) == nil {
			return candidate
		}
	}
	return ""
}

// shellQuote wraps a value in single quotes for the one place a check builds a shell line.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
