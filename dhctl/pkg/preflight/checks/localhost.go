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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type LocalhostDomainCheck struct {
	// NodeInterface is resolved when the check runs, not when its suite is built: see
	// NodeInterfaceFunc.
	NodeInterface NodeInterfaceFunc
	globalOptions *options.GlobalOptions
}

const LocalhostDomainCheckName preflight.CheckName = "resolve-localhost"

func (LocalhostDomainCheck) Description() string {
	return "the node resolves localhost to 127.0.0.1"
}

func (LocalhostDomainCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (LocalhostDomainCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c LocalhostDomainCheck) Run(ctx context.Context) error {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return err
	}

	file, err := template.RenderAndSavePreflightCheckLocalhostScript(ctx, c.globalOptions)
	if err != nil {
		return err
	}

	cmd := nodeInterface.UploadScript(file)
	out, err := cmd.Execute(ctx)
	if err != nil {
		return scriptFailure("check that localhost resolves", nodeInterface, out, err)
	}

	return nil
}

func LocalhostDomain(nodeInterface NodeInterfaceFunc, globalOptions *options.GlobalOptions) preflight.Check {
	check := LocalhostDomainCheck{NodeInterface: nodeInterface, globalOptions: globalOptions}
	return preflight.Check{
		Name:        LocalhostDomainCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         preflight.Detailless(check.Run),
	}
}
