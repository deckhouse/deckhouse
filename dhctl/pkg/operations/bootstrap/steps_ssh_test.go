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

package bootstrap

import (
	"testing"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSSHWaitAdvice: the wait fails after four minutes, and what the reader needs first is which
// connection it was — the bastion hop in particular is invisible in the old message.
func TestSSHWaitAdvice(t *testing.T) {
	t.Run("a direct connection names host, port and user", func(t *testing.T) {
		advice := sshWaitAdvice(session.NewSession(session.Input{
			User:           "ubuntu",
			Port:           "2222",
			AvailableHosts: []session.Host{{Host: "10.0.0.5"}},
		}))

		assert.Contains(t, advice, `10.0.0.5:2222`)
		assert.Contains(t, advice, `as "ubuntu"`)
		assert.NotContains(t, advice, "bastion")
	})

	t.Run("the default port is not invented", func(t *testing.T) {
		advice := sshWaitAdvice(session.NewSession(session.Input{
			User:           "ubuntu",
			AvailableHosts: []session.Host{{Host: "10.0.0.5"}},
		}))

		assert.Contains(t, advice, "10.0.0.5 as")
		assert.NotContains(t, advice, "10.0.0.5:")
	})

	t.Run("a bastion hop is named", func(t *testing.T) {
		advice := sshWaitAdvice(session.NewSession(session.Input{
			User:           "ubuntu",
			BastionHost:    "bastion.example.com",
			BastionUser:    "jump",
			AvailableHosts: []session.Host{{Host: "10.0.0.5"}},
		}))

		assert.Contains(t, advice, `through the bastion bastion.example.com as "jump"`)
	})

	t.Run("a bastion without its own user falls back to the SSH user", func(t *testing.T) {
		// --ssh-bastion-user is optional; when it is not given the same user is used for
		// both hops, and saying nothing would leave the reader checking the wrong account.
		advice := sshWaitAdvice(session.NewSession(session.Input{
			User:           "ubuntu",
			BastionHost:    "bastion.example.com",
			AvailableHosts: []session.Host{{Host: "10.0.0.5"}},
		}))

		assert.Contains(t, advice, `through the bastion bastion.example.com as "ubuntu"`)
	})

	t.Run("no session means no advice", func(t *testing.T) {
		assert.Empty(t, sshWaitAdvice(nil))
	})

	t.Run("every variant names the four causes", func(t *testing.T) {
		advice := sshWaitAdvice(session.NewSession(session.Input{
			User:           "ubuntu",
			AvailableHosts: []session.Host{{Host: "10.0.0.5"}},
		}))

		require.NotEmpty(t, advice)
		for _, cause := range []string{"booted", "22/TCP", "user exists", "key"} {
			assert.Contains(t, advice, cause)
		}
	})
}
