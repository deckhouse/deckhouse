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
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/testssh"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
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

// GetState hands out a copy, as the real store does by unmarshalling afresh: only SetState
// persists, so a test can tell a written-back state from a mutated one.
func (s *fakeStateStore) GetState(*Context) (*State, error) {
	copied := *s.state
	copied.ConvergeUserNodes = slices.Clone(s.state.ConvergeUserNodes)

	return &copied, nil
}

func (s *fakeStateStore) SetState(_ *Context, st *State) error {
	s.state = st

	return nil
}

func (s *fakeStateStore) Delete(*Context) error { return nil }

// switcherWithConvergeUserNodes builds a switcher whose converge state names the given
// masters as built by this converge, with "ubuntu" as the user dhctl was started with.
func switcherWithConvergeUserNodes(t *testing.T, connection *sshconfig.ConnectionConfig, nodes ...string) *KubeClientSwitcher {
	t.Helper()

	initializer := providerinitializer.NewSSHProviderInitializer(
		settings.NewBaseProviders(settings.ProviderParams{}),
		connection,
	)

	ctx := NewContext(t.Context(), Params{SSHProviderInitializer: initializer, Cache: cache.NewTestCache()})
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

// switchRecorder records the user of every switch attempt, and refuses the first attempt
// the way a master built without the converge user does: either at the switch, as the
// default backend does when it dials, or at the first command, as the legacy one does.
type switchRecorder struct {
	*testssh.SSHProvider
	users              []string
	refuseSwitch       bool
	refuseFirstCommand bool
}

func (p *switchRecorder) SwitchClient(ctx gocontext.Context, sess *session.Session, keys []session.AgentPrivateKey) (libcon.SSHClient, error) {
	p.users = append(p.users, sess.User)

	if p.refuseSwitch && len(p.users) == 1 {
		return nil, errors.New("ssh: handshake failed")
	}

	return p.SSHProvider.SwitchClient(ctx, sess, keys)
}

func TestSwitchClientFallsBackToOperatorUser(t *testing.T) {
	const host = "10.0.0.1"

	hosts := []session.Host{{Host: host, Name: "cluster-master-0"}}
	settings := session.NewSession(session.Input{User: "ubuntu", Port: "22", AvailableHosts: hosts})

	newRecorder := func() *switchRecorder {
		recorder := &switchRecorder{SSHProvider: testssh.NewSSHProvider(settings, true)}

		recorder.AddCommandProvider(host, func(_ testssh.Bastion, _ string, _ ...string) *testssh.Command {
			if recorder.refuseFirstCommand && len(recorder.users) == 1 {
				return testssh.NewCommand(nil).WithErr(errors.New("Permission denied (publickey)"))
			}

			return testssh.NewCommand(nil)
		})

		return recorder
	}

	for name, refusal := range map[string]func(*switchRecorder){
		"the switch is refused":             func(r *switchRecorder) { r.refuseSwitch = true },
		"only the first command is refused": func(r *switchRecorder) { r.refuseFirstCommand = true },
	} {
		t.Run(name+", so the converge user is retried as the operator", func(t *testing.T) {
			switcher := switcherWithConvergeUserNodes(t, operatorConnection(), "cluster-master-0")
			recorder := newRecorder()
			refusal(recorder)

			client, err := switcher.switchClientTo(t.Context(), recorder, settings,
				sshCredentials{User: global.ConvergeUserName}, hosts)
			require.NoError(t, err)

			require.Equal(t, []string{global.ConvergeUserName, "ubuntu"}, recorder.users,
				"the converge user is tried first, the operator only after it fails")
			require.Equal(t, "ubuntu", client.Session().User)

			switches := recorder.Switches()
			require.NotEmpty(t, switches)

			live := switches[len(switches)-1].Session
			require.Equal(t, "ubuntu", live.User)
			require.Equal(t, "become-pass", live.BecomePass, "the operator's sudo password comes along")
		})
	}

	t.Run("the operator user is not retried", func(t *testing.T) {
		switcher := switcherWithConvergeUserNodes(t, operatorConnection())
		recorder := newRecorder()
		recorder.refuseSwitch = true

		_, err := switcher.switchClientTo(t.Context(), recorder, settings,
			sshCredentials{User: "ubuntu"}, hosts)
		require.Error(t, err)
		require.Equal(t, []string{"ubuntu"}, recorder.users)
	})
}

func TestHostsOfOneGeneration(t *testing.T) {
	older := session.Host{Host: "10.0.0.1", Name: "cluster-master-0"}
	rebuilt := session.Host{Host: "10.0.0.2", Name: "cluster-master-1"}

	t.Run("the operator generation is preferred while it has a host", func(t *testing.T) {
		switcher := switcherWithConvergeUserNodes(t, operatorConnection(), "cluster-master-1")

		kept, err := switcher.hostsOfOneGeneration([]session.Host{older, rebuilt})
		require.NoError(t, err)
		require.Equal(t, []session.Host{older}, kept)
	})

	t.Run("with none left the converge generation is taken", func(t *testing.T) {
		switcher := switcherWithConvergeUserNodes(t, operatorConnection(), "cluster-master-0", "cluster-master-1")

		kept, err := switcher.hostsOfOneGeneration([]session.Host{older, rebuilt})
		require.NoError(t, err)
		require.Equal(t, []session.Host{older, rebuilt}, kept)
	})
}

// standaloneRecorder notes who connected where: a keyed standalone client keeps its
// session to itself, and the account the cleanup logs in as is half of what it does.
type standaloneRecorder struct {
	*testssh.SSHProvider
	usersByHost map[string]string
	stoppedKeys []string
	order       []string
}

func (p *standaloneRecorder) StandaloneClientFor(ctx gocontext.Context, key string, sess *session.Session, keys []session.AgentPrivateKey, opts ...libcon.StandaloneClientOpt) (libcon.SSHClient, error) {
	p.usersByHost[sess.Host()] = sess.User

	return p.SSHProvider.StandaloneClientFor(ctx, key, sess, keys, opts...)
}

func (p *standaloneRecorder) StopStandaloneClientFor(ctx gocontext.Context, key string) {
	p.stoppedKeys = append(p.stoppedKeys, key)

	p.SSHProvider.StopStandaloneClientFor(ctx, key)
}

func TestCleanupConvergeUser(t *testing.T) {
	const (
		rebuilt     = "10.0.0.1"
		preExisting = "10.0.0.2"
	)

	userdel := []string{"userdel", "-f", "-r", global.ConvergeUserName}

	// A master this converge built is kept out of the session, which carries one
	// generation of users, and its address is in the hosts cache alone.
	cachedHosts := map[string]string{"cluster-master-0": rebuilt, "cluster-master-1": preExisting}

	newRecorder := func(failing string) (*standaloneRecorder, map[string][]string) {
		ran := make(map[string][]string)

		base := session.NewSession(session.Input{
			User:           "ubuntu",
			Port:           "22",
			AvailableHosts: []session.Host{{Host: preExisting, Name: "cluster-master-1"}},
		})

		recorder := &standaloneRecorder{
			SSHProvider: testssh.NewSSHProvider(base, true),
			usersByHost: make(map[string]string),
		}

		for _, host := range []string{rebuilt, preExisting} {
			recorder.AddCommandProvider(host, func(_ testssh.Bastion, name string, args ...string) *testssh.Command {
				cmd := testssh.NewCommand(nil)

				if host == failing {
					cmd = cmd.
						WithStdErr([]byte("userdel: user d8-converge is currently used by process 1")).
						WithErr(errors.New("exit status 8"))
				}

				return cmd.WithRun(func() {
					ran[host] = append([]string{name}, args...)
					recorder.order = append(recorder.order, host)
				})
			})
		}

		return recorder, ran
	}

	newSwitcher := func(t *testing.T, nodes ...string) *KubeClientSwitcher {
		t.Helper()

		switcher := switcherWithConvergeUserNodes(t, operatorConnection(), nodes...)
		require.NoError(t, dstate.SaveMasterHosts(t.Context(), switcher.ctx.StateCache(), cachedHosts))

		return switcher
	}

	t.Run("runs on the nodes of this converge only", func(t *testing.T) {
		switcher := newSwitcher(t, "cluster-master-0")
		recorder, ran := newRecorder("")

		require.NoError(t, switcher.removeConvergeUser(t.Context(), recorder))

		// -f is the point: the ssh session running userdel belongs to the very account it
		// removes, and without it userdel refuses to remove a user that owns a process.
		require.Equal(t, userdel, ran[rebuilt])
		require.NotContains(t, ran, preExisting, "a master this converge did not build never got the user")

		require.Equal(t, global.ConvergeUserName, recorder.usersByHost[rebuilt])
		require.Len(t, recorder.stoppedKeys, 1, "the connection outlives the account it logs in with")

		state, err := switcher.ctx.ConvergeState()
		require.NoError(t, err)
		require.Empty(t, state.ConvergeUserNodes)
	})

	t.Run("a converge that built no master touches nothing", func(t *testing.T) {
		switcher := newSwitcher(t)
		recorder, ran := newRecorder("")

		require.NoError(t, switcher.removeConvergeUser(t.Context(), recorder))
		require.Empty(t, ran)
	})

	t.Run("one failing host does not stop the rest", func(t *testing.T) {
		switcher := newSwitcher(t, "cluster-master-0", "cluster-master-1")
		recorder, ran := newRecorder(rebuilt)

		err := switcher.removeConvergeUser(t.Context(), recorder)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cluster-master-0")
		require.Contains(t, err.Error(), "currently used by process")
		require.NotContains(t, err.Error(), "cluster-master-1")

		require.Equal(t, userdel, ran[preExisting], "the host after the failing one is cleaned up too")
		require.Len(t, ran, 2)
	})

	t.Run("a cleaned node leaves the state right away", func(t *testing.T) {
		switcher := newSwitcher(t, "cluster-master-0", "cluster-master-1")
		recorder, _ := newRecorder(preExisting)

		require.Error(t, switcher.removeConvergeUser(t.Context(), recorder))

		// Left listed, the cleaned node would send the next converge to log in as an
		// account that is already gone, and no converge of this cluster could finish.
		state, err := switcher.ctx.ConvergeState()
		require.NoError(t, err)
		require.Equal(t, []string{"cluster-master-1"}, state.ConvergeUserNodes)
	})

	t.Run("the node the live client is on is cleaned last", func(t *testing.T) {
		// The session is connected to the pre-existing master, and this converge rebuilt
		// it too: taking its account first would break the kube tunnel riding that session.
		switcher := newSwitcher(t, "cluster-master-1", "cluster-master-0")
		recorder, _ := newRecorder("")

		require.NoError(t, switcher.removeConvergeUser(t.Context(), recorder))
		require.Equal(t, []string{rebuilt, preExisting}, recorder.order)
	})

	t.Run("a node with no known address is reported, the rest are cleaned", func(t *testing.T) {
		switcher := newSwitcher(t, "cluster-master-0", "cluster-master-9")
		recorder, ran := newRecorder("")

		err := switcher.removeConvergeUser(t.Context(), recorder)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cluster-master-9")

		require.Equal(t, userdel, ran[rebuilt])
	})
}
