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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// mandatoryKernelModules are the ones step 005 loads on every node, with no guard: a `modprobe`
// that fails there takes the whole bundle down and repeats until it is killed. erofs is added
// only for ContainerdV2, which is the one case step 005 does check and exit on.
var mandatoryKernelModules = []string{"br_netfilter", "overlay"}

// NodeKernelModulesCheck asks whether the kernel modules Deckhouse loads can be loaded.
//
// A minimal cloud image is the usual way to meet this: the distribution ships a kernel without
// the extra module package, `modprobe overlay` fails, and the operator sees the failure from
// inside the retry storm with no mention of which package supplies the module.
type NodeKernelModulesCheck struct {
	MetaConfig    *config.MetaConfig
	NodeInterface NodeInterfaceFunc
}

const NodeKernelModulesCheckName preflight.CheckName = "node-kernel-modules"

func (NodeKernelModulesCheck) Description() string {
	return "the kernel modules Deckhouse needs can be loaded on the node"
}

func (NodeKernelModulesCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (NodeKernelModulesCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c NodeKernelModulesCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	wanted := append([]string(nil), mandatoryKernelModules...)
	if declaredCRI(c.MetaConfig) == containerdV2CRI {
		wanted = append(wanted, "erofs")
	}

	var missing []string
	for _, module := range wanted {
		// -n is a dry run and -q keeps it silent: nothing is loaded and nothing is printed,
		// so asking costs nothing and changes nothing on the node.
		if err := nodeInterface.Command("sudo", "modprobe", "-n", "-q", module).Run(ctx); err != nil {
			missing = append(missing, module)
		}
	}

	if len(missing) > 0 {
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("the kernel modules Deckhouse loads on %s", host),
			Observed: "cannot be loaded: " + strings.Join(missing, ", "),
			Expected: strings.Join(wanted, ", ") + " to be loadable",
			Fix: "install the kernel module package for the running kernel " +
				"(linux-modules-extra-$(uname -r) on Debian and Ubuntu, kernel-modules-extra on RHEL and its derivatives) " +
				"and reboot if the kernel changes",
		}
	}

	return fmt.Sprintf("%s can load %s", host, strings.Join(wanted, ", ")), nil
}

func NodeKernelModules(metaConfig *config.MetaConfig, nodeInterface NodeInterfaceFunc) preflight.Check {
	check := NodeKernelModulesCheck{MetaConfig: metaConfig, NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        NodeKernelModulesCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}

// selinuxPolicyTools are what step 005 compiles the Deckhouse SELinux policy with. They are not
// part of a minimal RedOS or RHEL install, and the step runs them without checking: the operator
// gets "checkmodule: command not found" from inside the storm (issue #15167).
var selinuxPolicyTools = []string{"checkmodule", "semodule_package", "semodule"}

// NodeSELinuxToolsCheck asks, on a node running SELinux in Enforcing mode, whether the tools that
// install the Deckhouse policy are there.
type NodeSELinuxToolsCheck struct {
	NodeInterface NodeInterfaceFunc
}

const NodeSELinuxToolsCheckName preflight.CheckName = "node-selinux-tools"

func (NodeSELinuxToolsCheck) Description() string {
	return "a node with SELinux enforcing has the tools to install the Deckhouse policy"
}

func (NodeSELinuxToolsCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (NodeSELinuxToolsCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c NodeSELinuxToolsCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	// The same two gates step 005 opens with: no getenforce at all, or SELinux not enforcing,
	// and the policy is never compiled.
	mode := commandOutput(ctx, nodeInterface, "getenforce")
	if mode == "" {
		return "", preflight.NotApplicable("%s does not have SELinux", host)
	}
	if !strings.EqualFold(mode, "Enforcing") {
		return "", preflight.NotApplicable("SELinux on %s is %s, so no policy is installed", host, mode)
	}

	var missing []string
	for _, tool := range selinuxPolicyTools {
		if err := nodeInterface.Command("command", "-v", tool).Run(ctx); err != nil {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("the SELinux policy tools on %s", host),
			Observed: "missing: " + strings.Join(missing, ", "),
			Expected: strings.Join(selinuxPolicyTools, ", ") + " on a node with SELinux enforcing",
			Fix: "install them (dnf install checkpolicy policycoreutils-python-utils), " +
				"or take SELinux out of enforcing mode on this node",
		}
	}

	return fmt.Sprintf("SELinux on %s is enforcing and the policy tools are present", host), nil
}

func NodeSELinuxTools(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := NodeSELinuxToolsCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        NodeSELinuxToolsCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
