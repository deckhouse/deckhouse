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
	gocontext "context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/testssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

func TestSelectMasterStates(t *testing.T) {
	first := &NodeState{Name: "cluster-master-0", State: []byte("s0")}
	others := []*NodeState{
		{Name: "cluster-master-1", State: []byte("s1")},
		{Name: "cluster-master-2", State: []byte("s2")},
	}

	t.Run("only first", func(t *testing.T) {
		got := selectMasterStates(first, others, func(name string) bool {
			return name == "cluster-master-0"
		})
		require.Equal(t, map[string][]byte{"cluster-master-0": []byte("s0")}, got)
	})

	t.Run("all but first", func(t *testing.T) {
		got := selectMasterStates(first, others, func(name string) bool {
			return name != "cluster-master-0"
		})
		require.Len(t, got, 2)
		require.NotContains(t, got, "cluster-master-0")
	})

	t.Run("excluding deleted", func(t *testing.T) {
		deleted := map[string]struct{}{"cluster-master-1": {}}
		got := selectMasterStates(first, others, func(name string) bool {
			_, ok := deleted[name]
			return !ok
		})
		require.Len(t, got, 2)
		require.NotContains(t, got, "cluster-master-1")
	})

	t.Run("nil first is skipped", func(t *testing.T) {
		got := selectMasterStates(nil, others, func(string) bool { return true })
		require.Len(t, got, 2)
	})
}

type fakeStateStore struct {
	state *State
}

func (s *fakeStateStore) GetState(*Context) (*State, error) { return s.state, nil }

func (s *fakeStateStore) SetState(*Context, *State) error { return nil }

func (s *fakeStateStore) Delete(*Context) error { return nil }

// switcherWithConvergeUserNodes builds a switcher whose converge state names the given
// masters as built by this converge, with "ubuntu" as the user dhctl was started with.
func switcherWithConvergeUserNodes(t *testing.T, connection *sshconfig.ConnectionConfig, nodes ...string) *KubeClientSwitcher {
	t.Helper()

	initializer := providerinitializer.NewSSHProviderInitializer(
		settings.NewBaseProviders(settings.ProviderParams{}),
		connection,
	)

	ctx := NewContext(t.Context(), Params{SSHProviderInitializer: initializer})
	ctx.stateStore = &fakeStateStore{state: &State{ConvergeUserNodes: nodes}}

	return NewKubeClientSwitcher(ctx, nil, KubeClientSwitcherParams{})
}

func operatorConnection() *sshconfig.ConnectionConfig {
	return &sshconfig.ConnectionConfig{
		Config: &sshconfig.Config{User: "ubuntu", SudoPassword: "become-pass"},
	}
}

func TestCredentialsForGeneration(t *testing.T) {
	t.Run("nodes created by this converge use the converge user", func(t *testing.T) {
		switcher := switcherWithConvergeUserNodes(t, operatorConnection(), "cluster-master-0", "cluster-master-1")

		creds, err := switcher.credentialsFor([]string{"cluster-master-1", "cluster-master-0"})
		require.NoError(t, err)
		require.Equal(t, global.ConvergeUserName, creds.User)
		// The converge user has NOPASSWD sudo and no password at all.
		require.Empty(t, creds.BecomePass)
	})

	t.Run("pre-existing nodes keep the user the operator passed", func(t *testing.T) {
		switcher := switcherWithConvergeUserNodes(t, operatorConnection(), "cluster-master-2")

		creds, err := switcher.credentialsFor([]string{"cluster-master-0", "cluster-master-1"})
		require.NoError(t, err)
		require.Equal(t, "ubuntu", creds.User)
		require.Equal(t, "become-pass", creds.BecomePass)
	})

	t.Run("a cluster this converge added nothing to keeps the operator user", func(t *testing.T) {
		switcher := switcherWithConvergeUserNodes(t, operatorConnection())

		creds, err := switcher.credentialsFor([]string{"cluster-master-0"})
		require.NoError(t, err)
		require.Equal(t, "ubuntu", creds.User)
	})

	t.Run("mixed set is rejected", func(t *testing.T) {
		switcher := switcherWithConvergeUserNodes(t, operatorConnection(), "cluster-master-0")

		_, err := switcher.credentialsFor([]string{"cluster-master-0", "cluster-master-1"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "cluster-master-0")
		require.Contains(t, err.Error(), "cluster-master-1")
	})

	t.Run("no nodes is rejected", func(t *testing.T) {
		switcher := switcherWithConvergeUserNodes(t, operatorConnection(), "cluster-master-0")

		_, err := switcher.credentialsFor(nil)
		require.Error(t, err)
	})

	t.Run("a connection without a user is rejected", func(t *testing.T) {
		switcher := switcherWithConvergeUserNodes(t, &sshconfig.ConnectionConfig{Config: &sshconfig.Config{}})

		_, err := switcher.credentialsFor([]string{"cluster-master-0"})
		require.Error(t, err)
	})
}

// failingOnceProvider refuses the first switch, the way a master built without the
// converge user refuses that user.
type failingOnceProvider struct {
	*testssh.SSHProvider
	refused bool
}

func (p *failingOnceProvider) SwitchClient(ctx gocontext.Context, sess *session.Session, keys []session.AgentPrivateKey) (libcon.SSHClient, error) {
	if !p.refused {
		p.refused = true
		return nil, errors.New("ssh: handshake failed")
	}

	return p.SSHProvider.SwitchClient(ctx, sess, keys)
}

func TestSwitchClientFallsBackToOperatorUser(t *testing.T) {
	hosts := []session.Host{{Host: "10.0.0.1", Name: "cluster-master-0"}}
	settings := session.NewSession(session.Input{User: "ubuntu", Port: "22", AvailableHosts: hosts})

	newProvider := func() *failingOnceProvider {
		return &failingOnceProvider{SSHProvider: testssh.NewSSHProvider(settings, true)}
	}

	t.Run("the converge user is retried as the operator", func(t *testing.T) {
		switcher := switcherWithConvergeUserNodes(t, operatorConnection(), "cluster-master-0")
		provider := newProvider()

		client, err := switcher.switchClientTo(t.Context(), provider, settings,
			sshCredentials{User: global.ConvergeUserName}, hosts)
		require.NoError(t, err)
		require.Equal(t, "ubuntu", client.Session().User)

		switches := provider.Switches()
		require.Len(t, switches, 1, "only the successful switch is recorded")
		require.Equal(t, "ubuntu", switches[0].Session.User)
		require.Equal(t, "become-pass", switches[0].Session.BecomePass)
	})

	t.Run("the operator user is not retried", func(t *testing.T) {
		switcher := switcherWithConvergeUserNodes(t, operatorConnection())
		provider := newProvider()

		_, err := switcher.switchClientTo(t.Context(), provider, settings,
			sshCredentials{User: "ubuntu"}, hosts)
		require.Error(t, err)
		require.Empty(t, provider.Switches())
	})
}
