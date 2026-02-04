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

type DeckhouseUserCheck struct {
	Node node.Interface
}

const DeckhouseUserCheckName preflightnew.CheckName = "deckhouse-user"

func (DeckhouseUserCheck) Description() string {
	return "deckhouse user and group aren't present on node"
}

func (DeckhouseUserCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePostInfra
}

func (DeckhouseUserCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.DefaultRetryPolicy
}

func (c DeckhouseUserCheck) Run(ctx context.Context) error {
	file, err := template.RenderAndSavePreflightCheckDeckhouseUserScript()
	if err != nil {
		return err
	}

	cmd := c.Node.UploadScript(file)
	out, err := cmd.Execute(ctx)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("Deckhouse user existence check failed: %w, %s", err, string(ee.Stderr))
		}
		return fmt.Errorf("Could not execute a script to check deckhouse user and group aren't present on the node: %w", err)
	}

	_ = strings.TrimSpace(string(out))
	return nil
}

func DeckhouseUser(nodeInterface node.Interface) preflightnew.Check {
	check := DeckhouseUserCheck{Node: nodeInterface}
	return preflightnew.Check{
		Name:        DeckhouseUserCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
