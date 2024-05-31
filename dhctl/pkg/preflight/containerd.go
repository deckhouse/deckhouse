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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func (pc *Checker) CheckContainerdExist() error {
	if app.PreflightSkipContainerdExistCheck {
		log.InfoLn("Containerd exist preflight check was skipped")
		return nil
	}

	log.DebugLn("Checking containerd exist")

	file, err := template.RenderAndSavePreflightCheckScript("check_containerd.sh", nil)
	if err != nil {
		return err
	}

	serversWithError := make([]string, 0)
	err = pc.sshClient.Loop(func(sshClient *ssh.Client) error {
		var out []byte
		scriptCmd := sshClient.UploadScript(file)
		out, err = scriptCmd.Execute()
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				serversWithError = append(serversWithError, sshClient.Settings.Host())
				return nil
			}
			log.ErrorLn(strings.Trim(string(out), "\n"))
			return fmt.Errorf(
				"could not execute a script to check containerd exist: %w",
				err,
			)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(serversWithError) > 0 {
		return fmt.Errorf(
			"containerd exist on servers %s, check failed. Deckhouse not support working with embedded containerd. Please uninstall containerd and try again",
			strings.Join(serversWithError, ", "),
		)
	}

	return nil
}
