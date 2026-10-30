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
	contextpkg "context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/testssh"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

type fakeSSHHostExtractor struct {
	addresses map[string]string
	internal  map[string]string
	errs      map[string]error
	calls     []string
}

func (e *fakeSSHHostExtractor) masterIPs(_ contextpkg.Context, params *sshIPExtractorParams) (infrastructure.OutputMasterIPs, error) {
	e.calls = append(e.calls, params.nodeName)
	ips := infrastructure.OutputMasterIPs{SSH: e.addresses[params.nodeName], Internal: e.internal[params.nodeName]}
	return ips, e.errs[params.nodeName]
}

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

// A state without addresses is a machine that is not there yet and is skipped either
// way. A failed extraction is skipped only when picking a host to connect to: the
// refresh must not act on an inventory with a master silently missing.
func TestMasterHostsFromStates(t *testing.T) {
	states := map[string][]byte{
		"cluster-master-2": []byte("state-2"),
		"cluster-master-0": []byte("state-0"),
		"cluster-master-1": []byte("state-1"),
	}
	newExtractor := func() *fakeSSHHostExtractor {
		return &fakeSSHHostExtractor{
			addresses: map[string]string{
				"cluster-master-2": "10.12.1.12",
				"cluster-master-0": "10.12.1.10",
			},
			internal: map[string]string{
				"cluster-master-2": "192.168.0.12",
				"cluster-master-0": "192.168.0.10",
			},
			errs: map[string]error{"cluster-master-1": fmt.Errorf("%w: output failed", errIPExtraction)},
		}
	}
	newSwitcher := func() *KubeClientSwitcher {
		return NewKubeClientSwitcher(NewContext(t.Context(), Params{}), nil, KubeClientSwitcherParams{})
	}

	t.Run("strict", func(t *testing.T) {
		_, err := newSwitcher().masterHostsFromStates(t.Context(), states, "", newExtractor(), true)

		require.ErrorContains(t, err, "output failed")
	})

	t.Run("lenient", func(t *testing.T) {
		hosts, err := newSwitcher().masterHostsFromStates(t.Context(), states, "", newExtractor(), false)

		require.NoError(t, err)
		require.Equal(t, []session.Host{
			{Host: "10.12.1.10", Name: "cluster-master-0"},
			{Host: "10.12.1.12", Name: "cluster-master-2"},
		}, hosts)
	})

	t.Run("lenient does not skip a failure that is not about the node", func(t *testing.T) {
		extractor := newExtractor()
		extractor.errs = map[string]error{"cluster-master-1": errors.New("cannot create output executor")}

		_, err := newSwitcher().masterHostsFromStates(t.Context(), states, "", extractor, false)

		require.ErrorContains(t, err, "cannot create output executor")
	})

	t.Run("no address yet", func(t *testing.T) {
		extractor := newExtractor()
		extractor.errs = nil

		hosts, err := newSwitcher().masterHostsFromStates(t.Context(), states, "", extractor, true)

		require.NoError(t, err)
		require.Equal(t, []session.Host{
			{Host: "10.12.1.10", Name: "cluster-master-0"},
			{Host: "10.12.1.12", Name: "cluster-master-2"},
		}, hosts)
	})

	t.Run("bastion takes the internal address", func(t *testing.T) {
		extractor := newExtractor()
		extractor.errs = nil

		hosts, err := newSwitcher().masterHostsFromStates(t.Context(), states, "bastion.example.com", extractor, true)

		require.NoError(t, err)
		require.Equal(t, []session.Host{
			{Host: "192.168.0.10", Name: "cluster-master-0"},
			{Host: "192.168.0.12", Name: "cluster-master-2"},
		}, hosts)
	})
}

// A refresh receives all master states, but usually only one has changed. Unchanged
// states should be read once, while a failed read must be retried.
func TestMasterIPsAreReadOncePerState(t *testing.T) {
	master0 := session.Host{Host: "10.12.1.10", Name: "cluster-master-0"}
	master1 := session.Host{Host: "10.12.1.11", Name: "cluster-master-1"}
	states := map[string][]byte{master0.Name: []byte("state-0"), master1.Name: []byte("state-1")}
	newExtractor := func() *fakeSSHHostExtractor {
		return &fakeSSHHostExtractor{
			addresses: map[string]string{master0.Name: master0.Host, master1.Name: master1.Host},
			internal:  map[string]string{master0.Name: "192.168.0.10", master1.Name: "192.168.0.11"},
		}
	}
	newSwitcher := func() *KubeClientSwitcher {
		return NewKubeClientSwitcher(NewContext(t.Context(), Params{}), nil, KubeClientSwitcherParams{})
	}

	t.Run("unchanged state is not read again", func(t *testing.T) {
		switcher, extractor := newSwitcher(), newExtractor()

		for range 3 {
			hosts, err := switcher.masterHostsFromStates(t.Context(), states, "", extractor, true)
			require.NoError(t, err)
			require.Equal(t, []session.Host{master0, master1}, hosts)
		}

		require.ElementsMatch(t, []string{master0.Name, master1.Name}, extractor.calls)
	})

	t.Run("changed state is read again", func(t *testing.T) {
		switcher, extractor := newSwitcher(), newExtractor()
		_, err := switcher.masterHostsFromStates(t.Context(), states, "", extractor, true)
		require.NoError(t, err)

		extractor.calls = nil
		extractor.addresses[master1.Name] = "10.12.1.21"
		changed := map[string][]byte{master0.Name: states[master0.Name], master1.Name: []byte("state-1-recreated")}

		hosts, err := switcher.masterHostsFromStates(t.Context(), changed, "", extractor, true)

		require.NoError(t, err)
		require.Equal(t, []string{master1.Name}, extractor.calls)
		require.Equal(t, []session.Host{master0, {Host: "10.12.1.21", Name: master1.Name}}, hosts)
	})

	t.Run("failure is not kept", func(t *testing.T) {
		switcher, extractor := newSwitcher(), newExtractor()
		extractor.errs = map[string]error{master1.Name: fmt.Errorf("%w: output failed", errIPExtraction)}

		_, err := switcher.masterHostsFromStates(t.Context(), states, "", extractor, true)
		require.Error(t, err)

		extractor.errs = nil
		extractor.calls = nil

		_, err = switcher.masterHostsFromStates(t.Context(), states, "", extractor, true)

		require.NoError(t, err)
		require.Contains(t, extractor.calls, master1.Name)
	})

	// The addresses an apply has read are those of the state it wrote: the refresh after
	// the apply runs no "output" for that master, and a bastion still picks from the pair.
	t.Run("addresses remembered from an apply", func(t *testing.T) {
		switcher, extractor := newSwitcher(), newExtractor()
		switcher.RememberMasterIPs(master0.Name, states[master0.Name], "10.12.1.30", "192.168.0.30")

		direct, err := switcher.masterHostsFromStates(t.Context(), map[string][]byte{master0.Name: states[master0.Name]}, "", extractor, true)
		require.NoError(t, err)

		viaBastion, err := switcher.masterHostsFromStates(t.Context(), map[string][]byte{master0.Name: states[master0.Name]}, "bastion.example.com", extractor, true)
		require.NoError(t, err)

		require.Empty(t, extractor.calls)
		require.Equal(t, []session.Host{{Host: "10.12.1.30", Name: master0.Name}}, direct)
		require.Equal(t, []session.Host{{Host: "192.168.0.30", Name: master0.Name}}, viaBastion)
	})
}

// The client is replaced only when the machine it is connected to is gone. Replacing it
// on every change of the inventory restarts the SSH connection and the lock for nothing,
// and each restart is one more chance to fail in the middle of a converge. A changed host
// set still makes the provider build a new kube client on the kept connection.
func TestRefreshSSHHosts(t *testing.T) {
	master0 := session.Host{Host: "10.12.1.10", Name: "cluster-master-0"}
	master1 := session.Host{Host: "10.12.1.11", Name: "cluster-master-1"}
	master2 := session.Host{Host: "10.12.1.12", Name: "cluster-master-2"}

	cases := []struct {
		name          string
		connectedTo   []session.Host
		addresses     map[string]string
		errs          map[string]error
		disableSwitch bool

		wantErr      bool
		wantSwitches int
		wantHost     string
		wantHosts    []session.Host
		wantCached   []session.Host
	}{
		{
			name:        "same inventory",
			connectedTo: []session.Host{master0, master1},
			addresses:   map[string]string{master0.Name: master0.Host, master1.Name: master1.Host},
			wantHost:    master0.Host,
			wantHosts:   []session.Host{master0, master1},
		},
		{
			name:        "current host stays, another one is gone",
			connectedTo: []session.Host{master0, master1},
			addresses:   map[string]string{master0.Name: master0.Host, master2.Name: master2.Host},
			wantHost:    master0.Host,
			wantHosts:   []session.Host{master0, master2},
		},
		{
			name:        "current host is not the first one",
			connectedTo: []session.Host{master1, master0},
			addresses:   map[string]string{master0.Name: master0.Host, master1.Name: master1.Host, master2.Name: master2.Host},
			wantHost:    master1.Host,
			wantHosts:   []session.Host{master1, master0, master2},
		},
		{
			name:         "current host is gone",
			connectedTo:  []session.Host{master0, master1},
			addresses:    map[string]string{master1.Name: master1.Host, master2.Name: master2.Host},
			wantSwitches: 1,
			wantHosts:    []session.Host{master1, master2},
		},
		{
			name:          "current host is not in the states and switching is disabled",
			connectedTo:   []session.Host{{Host: "master.example.com", Name: "master.example.com"}},
			addresses:     map[string]string{master1.Name: master1.Host, master2.Name: master2.Host},
			disableSwitch: true,
			wantHost:      "master.example.com",
			wantHosts:     []session.Host{{Host: "master.example.com", Name: "master.example.com"}, master1, master2},
			wantCached:    []session.Host{master1, master2},
		},
		{
			name:        "failed extraction changes nothing",
			connectedTo: []session.Host{master0, master1},
			addresses:   map[string]string{master1.Name: master1.Host},
			errs:        map[string]error{master2.Name: errors.New("output failed")},
			wantErr:     true,
			wantHost:    master0.Host,
			wantHosts:   []session.Host{master0, master1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateCache := cache.NewTestCache()
			ctx := NewContext(t.Context(), Params{Cache: stateCache})
			switcher := NewKubeClientSwitcher(ctx, nil, KubeClientSwitcherParams{DisableSwitch: tc.disableSwitch})

			sshProvider := testssh.NewSSHProvider(session.NewSession(session.Input{AvailableHosts: tc.connectedTo}), true)
			sshClient, err := sshProvider.Client(t.Context())
			require.NoError(t, err)

			states := make(map[string][]byte)
			for name := range tc.addresses {
				states[name] = []byte(name)
			}
			for name := range tc.errs {
				states[name] = []byte(name)
			}

			hosts, err := switcher.refreshSSHHosts(t.Context(), sshProvider, sshClient, states, &fakeSSHHostExtractor{addresses: tc.addresses, errs: tc.errs})

			switches := sshProvider.Switches()
			require.Len(t, switches, tc.wantSwitches)

			cachedHosts, cacheErr := state.GetMasterHostsIPs(t.Context(), stateCache)
			require.NoError(t, cacheErr)

			if tc.wantErr {
				require.Error(t, err)
				require.Empty(t, cachedHosts, "a failed refresh must not write the cache")
			} else {
				wantCached := tc.wantHosts
				if tc.wantCached != nil {
					wantCached = tc.wantCached
				}

				require.NoError(t, err)
				require.ElementsMatch(t, wantCached, hosts)
				require.ElementsMatch(t, wantCached, cachedHosts)
			}

			if tc.wantSwitches > 0 {
				require.Equal(t, tc.wantHosts, switches[0].Session.AvailableHosts())
				return
			}

			require.Equal(t, tc.wantHost, sshClient.Session().Host(), "the session must name the host its connection is open to")
			require.Equal(t, tc.wantHosts, sshClient.Session().AvailableHosts())
		})
	}
}

// Moving the client keeps everything about the session but its hosts: the same user
// reaches the same kind of machine through the same bastion.
func TestRefreshSSHClientReconnectsWithNewInventory(t *testing.T) {
	initialSession := session.NewSession(session.Input{
		User:            "converger",
		Port:            "2222",
		BastionHost:     "bastion.example.com",
		BastionPort:     "2200",
		BastionUser:     "bastion-user",
		BastionPassword: "bastion-password",
		ExtraArgs:       "-o ConnectTimeout=10",
		BecomePass:      "sudo-password",
		AvailableHosts: []session.Host{
			{Host: "10.12.1.10", Name: "cluster-master-0"},
			{Host: "10.12.1.11", Name: "cluster-master-1"},
		},
	})
	privateKeys := []session.AgentPrivateKey{{Key: "/tmp/id_rsa", Passphrase: "secret"}}
	sshProvider := testssh.NewSSHProvider(initialSession, true).WithInitPrivateKeys(privateKeys)
	sshClient, err := sshProvider.Client(t.Context())
	require.NoError(t, err)

	ctx := NewContext(t.Context(), Params{})
	switcher := NewKubeClientSwitcher(ctx, nil, KubeClientSwitcherParams{})
	desiredHosts := []session.Host{
		{Host: "10.12.1.11", Name: "cluster-master-1"},
		{Host: "10.12.1.12", Name: "cluster-master-2"},
	}

	require.NoError(t, switcher.refreshSSHClient(t.Context(), sshProvider, sshClient, desiredHosts))

	switches := sshProvider.Switches()
	require.Len(t, switches, 1)
	require.Equal(t, desiredHosts, switches[0].Session.AvailableHosts())
	require.Equal(t, privateKeys, switches[0].PrivateKeys)
	require.Equal(t, initialSession.User, switches[0].Session.User)
	require.Equal(t, initialSession.Port, switches[0].Session.Port)
	require.Equal(t, initialSession.BastionHost, switches[0].Session.BastionHost)
	require.Equal(t, initialSession.BastionPort, switches[0].Session.BastionPort)
	require.Equal(t, initialSession.BastionUser, switches[0].Session.BastionUser)
	require.Equal(t, initialSession.BastionPassword, switches[0].Session.BastionPassword)
	require.Equal(t, initialSession.ExtraArgs, switches[0].Session.ExtraArgs)
	require.Equal(t, initialSession.BecomePass, switches[0].Session.BecomePass)
}

type fakeKubeGetter struct {
	client *client.KubernetesClient
}

func (g fakeKubeGetter) KubeClientCtx(contextpkg.Context) (*client.KubernetesClient, error) {
	return g.client, nil
}

type failingSwitchProvider struct {
	*testssh.SSHProvider
}

func (failingSwitchProvider) SwitchClient(contextpkg.Context, *session.Session, []session.AgentPrivateKey) (libcon.SSHClient, error) {
	return nil, errors.New("dial failed")
}

// A failed switch leaves the run with no working client and no renewed lock, and the
// update loop stops on the sentinel. The lease stays where it was: deleting it would let
// an auto-converger take the cluster in the middle of the converge.
func TestSwitchSSHClientFailureKeepsPausedLease(t *testing.T) {
	kubeClient := client.NewFakeKubernetesClient()
	runner := lock.NewInLockRunner(t.Context(), fakeKubeGetter{client: kubeClient}, "converge-test", "")
	switcher := NewKubeClientSwitcher(NewContext(t.Context(), Params{}), runner, KubeClientSwitcherParams{})
	sess := session.NewSession(session.Input{AvailableHosts: []session.Host{{Host: "10.12.1.10", Name: "cluster-master-0"}}})

	require.NoError(t, runner.Run(t.Context(), func() error {
		err := switcher.switchSSHClient(t.Context(), failingSwitchProvider{testssh.NewSSHProvider(sess, true)}, sess, nil)
		require.ErrorIs(t, err, ErrSSHClientSwitchFailed)

		config := lock.GetLockLeaseConfig(t.Context(), "", "")
		_, err = kubeClient.CoordinationV1().Leases(config.Namespace).Get(t.Context(), config.Name, metav1.GetOptions{})
		require.NoError(t, err, "a failed switch must leave the paused lease in place")

		return nil
	}))
}

func TestSetHostsKeepingCurrent(t *testing.T) {
	master0 := session.Host{Host: "10.12.1.10", Name: "cluster-master-0"}
	master1 := session.Host{Host: "10.12.1.11", Name: "cluster-master-1"}
	master2 := session.Host{Host: "10.12.1.12", Name: "cluster-master-2"}

	cases := []struct {
		name      string
		current   session.Host
		hosts     []session.Host
		wantKept  bool
		wantHosts []session.Host
	}{
		{name: "first", current: master0, hosts: []session.Host{master0, master1, master2}, wantKept: true, wantHosts: []session.Host{master0, master1, master2}},
		{name: "middle", current: master1, hosts: []session.Host{master0, master1, master2}, wantKept: true, wantHosts: []session.Host{master1, master0, master2}},
		{name: "last", current: master2, hosts: []session.Host{master0, master1, master2}, wantKept: true, wantHosts: []session.Host{master2, master0, master1}},
		{name: "absent", current: master2, hosts: []session.Host{master0, master1}, wantHosts: []session.Host{master2}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess := session.NewSession(session.Input{AvailableHosts: []session.Host{tc.current}})
			hosts := append([]session.Host(nil), tc.hosts...)

			require.Equal(t, tc.wantKept, setHostsKeepingCurrent(sess, hosts))
			require.Equal(t, tc.current.Host, sess.Host())
			require.Equal(t, tc.wantHosts, sess.AvailableHosts())
			require.Equal(t, tc.hosts, hosts, "the caller's slice must not be reordered")
		})
	}
}

func TestSaveMasterHostsReplacesStaleCache(t *testing.T) {
	stateCache := cache.NewTestCache()
	state.SaveMasterHostsToCache(t.Context(), stateCache, map[string]string{
		"cluster-master-0": "10.12.1.10",
		"cluster-master-1": "10.12.1.11",
	})

	ctx := NewContext(t.Context(), Params{Cache: stateCache})
	switcher := NewKubeClientSwitcher(ctx, nil, KubeClientSwitcherParams{})
	switcher.saveMasterHosts(t.Context(), []session.Host{
		{Host: "10.12.1.12", Name: "cluster-master-2"},
	})

	hosts, err := state.GetMasterHostsIPs(t.Context(), stateCache)
	require.NoError(t, err)
	require.Equal(t, []session.Host{{Host: "10.12.1.12", Name: "cluster-master-2"}}, hosts)
}

func TestRefreshMasterHostsFromStatesInCommander(t *testing.T) {
	master0 := session.Host{Host: "10.12.1.10", Name: "cluster-master-0"}
	master2 := session.Host{Host: "10.12.1.12", Name: "cluster-master-2"}
	states := map[string][]byte{
		master0.Name: []byte("state-0"),
		master2.Name: []byte("state-2"),
	}

	stateCache := cache.NewTestCache()
	state.SaveMasterHostsToCache(t.Context(), stateCache, map[string]string{
		"cluster-master-1": "10.12.1.11",
	})

	ctx := NewCommanderContext(t.Context(), Params{Cache: stateCache}, &commander.CommanderModeParams{})
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	switcher := NewKubeClientSwitcher(ctx, nil, KubeClientSwitcherParams{TmpDir: filepath.Join(blocker, "tmp")})
	// Preload the values obtained by apply so the test exercises the Commander branch
	// without preparing a temporary state or invoking Terraform/OpenTofu.
	switcher.RememberMasterIPs(master0.Name, states[master0.Name], master0.Host, "192.168.0.10")
	switcher.RememberMasterIPs(master2.Name, states[master2.Name], master2.Host, "192.168.0.12")

	hosts, err := switcher.RefreshMasterHostsFromStates(t.Context(), states)
	require.NoError(t, err)
	require.Equal(t, []session.Host{master0, master2}, hosts)

	cachedHosts, err := state.GetMasterHostsIPs(t.Context(), stateCache)
	require.NoError(t, err)
	require.ElementsMatch(t, []session.Host{master0, master2}, cachedHosts)
}

func TestRefreshMasterHostsFromStatesInCommanderKeepsCacheOnFailure(t *testing.T) {
	cases := map[string]func(*testing.T) string{
		"extractor preparation": func(t *testing.T) string {
			blocker := filepath.Join(t.TempDir(), "file")
			require.NoError(t, os.WriteFile(blocker, nil, 0o600))
			return filepath.Join(blocker, "tmp")
		},
		// A valid temp directory lets extraction start; the deliberately incomplete
		// Commander context then makes provider preparation fail.
		"IP extraction": func(t *testing.T) string { return t.TempDir() },
	}

	for name, tmpDir := range cases {
		t.Run(name, func(t *testing.T) {
			stateCache := cache.NewTestCache()
			staleHost := session.Host{Host: "10.12.1.10", Name: "cluster-master-0"}
			state.SaveMasterHostsToCache(t.Context(), stateCache, map[string]string{
				staleHost.Name: staleHost.Host,
			})

			ctx := NewCommanderContext(t.Context(), Params{Cache: stateCache}, &commander.CommanderModeParams{})
			switcher := NewKubeClientSwitcher(ctx, nil, KubeClientSwitcherParams{TmpDir: tmpDir(t)})

			hosts, err := switcher.RefreshMasterHostsFromStates(t.Context(), map[string][]byte{
				staleHost.Name: []byte("state-0"),
			})
			require.NoError(t, err)
			require.Nil(t, hosts)

			cachedHosts, err := state.GetMasterHostsIPs(t.Context(), stateCache)
			require.NoError(t, err)
			require.Equal(t, []session.Host{staleHost}, cachedHosts)
		})
	}
}

// converge-migration in the auto-converger pod runs with in-cluster credentials and no
// SSH keys, so it has either no provider initializer or one without hosts. The refresh
// has no SSH inventory to rebuild there, and it must not fail the master update.
func TestRefreshMasterHostsFromStatesWithoutSSHHosts(t *testing.T) {
	cases := map[string]*providerinitializer.SSHProviderInitializer{
		"no initializer":        nil,
		"initializer, no hosts": providerinitializer.NewSSHProviderInitializer(nil, nil),
	}

	for name, initializer := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := NewContext(t.Context(), Params{SSHProviderInitializer: initializer})
			switcher := NewKubeClientSwitcher(ctx, nil, KubeClientSwitcherParams{DisableSwitch: true})

			hosts, err := switcher.RefreshMasterHostsFromStates(t.Context(), map[string][]byte{"cluster-master-0": []byte("state")})
			require.NoError(t, err)
			require.Nil(t, hosts)
		})
	}
}
