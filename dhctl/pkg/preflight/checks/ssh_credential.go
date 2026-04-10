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
	"fmt"

	"github.com/deckhouse/lib-connection/pkg/ssh"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

type SSHCredentialCheck struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
}

var ErrAuthSSHFailed = fmt.Errorf("authentication failed")

const SSHCredentialCheckName preflight.CheckName = "static-ssh-credential"

func (*SSHCredentialCheck) Description() string {
	return "ssh credentials are valid"
}

func (*SSHCredentialCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (*SSHCredentialCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.DefaultRetryPolicy
}

func (c *SSHCredentialCheck) Run(ctx context.Context) error {
	nodeInterface, err := helper.GetNodeInterface(c.SSHProviderInitializer, ctx, c.SSHProviderInitializer.GetSettings())
	if err != nil {
		return err
	}
	wrapper, ok := nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nil
	}
	if err := wrapper.Client().Check().CheckAvailability(ctx); err != nil {
		return fmt.Errorf("ssh %w. Please check ssh credential and try again. Error: %w", ErrAuthSSHFailed, err)
	}
	return nil
}

func SSHCredential(sshProvider *providerinitializer.SSHProviderInitializer) preflight.Check {
	check := SSHCredentialCheck{SSHProviderInitializer: sshProvider}
	return preflight.Check{
		Name:        SSHCredentialCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
