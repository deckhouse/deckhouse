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

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// deckhouseContainerdPath is the only containerd Deckhouse installs, and the one binary whose
// presence is not a leftover.
const deckhouseContainerdPath = "/opt/deckhouse/bin/containerd"

// NodeLeftoversCheck looks for a container runtime or a Kubernetes the node already has.
//
// bashible refuses such a node at step 000, but inside the retry storm: the message repeats until
// the bundle is killed, and on a node that carries a previous Deckhouse install it does not even
// get that far — "Bashible has already run! Skipping" sends the run on to fail later with
// "Cluster UUIDs are not equal", whose advice is about a hostname and is wrong for a static
// cluster (issue #16040).
type NodeLeftoversCheck struct {
	NodeInterface NodeInterfaceFunc
}

const NodeLeftoversCheckName preflight.CheckName = "node-leftovers"

func (NodeLeftoversCheck) Description() string {
	return "the node carries no container runtime or Kubernetes of its own"
}

func (NodeLeftoversCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (NodeLeftoversCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c NodeLeftoversCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	var leftovers []string

	// containerd is the one with an exception: Deckhouse installs its own under
	// /opt/deckhouse/bin, and finding that one is the node being re-bootstrapped, not a
	// pre-provisioned runtime.
	if path := commandPath(ctx, nodeInterface, "containerd"); path != "" && path != deckhouseContainerdPath {
		leftovers = append(leftovers, fmt.Sprintf("containerd at %s, which Deckhouse did not install", path))
	}
	for _, binary := range []string{"dockerd", "kubelet", "crio"} {
		if path := commandPath(ctx, nodeInterface, binary); path != "" {
			leftovers = append(leftovers, fmt.Sprintf("%s at %s", binary, path))
		}
	}

	// A previous Deckhouse bootstrap leaves this behind, and it is what makes bashible say
	// "Bashible has already run! Skipping" and hand the run a node it never configures.
	if fileExists(ctx, nodeInterface, "/var/lib/bashible/bashible.sh") {
		leftovers = append(leftovers, "/var/lib/bashible, left by a previous Deckhouse bootstrap")
	}

	if len(leftovers) > 0 {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the container runtime and Kubernetes binaries on %s", host),
			Observed: "- " + strings.Join(leftovers, "\n- "),
			Expected: "a node with none of them: Deckhouse installs and owns its own",
			Fix: "remove them and their state (/var/lib/containerd, /var/lib/kubelet, /var/lib/bashible, " +
				"/etc/kubernetes), or bootstrap onto a clean machine",
		})
	}

	return fmt.Sprintf("%s carries no container runtime or Kubernetes of its own", host), nil
}

// commandPath returns where a binary is on the node, or "" when it is not there.
func commandPath(ctx context.Context, nodeInterface libcon.Interface, binary string) string {
	stdout, _, err := nodeInterface.Command("command", "-v", binary).Output(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(stdout))
}

func fileExists(ctx context.Context, nodeInterface libcon.Interface, path string) bool {
	return nodeInterface.Command("test", "-e", path).Run(ctx) == nil
}

func NodeLeftovers(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := NodeLeftoversCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        NodeLeftoversCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
