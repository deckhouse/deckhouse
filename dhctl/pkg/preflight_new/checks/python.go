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
	"strings"

	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

type PythonCheck struct {
	Node node.Interface
}

const PythonCheckName preflightnew.CheckName = "python-modules"

func (PythonCheck) Description() string {
	return "python and required modules are installed"
}

func (PythonCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePostInfra
}
func (PythonCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.DefaultRetryPolicy
}

func (PythonCheck) Enabled() bool {
	return true
}

func (c PythonCheck) Run(ctx context.Context) error {
	pythonBinary, err := detectPythonBinary(ctx, c.Node)
	if err != nil {
		return fmt.Errorf("Detect Python binary name: %w", err)
	}

	requiredPythonModules := [][]string{
		{"urllib.request", "urllib2"},
		{"urllib.error", "urllib2"},
		{"configparser", "ConfigParser"},
		{"http.server", "SimpleHTTPServer"},
		{"http.server", "SocketServer"},
	}

	for _, moduleSet := range requiredPythonModules {
		found := false
		for _, moduleName := range moduleSet {
			cmd := c.Node.Command(pythonBinary, "-c", "import "+moduleName)
			if err := cmd.Run(ctx); err != nil {
				var ee *exec.ExitError
				if errors.As(err, &ee) && ee.ExitCode() != 255 {
					continue
				}
				return fmt.Errorf("Unexpected error during python modules validation: %w", err)
			}
			found = true
			break
		}
		if !found {
			return fmt.Errorf("Please install at least one of the following python modules on the node to continue: %s", strings.Join(moduleSet, ", "))
		}
	}

	return nil
}

func detectPythonBinary(ctx context.Context, sshCl node.Interface) (string, error) {
	possibleBinaries := []string{"python3", "python2", "python"}
	for _, binary := range possibleBinaries {
		err := sshCl.Command("command", "-v", binary).Run(ctx)
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

func Python(nodeInterface node.Interface) preflightnew.Check {
	check := PythonCheck{Node: nodeInterface}
	return preflightnew.Check{
		Name:        PythonCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Enabled:     check.Enabled,
		Run:         check.Run,
	}
}
