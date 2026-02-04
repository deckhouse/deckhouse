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

type SingleSSHHostCheck struct{ Node node.Interface }

const SingleSSHHostCheckName preflightnew.CheckName = "static-single-ssh-host"

func (SingleSSHHostCheck) Description() string {
	return "only one ssh host is provided"
}

func (SingleSSHHostCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePostInfra
}

func (SingleSSHHostCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.RetryPolicy{Attempts: 1}
}

func (c SingleSSHHostCheck) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, preflightnew.DefaultPreflightCheckTimeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}

	wrapper, ok := c.Node.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nil
	}
	if len(wrapper.Client().Session().AvailableHosts()) > 1 {
		return fmt.Errorf("during the bootstrap of the first static master node, only one --ssh-host parameter is allowed")
	}
	return nil
}

func SingleSSHHost(nodeInterface node.Interface) preflightnew.Check {
	check := SingleSSHHostCheck{Node: nodeInterface}
	return preflightnew.Check{
		Name:        SingleSSHHostCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
