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

package suites

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunNodeAccessPreflightsWithoutHosts: converge, destroy and check all reach a cluster over a
// kubeconfig as often as over SSH, and with no machine to look at these checks have nothing to
// ask. Reporting an empty box there — or worse, failing — would make the kubeconfig path noisier
// than the one that has something to check.
func TestRunNodeAccessPreflightsWithoutHosts(t *testing.T) {
	require.NoError(t, RunNodeAccessPreflights(t.Context(), nil, nil, "Preflight checks: converge"),
		"no SSH provider at all is the kubeconfig path, not a failure")
}

// TestNodeAccessSuiteIsTheCriticalPathOnly: this suite is what stands between an operation and
// four minutes of silence, and it is deliberately two checks — reaching the machine and being
// allowed to act on it. Anything more would be asking a cluster that already exists about the
// configuration it was built from.
func TestNodeAccessSuiteIsTheCriticalPathOnly(t *testing.T) {
	suite := NewNodeAccessSuite(NodeAccessDeps{})

	names := make([]string, 0, len(suite.Checks()))
	for _, check := range suite.Checks() {
		names = append(names, check.Name.String())
	}

	assert.ElementsMatch(t, []string{"static-ssh-credential", "sudo-installed", "sudo-allowed"}, names)
}
