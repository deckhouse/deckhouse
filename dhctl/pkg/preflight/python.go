// Copyright 2024 Flant JSC
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
)

func (pc *Checker) CheckPythonAndItsModules() error {
	if app.PreflightSkipPythonChecks {
		log.InfoLn("Python installation preflight check was skipped")
		return nil
	}

	pythonBinary, err := detectPythonBinary(pc.sshClient)
	if err != nil {
		return fmt.Errorf("Detect Python binary name: %w", err)
	}

	// Each subslice is a Python 3 module name and a python 2 fallback for it
	requiredPythonModules := [][]string{
		{"urllib.request", "urllib2"},
		{"urllib.error", "urllib2"},
		{"configparser", "ConfigParser"},
		{"http.server", "SimpleHTTPServer"},
		{"http.server", "SocketServer"},
	}

	for _, moduleSet := range requiredPythonModules {
		atLeastOneModuleFoundForSet := false
		for _, moduleName := range moduleSet {
			cmd := pc.sshClient.Command(pythonBinary, "-c", "import "+moduleName)
			err := cmd.Run()
			if err != nil {
				var ee *exec.ExitError
				if errors.As(err, &ee) && ee.ExitCode() != 255 {
					log.InfoF("Module %q is not installed\n", moduleName)
					continue
				}
				return fmt.Errorf("Unexpected error during python modules validation: %w", err)
			}

			atLeastOneModuleFoundForSet = true
			log.InfoF("Module %q is installed\n", moduleName)
			break
		}

		if !atLeastOneModuleFoundForSet {
			return fmt.Errorf(
				"Please install at least one of the following python modules on the node to continue: %s",
				strings.Join(moduleSet, ", "),
			)
		}
	}

	return nil
}

func detectPythonBinary(sshCl *ssh.Client) (string, error) {
	possibleBinaries := []string{"python3", "python2", "python"}
	for _, binary := range possibleBinaries {
		err := sshCl.Command("command", "-v", binary).Run()
		if err == nil {
			return binary, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() != 255 {
			continue
		}
		return "", fmt.Errorf("Unexpected error during python binary lookup: %w", err)
	}

	return "", fmt.Errorf(
		"Python was not found under any of expected names (%s), please install Python 2 or 3 on the node",
		strings.Join(possibleBinaries, ", "),
	)
}
