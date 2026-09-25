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

// NodeXFSFtypeCheck looks for an XFS filesystem formatted without d_type.
//
// containerd's overlayfs snapshotter cannot work on one, and the failure is not obvious: images
// unpack and then behave as though files were missing. bashible refuses such a node at step 005,
// from inside the retry storm, and its message does not say that the fix is mkfs — the filesystem
// has to be recreated, which is why this is worth knowing before anything is installed.
type NodeXFSFtypeCheck struct {
	NodeInterface NodeInterfaceFunc
}

const NodeXFSFtypeCheckName preflight.CheckName = "node-xfs-ftype"

func (NodeXFSFtypeCheck) Description() string {
	return "every XFS filesystem on the node is formatted with ftype=1"
}

func (NodeXFSFtypeCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (NodeXFSFtypeCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c NodeXFSFtypeCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	mounted := commandOutput(ctx, nodeInterface, "mount", "-l", "-t", "xfs")
	devices := xfsDevices(mounted)
	if len(devices) == 0 {
		return "", preflight.NotApplicable("%s has no XFS filesystem mounted", host)
	}

	// xfs_info is in xfsprogs, which a node with XFS mounted normally has; without it the
	// question cannot be asked from here.
	if nodeInterface.Command("command", "-v", "xfs_info").Run(ctx) != nil {
		return "", preflight.NotApplicable("xfs_info is not installed on %s, so ftype cannot be read", host)
	}

	var withoutFtype []string
	for _, device := range devices {
		if strings.Contains(commandOutput(ctx, nodeInterface, "xfs_info", device), "ftype=0") {
			withoutFtype = append(withoutFtype, device)
		}
	}

	if len(withoutFtype) > 0 {
		// A filesystem is not reformatted between two attempts of the same check.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the XFS filesystems mounted on %s", host),
			Observed: fmt.Sprintf("%s formatted with ftype=0", strings.Join(withoutFtype, ", ")),
			Expected: "an XFS filesystem formatted with ftype=1",
			Fix:      "recreate the filesystem with `mkfs.xfs -n ftype=1` and restore its contents from a backup",
		})
	}

	return fmt.Sprintf("all XFS filesystems on %s are formatted with ftype=1 (%d checked)", host, len(devices)), nil
}

// xfsDevices reads the device names out of `mount -l -t xfs`, whose first column they are.
func xfsDevices(mountOutput string) []string {
	var devices []string
	for _, line := range strings.Split(strings.TrimSpace(mountOutput), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		devices = append(devices, fields[0])
	}
	return devices
}

func NodeXFSFtype(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := NodeXFSFtypeCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        NodeXFSFtypeCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}

// NodeResolveHostnameCheck asks whether the node can resolve its own name.
//
// kubelet and kubeadm both look it up, and a node that cannot resolve it fails during bootstrap
// with an error about neither the hostname nor DNS. It is the classic missing line in /etc/hosts,
// and node-hostname — which checks the shape of the name — says nothing about it.
type NodeResolveHostnameCheck struct {
	NodeInterface NodeInterfaceFunc
}

const NodeResolveHostnameCheckName preflight.CheckName = "node-resolve-hostname"

func (NodeResolveHostnameCheck) Description() string {
	return "the node resolves its own hostname"
}

func (NodeResolveHostnameCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (NodeResolveHostnameCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c NodeResolveHostnameCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	hostname := commandOutput(ctx, nodeInterface, "hostname")
	if hostname == "" {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("`hostname` on %s", host),
			Observed: "`hostname` printed nothing",
			Expected: "a hostname set on the node",
			Fix:      "set a hostname on the node (hostnamectl set-hostname <name>)",
		})
	}

	resolved := commandOutput(ctx, nodeInterface, "getent", "hosts", hostname)
	if strings.TrimSpace(resolved) == "" {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("`getent hosts %s` on %s", hostname, host),
			Observed: fmt.Sprintf("the node cannot resolve its own hostname %q", hostname),
			Expected: "a hostname that resolves to an address of the node",
			Fix:      fmt.Sprintf("add %q to /etc/hosts on the node, or make it resolvable by DNS", hostname),
		})
	}

	address := strings.Fields(resolved)
	return fmt.Sprintf("%s resolves its own hostname %q to %s", host, hostname, address[0]), nil
}

func NodeResolveHostname(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := NodeResolveHostnameCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        NodeResolveHostnameCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
