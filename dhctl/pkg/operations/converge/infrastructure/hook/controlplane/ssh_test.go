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

package controlplane

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/testssh"
)

// The checker used to copy the live session, so it reached every master as whoever the
// clients happened to run as. A master rebuilt by this converge answers to another user,
// and checking it as the operator fails the readiness gate that guards the next master.
func TestSSHCheckerAsksForTheNodesOwnSession(t *testing.T) {
	const (
		address = "10.12.1.2"
		// The builder hands back a host the checker was never given: only the session it
		// returns reaches it, so a client built from a copy of the live session fails
		// here — and so does one built under the wrong user on a real node.
		rebuilt = "10.12.1.99"
	)

	live := session.NewSession(session.Input{
		User:           "ubuntu",
		Port:           "22",
		AvailableHosts: []session.Host{{Host: "10.12.1.1", Name: "cluster-master-0"}},
	})

	provider := testssh.NewSSHProvider(live, true)
	provider.AddCommandProvider(rebuilt, func(_ testssh.Bastion, _ string, _ ...string) *testssh.Command {
		return testssh.NewCommand(nil)
	})

	var asked session.Host

	checker := NewSSHChecker(
		provider,
		map[string]string{"cluster-master-1": address},
		func(base *session.Session, host session.Host) (*session.Session, error) {
			asked = host

			return session.NewSession(session.Input{
				User:           "d8-converge",
				Port:           base.Port,
				AvailableHosts: []session.Host{{Host: rebuilt, Name: host.Name}},
			}), nil
		},
	)

	ready, err := checker.IsReady(t.Context(), "cluster-master-1")
	require.NoError(t, err)
	require.True(t, ready)
	require.Equal(t, session.Host{Host: address, Name: "cluster-master-1"}, asked)
}
