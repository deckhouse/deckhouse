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
	"strings"

	"github.com/deckhouse/lib-connection/pkg/ssh"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type SingleSSHHostCheck struct {
	// NodeInterface resolves the connection at the moment the check runs — see NodeInterfaceFunc.
	NodeInterface NodeInterfaceFunc
}

const SingleSSHHostCheckName preflight.CheckName = "static-single-ssh-host"

func (SingleSSHHostCheck) Description() string {
	return "only one ssh host is provided"
}

func (SingleSSHHostCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (SingleSSHHostCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c SingleSSHHostCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	wrapper, ok := nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return "", preflight.NotApplicable("dhctl was given no SSH host")
	}

	hosts := wrapper.Client().Session().AvailableHosts()
	if len(hosts) > 1 {
		addresses := make([]string, 0, len(hosts))
		for _, host := range hosts {
			addresses = append(addresses, host.Host)
		}
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  "the --ssh-host arguments (or the SSHHost resources of --connection-config)",
			Observed: fmt.Sprintf("%d hosts were given: %s", len(hosts), strings.Join(addresses, ", ")),
			Expected: "one host, the machine the first master will be bootstrapped on",
			Fix: "pass a single --ssh-host for the bootstrap; the other masters are added afterwards " +
				"by `dhctl converge` once the first one is up",
		})
	}

	if len(hosts) == 0 {
		return "", preflight.NotApplicable("dhctl was given no SSH host")
	}
	return fmt.Sprintf("one ssh host was given: %s", hosts[0].Host), nil
}

func SingleSSHHost(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := SingleSSHHostCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        SingleSSHHostCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
