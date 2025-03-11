// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/alessio/shellescape"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

func (pc *Checker) CheckSudoIsAllowedForUser() error {
	log.InfoF("app.PreflightSkipSudoIsAllowedForUserCheck=%v\n", app.PreflightSkipSudoIsAllowedForUserCheck)
	if app.PreflightSkipSudoIsAllowedForUserCheck {
		log.DebugLn("sudoers preflight check is skipped")
		return nil
	}

	if app.AskBecomePass {
		return callSudo(pc.nodeInterface, app.BecomePass)
	}

	return callSudo(pc.nodeInterface, app.BecomePass)

}

func callSudo(nodeInterface node.Interface, password string) error {
	args := []string{"-Sv", "<<<", shellescape.Quote(password)}
	if password == "" {
		args = []string{"-n", "echo", "-n"}
	}

	cmd := nodeInterface.Command("sudo", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() != 255 {
			return fmt.Errorf("provided SSH user is not allowed to sudo, please check that your password is correct and that this user is in the sudoers file: %w; password: %q; output:%q; command:%q", err, password, out, strings.Join(append([]string{"sudo"}, args...), " "))
		}
		return fmt.Errorf("unexpected error when checking sudoers permissions for SSH user: %v", err)
	}

	return nil
}
