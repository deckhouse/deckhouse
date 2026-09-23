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

// What Deckhouse needs of any node, whatever container runtime it runs. The kernel floor is the
// one the documentation states; the package manager is how bashible installs anything at all.
const (
	minimumKernelMajor = 5
	minimumKernelMinor = 8
)

// supportedPackageManagers are the ones bashible knows how to drive. A node with none of them
// reaches step 031 and fails with "bb-pkg: action 'install' is not supported", inside the retry
// storm (issue #3169).
var supportedPackageManagers = []string{"apt-get", "apt", "dnf", "yum", "rpm", "zypper"}

// NodeOSSupportedCheck asks whether the node is one bashible can configure at all.
//
// bashible detects the distribution at step 56 and only warns when it does not recognise it; what
// the operator then sees is "bundle unknown" or a package manager error several steps later, with
// nothing connecting either to the distribution being unsupported.
type NodeOSSupportedCheck struct {
	NodeInterface NodeInterfaceFunc
}

const NodeOSSupportedCheckName preflight.CheckName = "node-os-supported"

func (NodeOSSupportedCheck) Description() string {
	return "the node runs an operating system Deckhouse can configure"
}

func (NodeOSSupportedCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (NodeOSSupportedCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c NodeOSSupportedCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	var unmet []string
	var found []string

	kernel := commandOutput(ctx, nodeInterface, "uname", "-r")
	switch major, minor, ok := parseKernelVersion(kernel); {
	case !ok:
		unmet = append(unmet, fmt.Sprintf("the kernel version could not be read (uname -r said %q)", kernel))
	case major < minimumKernelMajor || (major == minimumKernelMajor && minor < minimumKernelMinor):
		unmet = append(unmet, fmt.Sprintf("kernel %s, at least %d.%d is required",
			kernel, minimumKernelMajor, minimumKernelMinor))
	default:
		found = append(found, "kernel "+kernel)
	}

	if manager := firstAvailable(ctx, nodeInterface, supportedPackageManagers); manager != "" {
		found = append(found, manager)
	} else {
		unmet = append(unmet, fmt.Sprintf("none of %s is installed",
			strings.Join(supportedPackageManagers, ", ")))
	}

	// systemd runs every unit Deckhouse installs, starting with the kubelet.
	if _, ok := parseSystemdVersion(commandOutput(ctx, nodeInterface, "systemctl", "--version")); !ok {
		unmet = append(unmet, "systemd is not running on the node")
	} else {
		found = append(found, "systemd")
	}

	if len(unmet) > 0 {
		// None of these changes without reinstalling or rebooting the node.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the operating system of %s", host),
			Observed: "- " + strings.Join(unmet, "\n- "),
			Expected: fmt.Sprintf("kernel %d.%d or newer, systemd, and one of %s",
				minimumKernelMajor, minimumKernelMinor, strings.Join(supportedPackageManagers, ", ")),
			Fix: "use one of the supported distributions " +
				"(https://deckhouse.io/products/kubernetes-platform/documentation/v1/supported_versions.html)",
		})
	}

	return fmt.Sprintf("%s has %s", host, strings.Join(found, ", ")), nil
}

// firstAvailable returns the first of the binaries that is on PATH.
func firstAvailable(ctx context.Context, nodeInterface nodeCommandRunner, binaries []string) string {
	for _, binary := range binaries {
		if nodeInterface.Command("command", "-v", binary).Run(ctx) == nil {
			return binary
		}
	}
	return ""
}

func NodeOSSupported(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := NodeOSSupportedCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        NodeOSSupportedCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
