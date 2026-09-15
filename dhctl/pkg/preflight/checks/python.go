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
	"strings"

	libcon "github.com/deckhouse/lib-connection/pkg"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type PythonCheck struct {
	// NodeInterface is resolved when the check runs, not when its suite is built: see
	// NodeInterfaceFunc.
	NodeInterface NodeInterfaceFunc
}

const PythonCheckName preflight.CheckName = "python-modules"

func (PythonCheck) Description() string {
	return "python and required modules are installed"
}

func (PythonCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (PythonCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c PythonCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	pythonBinary, err := detectPythonBinary(ctx, nodeInterface)
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("python3, python2 and python on %s", host),
			Observed: "none of them is on PATH",
			Expected: "a Python interpreter, which the bootstrap scripts are written against",
			Fix:      "install python3 on the node",
			Err:      err,
		})
	}

	requiredPythonModules := [][]string{
		{"urllib.request", "urllib2"},
		{"urllib.error", "urllib2"},
		{"configparser", "ConfigParser"},
		{"http.server", "SimpleHTTPServer"},
		{"http.server", "SocketServer"},
	}

	// Every missing module, not the first: they are installed together, and reporting them one
	// per run costs a bootstrap attempt each.
	var missing []string

	for _, moduleSet := range requiredPythonModules {
		found := false
		for _, moduleName := range moduleSet {
			cmd := nodeInterface.Command(pythonBinary, "-c", "import "+moduleName)
			if err := cmd.Run(ctx); err != nil {
				// A non-zero status is the module being absent; 255 is the transport failing.
				if status, ok := exitStatus(err); ok && status != 255 {
					continue
				}
				return "", scriptFailure("check the python modules", nodeInterface, nil, err)
			}
			found = true
			break
		}
		if !found {
			missing = append(missing, strings.Join(moduleSet, " or "))
		}
	}

	if len(missing) > 0 {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the modules %s needs on %s", pythonBinary, host),
			Observed: "- " + strings.Join(missing, "\n- "),
			Expected: "the standard library modules the bootstrap scripts import",
			Fix:      fmt.Sprintf("install the missing modules on the node (on most distributions they come with the %s package)", pythonBinary),
		})
	}

	return fmt.Sprintf("%s on %s has the required modules", pythonBinary, host), nil
}

func detectPythonBinary(ctx context.Context, nodeInterface libcon.Interface) (string, error) {
	possibleBinaries := []string{"python3", "python2", "python"}

	for _, binary := range possibleBinaries {
		err := nodeInterface.Command("command", "-v", binary).Run(ctx)
		if err == nil {
			return binary, nil
		}
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() != 255 {
			continue
		}
		return "", fmt.Errorf("Unexpected error during python binary lookup: %w", err)
	}

	return "", fmt.Errorf(
		"Python was not found under any of the expected names (%s), please install Python 2 or 3 on the node",
		strings.Join(possibleBinaries, ", "),
	)
}

func Python(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := PythonCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        PythonCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
