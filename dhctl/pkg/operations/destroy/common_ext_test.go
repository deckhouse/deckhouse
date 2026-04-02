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

package destroy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/testssh"
)

func testCreateDefaultTestSSHProviderExt(destroyOverHost session.Host, overBastion bool) *testssh.SSHProvider {
	initKeys := make([]session.AgentPrivateKey, 0, len(inputPrivateKeys))
	for _, key := range inputPrivateKeys {
		initKeys = append(initKeys, session.AgentPrivateKey{
			Key: key,
		})
	}

	input := session.Input{
		User:       inputUser,
		Port:       inputPort,
		BecomePass: "",
		AvailableHosts: []session.Host{
			destroyOverHost,
		},
	}

	if overBastion {
		input.BastionHost = bastionHost
		input.BastionUser = bastionUser
		input.BastionPort = bastionPort
	}

	return testssh.NewSSHProvider(session.NewSession(input), true).WithInitPrivateKeys(initKeys)
}

func assertOverDefaultBastionExt(t *testing.T, overBastion bool, bastion testssh.Bastion, tp string) {
	require.False(t, bastion.NoSession, "bastion should have session")

	assert := func(t *testing.T, expected, actual string) {
		require.Empty(t, actual, fmt.Sprintf("call '%s' should not over bastion", tp))
	}

	if overBastion {
		assert = func(t *testing.T, expected, actual string) {
			require.Equal(t, expected, actual, fmt.Sprintf("call '%s' should over bastion", tp))
		}
	}

	assert(t, bastionHost, bastion.Host)
	assert(t, bastionPort, bastion.Port)
	assert(t, bastionUser, bastion.User)
}
