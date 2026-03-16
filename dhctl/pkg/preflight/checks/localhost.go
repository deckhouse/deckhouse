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

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type LocalhostDomainCheck struct {
	Node node.Interface
}

const LocalhostDomainCheckName preflight.CheckName = "resolve-localhost"

func (LocalhostDomainCheck) Description() string {
	return "resolve the localhost domain"
}

func (LocalhostDomainCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (LocalhostDomainCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.DefaultRetryPolicy
}

func (c LocalhostDomainCheck) Run(ctx context.Context) error {
	file, err := template.RenderAndSavePreflightCheckLocalhostScript()
	if err != nil {
		return err
	}

	cmd := c.Node.UploadScript(file)
	out, err := cmd.Execute(ctx)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("Localhost domain resolving check failed: %w, %s", err, string(ee.Stderr))
		}
		return fmt.Errorf("Could not execute a script to check for localhost domain resolution: %w", err)
	}

	_ = strings.TrimSpace(string(out))
	return nil
}

func LocalhostDomain(nodeInterface node.Interface) preflight.Check {
	check := LocalhostDomainCheck{Node: nodeInterface}
	return preflight.Check{
		Name:        LocalhostDomainCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
