// Copyright 2025 Flant JSC
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

	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

type SudoAllowedCheck struct {
	Node node.Interface
}

const SudoAllowedCheckName preflightnew.CheckName = "sudo-allowed"

func (SudoAllowedCheck) Description() string {
	return "sudo is allowed for user"
}

func (SudoAllowedCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePostInfra
}

func (SudoAllowedCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.DefaultRetryPolicy
}

func (SudoAllowedCheck) Enabled() bool {
	return true
}

func (c SudoAllowedCheck) Run(ctx context.Context) error {
	cmd := c.Node.Command("echo")
	cmd.Sudo(ctx)

	if err := cmd.Run(ctx); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() != 255 {
			return errors.New("Provided SSH user is not allowed to sudo, please check that your password is correct and that this user is in the sudoers file.")
		}
		return fmt.Errorf("Unexpected error when checking sudoers permissions for SSH user: %v", err)
	}

	return nil
}

func SudoAllowed(nodeInterface node.Interface) preflightnew.Check {
	check := SudoAllowedCheck{Node: nodeInterface}
	return preflightnew.Check{
		Name:        SudoAllowedCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Enabled:     check.Enabled,
		Run:         check.Run,
	}
}
