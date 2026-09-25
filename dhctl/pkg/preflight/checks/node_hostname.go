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
	"regexp"
	"strings"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// nodeNamePattern is the rule bashible applies in 005_check_hostname.sh.tpl, and the rule
// Kubernetes applies to an object name: a DNS-1123 subdomain, at most 63 characters.
var nodeNamePattern = regexp.MustCompile(`^[a-z0-9](([a-z0-9\-.]{0,61}[a-z0-9])|[a-z0-9]{0,62})$`)

// NodeHostnameCheck reads the node's hostname and applies that rule here, before anything is
// installed.
//
// bashible applies it too, but at step 005, after eleven packages have been installed — and its
// failure lands inside the retry storm, so "FAIL Hostname '…' should be contain no more than 63
// characters" is printed over and over. The node's PKI is generated from this name, so it cannot
// be fixed afterwards without recreating the node (issues #5048, #5072).
type NodeHostnameCheck struct {
	NodeInterface NodeInterfaceFunc
}

const NodeHostnameCheckName preflight.CheckName = "node-hostname"

func (NodeHostnameCheck) Description() string {
	return "the node hostname is one Kubernetes accepts as a node name"
}

func (NodeHostnameCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (NodeHostnameCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c NodeHostnameCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	host := hostPhrase(nodeInterface)

	stdout, _, err := nodeInterface.Command("hostname").Output(ctx)
	if err != nil {
		return "", scriptFailure("read the hostname", nodeInterface, nil, err)
	}

	hostname := strings.TrimSpace(string(stdout))
	if hostname == "" {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("`hostname` on %s", host),
			Observed: "`hostname` printed nothing",
			Expected: "a hostname set on the node",
			Fix:      "set a hostname on the node (hostnamectl set-hostname <name>)",
		})
	}

	if !nodeNamePattern.MatchString(hostname) {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the hostname of %s", host),
			Observed: fmt.Sprintf("%q %s", hostname, hostnameProblem(hostname)),
			Expected: "a hostname of at most 63 characters, made of lower-case letters, digits, '-' and '.', " +
				"starting and ending with a letter or a digit",
			Fix: "rename the node before bootstrapping it (hostnamectl set-hostname <name>)",
		})
	}

	return fmt.Sprintf("%s is named %q", host, hostname), nil
}

// hostnameProblem says which of the rules the name broke, rather than restating all of them.
func hostnameProblem(hostname string) string {
	switch {
	case len(hostname) > 63:
		return fmt.Sprintf("is %d characters, the limit is 63", len(hostname))
	case strings.ToLower(hostname) != hostname:
		return "contains upper-case letters"
	case strings.ContainsAny(hostname, "_"):
		return "contains an underscore"
	case strings.HasPrefix(hostname, "-") || strings.HasPrefix(hostname, "."):
		return "starts with '-' or '.'"
	case strings.HasSuffix(hostname, "-") || strings.HasSuffix(hostname, "."):
		return "ends with '-' or '.'"
	default:
		return "contains characters a node name may not have"
	}
}

func NodeHostname(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := NodeHostnameCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        NodeHostnameCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
