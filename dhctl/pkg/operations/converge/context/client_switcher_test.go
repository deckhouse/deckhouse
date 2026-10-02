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

package context

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
)

func TestPrivateKeysForSwitch(t *testing.T) {
	current := []session.AgentPrivateKey{
		{Key: "node-key"},
		{Key: "bastion-key"},
	}
	additional := session.AgentPrivateKey{Key: "converger-key", Passphrase: "secret"}

	require.Equal(t, current, privateKeysForSwitch(current, nil))
	require.Equal(t, []session.AgentPrivateKey{
		{Key: "node-key"},
		{Key: "bastion-key"},
		{Key: "converger-key", Passphrase: "secret"},
	}, privateKeysForSwitch(current, &additional))
}
