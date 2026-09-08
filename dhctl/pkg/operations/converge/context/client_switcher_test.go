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

	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// An sshless converge has no way to log in to a node, so the switcher must not create
// the NodeUser and wait for bashible to report it: that wait never ends. The context
// carries no kube provider here — reaching the cluster at all would fail the test.
func TestSwitcherSkipsNodeUserWhenSSHless(t *testing.T) {
	// Own Kubernetes credentials and no SSH host known: nothing can reach a node.
	ctx := NewContext(t.Context(), Params{KubeOwnCredentials: true})
	require.True(t, ctx.SSHless())

	switcher := NewKubeClientSwitcher(ctx, nil, KubeClientSwitcherParams{})

	require.NoError(t, switcher.SwitchToNodeUser(t.Context(), nil))
	require.NoError(t, switcher.CleanupNodeUser())

	// Strict switches are skipped as well: with no SSH there is no session to move.
	require.NoError(t, switcher.SwitchToFirstMaster(t.Context()))
	require.NoError(t, switcher.SwitchToNotFirstMaster(t.Context()))
}

// Kubernetes credentials of its own do not make a converge sshless on their own: the
// hosts may well be there, and then every switch has to run as before.
func TestSSHlessIsFalseWhileHostsAreKnown(t *testing.T) {
	initializer := providerinitializer.NewSSHProviderInitializer(
		settings.NewBaseProviders(settings.ProviderParams{}),
		&sshconfig.ConnectionConfig{
			Config: &sshconfig.Config{},
			Hosts:  []sshconfig.Host{{Host: "10.12.1.10"}},
		},
	)

	ctx := NewContext(t.Context(), Params{KubeOwnCredentials: true, SSHProviderInitializer: initializer})

	require.False(t, ctx.SSHless())
}
