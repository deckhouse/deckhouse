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
//
// So the question is not "does the reported path exist" but "will step 005 find one disk". A path
// that does not exist is normal on more providers than not: AWS reports the attachment name
// (/dev/xvdf) while a Nitro instance presents the disk as /dev/nvme1n1, and the check reported
// that healthy machine as having no data disk at all. This check answers the same way step 005
// will, by falling back the same way.
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
	autodetected := false

	if path == "" {
		// What step 005 does next, and the only answer that matters: one unused disk is what it
		// needs, and it takes it whatever the provider called it.
		candidates := unusedDisks(ctx, nodeInterface)
		if len(candidates) != 1 {
			// The disk is attached by the provider; that does not change between two attempts.
			return "", preflight.Permanent(&preflight.Failure{
				Checked:  fmt.Sprintf("the Kubernetes data disk %q on %s", reported, host),
				Observed: unusedDiskProblem(candidates),
				Expected: "one disk for Kubernetes data: the one the provider reported, or a single unused one to fall back to",
				Fix: "check that the master node group requests a disk for Kubernetes data, and that the cloud " +
					"attached it. If the node has several spare disks, remove the ones Deckhouse must not take",
			})
		}
		path, autodetected = candidates[0], true
	}

	resolved := commandOutput(ctx, nodeInterface, "readlink", "-f", path)
	if resolved == "" {
		resolved = path
	}

	// Already carrying a filesystem is not a failure — a resumed bootstrap finds its own — but it
	// is worth saying, because a disk with someone else's data on it looks exactly the same.
	if fsType := commandOutput(ctx, nodeInterface, "lsblk", "-no", "FSTYPE", resolved); fsType != "" {
		return fmt.Sprintf("%s is attached to %s and already carries a filesystem (%s)%s",
			resolved, host, fsType, autodetectedNote(autodetected, reported)), nil
	}

	return fmt.Sprintf("%s is attached to %s and is empty%s",
		resolved, host, autodetectedNote(autodetected, reported)), nil
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

// autodetectedNote says the device was not the one the provider named, which is worth seeing in a
// passing line: it is normal on AWS and it is also what a misconfigured attachment looks like.
func autodetectedNote(autodetected bool, reported string) string {
	if !autodetected {
		return ""
	}
	return fmt.Sprintf(" (%q does not exist on the node, so bashible will autodetect this one)", reported)
}

// unusedDiskProblem states which half of "exactly one" failed, because the two need different
// things done about them.
func unusedDiskProblem(candidates []string) string {
	if len(candidates) == 0 {
		return "no block device matches it, and the node has no unused disk to fall back to"
	}
	return fmt.Sprintf("no block device matches it, and the node has %d unused disks, so bashible cannot choose: %s",
		len(candidates), strings.Join(candidates, ", "))
}

// unusedDisks lists the whole disks that carry nothing — no partitions, no filesystem, not
// mounted — which is what step 005 falls back to.
//
// The same rule as that step, read out of lsblk's key=value output rather than its JSON: jq is
// what bashible uses, and bashible has installed it by then. This runs before any of that, on
// whatever the image shipped.
func unusedDisks(ctx context.Context, nodeInterface libcon.Interface) []string {
	output := commandOutput(ctx, nodeInterface, "lsblk", "-Pno", "PATH,TYPE,MOUNTPOINT,FSTYPE")

	type device struct{ path, kind, mountpoint, fstype string }

	var devices []device
	for _, line := range strings.Split(output, "\n") {
		fields := lsblkPairs(line)
		if fields["PATH"] == "" {
			continue
		}
		devices = append(devices, device{fields["PATH"], fields["TYPE"], fields["MOUNTPOINT"], fields["FSTYPE"]})
	}

	var unused []string
	for _, candidate := range devices {
		if candidate.kind != "disk" || candidate.mountpoint != "" || candidate.fstype != "" {
			continue
		}
		// zram is memory, and lsblk reports it as a disk like any other.
		if strings.Contains(candidate.path, "zram") {
			continue
		}
		// A disk with partitions is in use even when the disk itself carries no filesystem.
		partitioned := false
		for _, other := range devices {
			if other.path != candidate.path && strings.HasPrefix(other.path, candidate.path) {
				partitioned = true
				break
			}
		}
		if !partitioned {
			unused = append(unused, candidate.path)
		}
	}
	return unused
}

// lsblkPairs reads one KEY="value" line of `lsblk -P`. The pairs form is used rather than the raw
// one because an empty MOUNTPOINT or FSTYPE is exactly what is being looked for, and raw output
// collapses empty columns into nothing.
func lsblkPairs(line string) map[string]string {
	pairs := map[string]string{}
	for _, field := range strings.Fields(strings.TrimSpace(line)) {
		key, value, found := strings.Cut(field, "=")
		if !found {
			continue
		}
		pairs[key] = strings.Trim(value, `"`)
	}
	return pairs
}
