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

	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

type SSHCredentialCheck struct{ Node node.Interface }

var ErrAuthSSHFailed = fmt.Errorf("authentication failed")

const SSHCredentialCheckName preflightnew.CheckName = "static-ssh-credential"

func (SSHCredentialCheck) Description() string {
	return "ssh credentials are valid"
}

func (SSHCredentialCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePostInfra
}

func (SSHCredentialCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.DefaultRetryPolicy
}

func (c SSHCredentialCheck) Run(ctx context.Context) error {
	wrapper, ok := c.Node.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nil
	}
	if err := wrapper.Client().Check().CheckAvailability(ctx); err != nil {
		return fmt.Errorf("ssh %w. Please check ssh credential and try again. Error: %w", ErrAuthSSHFailed, err)
	}
	return nil
}

func SSHCredential(nodeInterface node.Interface) preflightnew.Check {
	check := SSHCredentialCheck{Node: nodeInterface}
	return preflightnew.Check{
		Name:        SSHCredentialCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
