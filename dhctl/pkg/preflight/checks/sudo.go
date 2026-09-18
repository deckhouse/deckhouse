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

	libcon "github.com/deckhouse/lib-connection/pkg"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type SudoAllowedCheck struct {
	// NodeInterface is resolved when the check runs, not when its suite is built: see
	// NodeInterfaceFunc.
	NodeInterface NodeInterfaceFunc
}

// SudoInstalledCheck is the half of the old sudo check that asks whether the command exists at
// all. It is a separate check because the two have different fixes — install a package, or edit
// sudoers — and because the second is meaningless while the first fails.
//
// It applies to root as well. lib-connection wraps every privileged command in
// `sudo -p SudoPassword -H -S -i bash -c …` unconditionally, on both backends and regardless of
// the SSH user, so a root account on a node with no sudo binary fails at the first such command —
// which is why this check does not exempt root, and why the old advice to "use root user for
// bootstrap" was wrong.
type SudoInstalledCheck struct {
	NodeInterface NodeInterfaceFunc
}

const (
	SudoInstalledCheckName preflight.CheckName = "sudo-installed"
	SudoAllowedCheckName   preflight.CheckName = "sudo-allowed"
)

func (SudoInstalledCheck) Description() string {
	return "sudo is installed on the node"
}

func (SudoInstalledCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (SudoInstalledCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c SudoInstalledCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	if err := nodeInterface.Command("command", "-v", "sudo").Run(ctx); err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("`command -v sudo` on %s", host),
			Observed: "sudo is not installed",
			Expected: "sudo on PATH",
			Fix: "install the sudo package on the node. Connecting as root does not avoid this: " +
				"every privileged command dhctl runs is wrapped in sudo, whoever the SSH user is",
			Err: err,
		})
	}
	return fmt.Sprintf("sudo is installed on %s", host), nil
}

func SudoInstalled(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := SudoInstalledCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        check.Name(),
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}

func (SudoInstalledCheck) Name() preflight.CheckName { return SudoInstalledCheckName }

func (SudoAllowedCheck) Description() string {
	return "the SSH user may run commands through sudo"
}

func (SudoAllowedCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (SudoAllowedCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

// checkSudo asks whether the SSH user may actually use sudo. Whether sudo exists at all is
// sudo-installed's question, declared as a dependency of this one.
func checkSudo(ctx context.Context, nodeInterface libcon.Interface) error {
	host := hostPhrase(nodeInterface)

	cmd := nodeInterface.Command("true")
	cmd.Sudo(ctx)

	if err := cmd.Run(ctx); err == nil {
		return nil
	} else {
		// 255 is what the SSH transport itself exits with, so it says nothing about sudo; any
		// other status came from the node and is sudo refusing. The status is read through
		// exitStatus rather than *exec.ExitError, which the default backend never returns —
		// this branch used to be unreachable and every refusal fell through to the wrapper.
		if status, ok := exitStatus(err); ok && status != 255 {
			return preflight.Permanent(&preflight.Failure{
				Checked:  fmt.Sprintf("`sudo -n true` on %s", host),
				Observed: firstNonEmpty(strings.TrimSpace(string(cmd.StderrBytes())), "sudo refused"),
				Expected: "passwordless sudo for the SSH user, or a sudo password",
				Fix: "add the SSH user to sudoers on the node, " +
					"or pass the password with --ask-become-pass / becomePass in --connection-config. " +
					"This applies to root too: dhctl runs privileged commands through sudo regardless of the user",
				Err: err,
			})
		}

		// Status 255, or no status at all: the transport failed rather than sudo. The stderr is
		// the node's if there is any, and the transport error is what to act on if there is not.
		return &preflight.Failure{
			Checked:  fmt.Sprintf("`sudo -n true` on %s", host),
			Observed: firstNonEmpty(strings.TrimSpace(string(cmd.StderrBytes())), "the command did not complete"),
			Expected: "sudo to answer",
			Fix:      "check that the node is reachable over SSH and that the SSH user can run commands on it",
			Err:      err,
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (c SudoAllowedCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	if err := checkSudo(ctx, nodeInterface); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s can sudo", hostPhrase(nodeInterface)), nil
}

func SudoAllowed(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := SudoAllowedCheck{
		NodeInterface: nodeInterface,
	}

	return preflight.Check{
		Name:        SudoAllowedCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
