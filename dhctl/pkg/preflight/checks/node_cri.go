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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// What containerd v2 needs of the node it runs on, as
// candi/bashible/common-steps/all/000_check_containerd_v2_support.sh.tpl states it.
const (
	containerdV2MinKernelMajor = 5
	containerdV2MinKernelMinor = 8
	containerdV2MinSystemd     = 244
	containerdV2CRI            = "ContainerdV2"
	// What kernelHasErofsCVE actually matches, which is every 6.12.x and 6.14.x. The message used
	// to print the advisory's patch ranges (6.12.0–6.12.28, 6.14.0–6.14.6) while the matcher
	// ignored the patch level, so a node on 6.12.40 was told it was affected and shown a range it
	// sits outside of. See kernelHasErofsCVE: the matcher is the half that is wrong.
	containerdV2ErofsCVEMessage = "is affected by CVE-2025-37999 in EROFS (kernels 6.12.x and 6.14.x)"
)

// NodeCRIRequirementsCheck asks the node for what containerd v2 needs, before bashible does.
//
// bashible checks the same four things at step 000, and a node that fails them fails inside the
// retry storm — the message repeats until the bundle is killed. None of the four can be fixed
// without a reboot, and a kernel upgrade means recreating the node on most distributions.
//
// It applies only when the cluster asks for ContainerdV2: the default is Containerd, which has
// none of these requirements.
type NodeCRIRequirementsCheck struct {
	MetaConfig    *config.MetaConfig
	NodeInterface NodeInterfaceFunc
}

const NodeCRIRequirementsCheckName preflight.CheckName = "node-cri-requirements"

func (NodeCRIRequirementsCheck) Description() string {
	return "the node meets what the requested container runtime needs"
}

func (NodeCRIRequirementsCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (NodeCRIRequirementsCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c NodeCRIRequirementsCheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", fmt.Errorf("the cluster configuration was not passed to this check")
	}

	cri := c.declaredCRI()
	if cri != containerdV2CRI {
		return "", preflight.NotApplicable("the cluster uses %s, which has no kernel or systemd requirements of its own", criLabel(cri))
	}

	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	var unmet []string
	var reported []string

	kernel := commandOutput(ctx, nodeInterface, "uname", "-r")
	switch major, minor, ok := parseKernelVersion(kernel); {
	case !ok:
		unmet = append(unmet, fmt.Sprintf("the kernel version could not be read (uname -r said %q)", kernel))
	case major < containerdV2MinKernelMajor || (major == containerdV2MinKernelMajor && minor < containerdV2MinKernelMinor):
		unmet = append(unmet, fmt.Sprintf("kernel %s is older than %d.%d",
			kernel, containerdV2MinKernelMajor, containerdV2MinKernelMinor))
	case kernelHasErofsCVE(major, minor):
		unmet = append(unmet, fmt.Sprintf("kernel %s %s", kernel, containerdV2ErofsCVEMessage))
	default:
		reported = append(reported, "kernel "+kernel)
	}

	systemd := commandOutput(ctx, nodeInterface, "systemctl", "--version")
	switch version, ok := parseSystemdVersion(systemd); {
	case !ok:
		unmet = append(unmet, "the systemd version could not be read")
	case version < containerdV2MinSystemd:
		unmet = append(unmet, fmt.Sprintf("systemd %d is older than %d", version, containerdV2MinSystemd))
	default:
		reported = append(reported, fmt.Sprintf("systemd %d", version))
	}

	// cgroup2fs is what /sys/fs/cgroup is when the node boots on the unified hierarchy.
	if fsType := commandOutput(ctx, nodeInterface, "stat", "-fc", "%T", "/sys/fs/cgroup"); fsType != "cgroup2fs" {
		unmet = append(unmet, fmt.Sprintf("cgroup v2 is not in use (/sys/fs/cgroup is %q)", fsType))
	} else {
		reported = append(reported, "cgroup v2")
	}

	if !hasErofs(ctx, nodeInterface) {
		unmet = append(unmet, "the erofs kernel module is not available")
	} else {
		reported = append(reported, "erofs")
	}

	if len(unmet) > 0 {
		// None of these changes without a reboot, and most need a different kernel.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("what ContainerdV2 needs of %s", host),
			Observed: "- " + strings.Join(unmet, "\n- "),
			Expected: fmt.Sprintf("kernel %d.%d or newer, systemd %d or newer, cgroup v2 and the erofs module",
				containerdV2MinKernelMajor, containerdV2MinKernelMinor, containerdV2MinSystemd),
			Fix: "upgrade the node, or set defaultCRI to Containerd in the ClusterConfiguration",
		})
	}

	return fmt.Sprintf("%s supports ContainerdV2 (%s)", host, strings.Join(reported, ", ")), nil
}

func (c NodeCRIRequirementsCheck) declaredCRI() string {
	return declaredCRI(c.MetaConfig)
}

// declaredCRI is what the cluster asked for, or "" for the default. Two checks depend on it —
// this one and node-kernel-modules, which needs erofs only under ContainerdV2 — so they read the
// field through one function rather than each parsing it.
func declaredCRI(metaConfig *config.MetaConfig) string {
	if metaConfig == nil {
		return ""
	}
	raw, ok := metaConfig.ClusterConfig["defaultCRI"]
	if !ok || len(raw) == 0 {
		return ""
	}
	var cri string
	if err := json.Unmarshal(raw, &cri); err != nil {
		return ""
	}
	return cri
}

func criLabel(cri string) string {
	if cri == "" {
		return "the default container runtime"
	}
	return cri
}

// kernelHasErofsCVE covers the two ranges the bashible check names. A node inside them boots
// containerd v2 and then hits the bug in EROFS, which is what containerd v2 stores images on.
// kernelHasErofsCVE is coarser than the check it mirrors, and the comment here used to claim
// otherwise: is_kernel_erofs_cve_vulnerable in
// candi/bashible/common-steps/all/000_check_containerd_v2_support.sh.tpl compares full versions
// (6.12.0 up to but not including 6.12.29, and 6.14.0 up to but not including 6.14.7) and carries
// per-flavour exceptions for the -generic, -aws, -azure, -gcp, -oracle, -oem and el9uek/el10uek
// kernels whose fix was backported. This one flags every 6.12.x and 6.14.x, so it refuses a node
// on 6.12.40 that bashible would let through — a preflight that blocks a bootstrap the node would
// have survived. Narrowing it means comparing the patch level and porting those exceptions, which
// is a behaviour change rather than the wording fix this pass was.
func kernelHasErofsCVE(major, minor int) bool {
	return major == 6 && (minor == 12 || minor == 14)
}

// parseKernelVersion reads the major and minor out of what `uname -r` printed, which carries a
// distribution suffix on most nodes ("5.15.0-89-generic").
func parseKernelVersion(version string) (int, int, bool) {
	fields := strings.FieldsFunc(strings.TrimSpace(version), func(r rune) bool {
		return r == '.' || r == '-' || r == '+'
	})
	if len(fields) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, false
	}
	minor, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// parseSystemdVersion reads the number out of "systemd 249 (249.11-0ubuntu3)".
func parseSystemdVersion(output string) (int, bool) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "systemd" {
			continue
		}
		version, err := strconv.Atoi(strings.TrimSuffix(fields[1], "."))
		if err != nil {
			continue
		}
		return version, true
	}
	return 0, false
}

// hasErofs accepts the module either built in or loadable.
func hasErofs(ctx context.Context, nodeInterface libcon.Interface) bool {
	if strings.Contains(commandOutput(ctx, nodeInterface, "cat", "/proc/filesystems"), "erofs") {
		return true
	}
	return nodeInterface.Command("modprobe", "-n", "erofs").Run(ctx) == nil
}

// commandOutput returns a command's stdout, trimmed, or "" if it could not be run. The checks
// above turn an empty answer into their own message, which is more useful than the raw error.
func commandOutput(ctx context.Context, nodeInterface libcon.Interface, name string, args ...string) string {
	stdout, _, err := nodeInterface.Command(name, args...).Output(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(stdout))
}

func NodeCRIRequirements(metaConfig *config.MetaConfig, nodeInterface NodeInterfaceFunc) preflight.Check {
	check := NodeCRIRequirementsCheck{MetaConfig: metaConfig, NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        NodeCRIRequirementsCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
