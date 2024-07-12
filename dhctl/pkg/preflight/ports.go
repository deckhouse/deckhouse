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
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func (pc *Checker) CheckAvailabilityPorts() error {
	if app.PreflightSkipAvailabilityPorts {
		log.InfoLn("Availability ports preflight check was skipped")
		return nil
	}

	log.DebugLn("Checking availability ports")

	file, err := template.RenderAndSavePreflightCheckPortsScript()
	if err != nil {
		return err
	}

	scriptCmd := pc.sshClient.UploadScript(file)
	out, err := scriptCmd.Execute()
	if err != nil {
		log.ErrorLn(strings.Trim(string(out), "\n"))
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("Required ports check failed: %w, %s", err, string(ee.Stderr))
		}
		return fmt.Errorf("Could not execute a script to check if all necessary ports are open on the node: %w", err)
	}

	log.DebugLn(string(out))
	return nil
}
