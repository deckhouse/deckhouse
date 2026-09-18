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

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type PortsCheck struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
	globalOptions          *options.GlobalOptions
}

const PortsCheckName preflight.CheckName = "ports-availability"

func (PortsCheck) Description() string {
	return "required ports are open on the node"
}

func (PortsCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (PortsCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c PortsCheck) Run(ctx context.Context) error {
	nodeInterface, err := ResolveNodeInterface(ctx, c.SSHProviderInitializer)
	if err != nil {
		return err
	}
	return checkAvailabilityPorts(ctx, nodeInterface, c.globalOptions)
}

func checkAvailabilityPorts(ctx context.Context, nodeInterface libcon.Interface, globalOptions *options.GlobalOptions) error {
	file, err := template.RenderAndSavePreflightCheckPortsScript(ctx, globalOptions)
	if err != nil {
		return err
	}

	scriptCmd := nodeInterface.UploadScript(file)
	out, err := scriptCmd.Execute(ctx)
	if err != nil {
		return scriptFailure("check that the required ports are free", nodeInterface, out, err)
	}

	return nil
}

func Ports(sshProviderInitializer *providerinitializer.SSHProviderInitializer, globalOptions *options.GlobalOptions) preflight.Check {
	check := PortsCheck{SSHProviderInitializer: sshProviderInitializer, globalOptions: globalOptions}
	return preflight.Check{
		Name:        PortsCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         preflight.Detailless(check.Run),
	}
}
