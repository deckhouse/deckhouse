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

	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type PortsCheck struct {
	Node node.Interface
}

const PortsCheckName preflightnew.CheckName = "ports-availability"

func (PortsCheck) Description() string {
	return "required ports are open on the node"
}

func (PortsCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePostInfra
}

func (PortsCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.DefaultRetryPolicy
}

func (c PortsCheck) Run(ctx context.Context) error {
	if c.Node == nil {
		return fmt.Errorf("ports check: node interface is nil")
	}
	return checkAvailabilityPorts(ctx, c.Node)
}

func checkAvailabilityPorts(ctx context.Context, nodeInterface node.Interface) error {
	file, err := template.RenderAndSavePreflightCheckPortsScript()
	if err != nil {
		return err
	}

	scriptCmd := nodeInterface.UploadScript(file)
	out, err := scriptCmd.Execute(ctx)
	if err != nil {
		outMsg := strings.Trim(string(out), "\n")
		if outMsg != "" {
			return fmt.Errorf("required ports check failed: %s", outMsg)
		}

		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("required ports check failed: %w, %s", err, string(ee.Stderr))
		}
		return fmt.Errorf("Could not execute a script to check if all necessary ports are open on the node: %w", err)
	}

	return nil
}

func Ports(nodeInterface node.Interface) preflightnew.Check {
	check := PortsCheck{Node: nodeInterface}
	return preflightnew.Check{
		Name:        PortsCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
