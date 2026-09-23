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

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type DeckhouseUserCheck struct {
	// NodeInterface is resolved when the check runs, not when its suite is built: see
	// NodeInterfaceFunc.
	NodeInterface NodeInterfaceFunc
	globalOptions *options.GlobalOptions
}

const DeckhouseUserCheckName preflight.CheckName = "deckhouse-user"

func (DeckhouseUserCheck) Description() string {
	return "no conflicting deckhouse user or group on the node"
}

func (DeckhouseUserCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (DeckhouseUserCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c DeckhouseUserCheck) Run(ctx context.Context) error {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return err
	}

	file, err := template.RenderAndSavePreflightCheckDeckhouseUserScript(ctx, c.globalOptions)
	if err != nil {
		return err
	}

	cmd := nodeInterface.UploadScript(file)
	out, err := cmd.Execute(ctx)
	if err != nil {
		return deckhouseUserFailure(nodeInterface, out, err)
	}

	return nil
}

// deckhouseUserFailure turns what the node reported into a verdict with a way out.
//
// The way out is the point. An account left behind by an earlier cluster is the usual cause, and
// the operator cannot act on "Deckhouse user existence check failed: execute on remote: exit
// status 1" — the old message, which dropped the script's own diagnosis and then retried it five
// times, as though a leftover account might go away by itself. The node says what it found; the
// report says what to run, and names the cleanup script by path because that is the one step that
// is not guessable.
func deckhouseUserFailure(nodeInterface libcon.Interface, out []byte, err error) error {
	observed := strings.TrimSpace(string(out))
	if observed == "" {
		// The script did not get far enough to say anything — a connection that dropped, a
		// missing interpreter. That is a different failure and scriptFailure names it.
		return scriptFailure("check the deckhouse user and group", nodeInterface, out, err)
	}

	return preflight.Permanent(&preflight.Failure{
		Checked:  fmt.Sprintf("the deckhouse user and group on %s", hostPhrase(nodeInterface)),
		Observed: observed,
		Expected: "no deckhouse user or group, or the pair Deckhouse creates itself (uid and gid 64535, no sudo)",
		Fix: "if this node ran Deckhouse before, run:\n" +
			"    sudo bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing\n" +
			"otherwise remove the account by hand:\n" +
			"    sudo userdel deckhouse && sudo groupdel deckhouse",
		Err: err,
	})
}

func DeckhouseUser(nodeInterface NodeInterfaceFunc, globalOptions *options.GlobalOptions) preflight.Check {
	check := DeckhouseUserCheck{NodeInterface: nodeInterface, globalOptions: globalOptions}
	return preflight.Check{
		Name:        DeckhouseUserCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         preflight.Detailless(check.Run),
	}
}
