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
	"errors"
	"fmt"
	"os/exec"

	libcon "github.com/deckhouse/lib-connection/pkg"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type SudoAllowedCheck struct {
	NodeInterface libcon.Interface
}

const SudoAllowedCheckName preflight.CheckName = "sudo-allowed"

func (SudoAllowedCheck) Description() string {
	return "sudo is installed and allowed for user"
}

func (SudoAllowedCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (SudoAllowedCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.DefaultRetryPolicy
}

// checkSudo checks that sudo is installed and that the SSH user
// is allowed to execute commands through sudo.
func checkSudo(ctx context.Context, nodeInterface libcon.Interface) error {
	checkInstalledCmd := nodeInterface.Command("command -v sudo >/dev/null 2>&1")
	if err := checkInstalledCmd.Run(ctx); err != nil {
		return errors.New(`required command "sudo" is not installed; install sudo or use root user for bootstrap`)
	}

	cmd := nodeInterface.Command("true")
	cmd.Sudo(ctx)

	if err := cmd.Run(ctx); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() != 255 {
			return errors.New(
				"provided SSH user is not allowed to sudo; check that the password is correct and that the user is in the sudoers file",
			)
		}

		return fmt.Errorf(
			"unexpected error when checking sudo permissions for SSH user: %w\nstderr: %s",
			err,
			string(cmd.StderrBytes()),
		)
	}

	return nil
}

func (c SudoAllowedCheck) Run(ctx context.Context) error {
	return checkSudo(ctx, c.NodeInterface)
}

func SudoAllowed(nodeInterface libcon.Interface) preflight.Check {
	check := SudoAllowedCheck{
		NodeInterface: nodeInterface,
	}

	return preflight.Check{
		Name:        SudoAllowedCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
