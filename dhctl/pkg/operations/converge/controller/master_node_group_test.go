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

package controller

import (
	gocontext "context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// Without SSH hosts the hook cannot be built. Reporting that instead of a nil hook
// is what keeps the runner from silently falling back to DummyHook and recreating a
// master VM that still holds its etcd membership and its Node object.
func TestNewHookForUpdatePipelineFailsWithoutSSHHosts(t *testing.T) {
	noHosts := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{}}
	convergeCtx := context.NewContext(t.Context(), context.Params{
		SSHProviderInitializer: providerinitializer.NewSSHProviderInitializer(
			settings.NewBaseProviders(settings.ProviderParams{}),
			noHosts,
		),
	})

	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil),
		false,
	)

	hook, err := controller.newHookForUpdatePipeline(convergeCtx, "cluster-master-0")

	require.Error(t, err)
	require.Nil(t, hook)
}

// A run started with kube flags and unreadable SSH keys carries no provider initializer
// at all. The host map then stays empty, and an empty map is how a single-master cluster
// is recognised: a destructive plan would scale 1→3→1 and queue the two healthy masters
// for destruction.
func TestNewHookForUpdatePipelineFailsWithoutSSHConfiguration(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{})

	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{
			State: map[string][]byte{
				"cluster-master-0": nil,
				"cluster-master-1": nil,
				"cluster-master-2": nil,
			},
		}, nil, nil),
		false,
	)

	hook, err := controller.newHookForUpdatePipeline(convergeCtx, "cluster-master-0")

	require.Error(t, err)
	require.Nil(t, hook)
}

// The readiness gate asks one question per surviving master, and an answer of "no"
// skips the check instead of failing it. With no terminal that answer defaulted to no,
// so a master was retired with the others' readiness never looked at.
func TestUpdatePipelineChecksSurvivingMastersWithoutTerminal(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{
		KubeProvider: unreachableKubeProvider{},
	})

	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
		State: map[string][]byte{"cluster-master-0": nil, "cluster-master-1": nil},
	}, nil, nil)
	nodeGroup.immutable = true

	hook, err := NewMasterNodeGroupController(nodeGroup, false).
		newHookForUpdatePipeline(convergeCtx, "cluster-master-0")
	require.NoError(t, err)

	pipeline, ok := hook.(*controlplane.HookForUpdatePipeline)
	require.True(t, ok, "the update pipeline hook carries the readiness gate")

	// Cancelled so the readiness loop gives up on the unreachable cluster at once.
	checkCtx, cancel := gocontext.WithCancel(t.Context())
	cancel()

	require.Error(t, pipeline.IsAllNodesReady(checkCtx),
		"cluster-master-1 was reported ready without a single check")
}

// An immutable master is never registered as an SSH host, so the provider lookup
// fails on a master-hosts cache nobody wrote. Reporting that aborted every converge
// of an immutable master before it could render the node's payload.
func TestNewHookForUpdatePipelineNeedsNoSSHForImmutableMaster(t *testing.T) {
	noHosts := &sshconfig.ConnectionConfig{Config: &sshconfig.Config{}}
	convergeCtx := context.NewContext(t.Context(), context.Params{
		SSHProviderInitializer: providerinitializer.NewSSHProviderInitializer(
			settings.NewBaseProviders(settings.ProviderParams{}),
			noHosts,
		),
	})

	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
		State: map[string][]byte{"cluster-master-0": nil, "cluster-master-1": nil},
	}, nil, nil)
	nodeGroup.immutable = true

	hook, err := NewMasterNodeGroupController(nodeGroup, false).
		newHookForUpdatePipeline(convergeCtx, "cluster-master-0")

	require.NoError(t, err)
	require.NotNil(t, hook)
}

// The cached address is what makes Context.SSHless() false. Caching an immutable
// master sends the next converge looking for an sshd that the machine does not run.
func TestMasterHostsCacheSkipsImmutableNodes(t *testing.T) {
	newHost := []session.Host{{Host: "10.12.1.10", Name: "cluster-master-0"}}

	t.Run("immutable", func(t *testing.T) {
		stateCache := cache.NewTestCache()
		convergeCtx := context.NewContext(t.Context(), context.Params{Cache: stateCache})

		nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil)
		nodeGroup.immutable = true

		NewMasterNodeGroupController(nodeGroup, false).addNewNodesToCache(convergeCtx, newHost)

		hosts, err := state.GetMasterHostsIPs(t.Context(), stateCache)
		require.NoError(t, err)
		require.Empty(t, hosts)
	})

	t.Run("bashible", func(t *testing.T) {
		stateCache := cache.NewTestCache()
		convergeCtx := context.NewContext(t.Context(), context.Params{Cache: stateCache})

		nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil)

		NewMasterNodeGroupController(nodeGroup, false).addNewNodesToCache(convergeCtx, newHost)

		hosts, err := state.GetMasterHostsIPs(t.Context(), stateCache)
		require.NoError(t, err)
		require.Equal(t, newHost, hosts)
	})
}

// The map of the other masters is what tells a multi-master cluster from a single-master
// one: an empty one sends every destructive plan into the 1→3→1 scale dance. Immutable
// masters have no SSH addresses to fill it with, so it is filled by name.
func TestPopulateNodeToHostListsImmutableMastersByName(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{})

	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
		State: map[string][]byte{
			"cluster-master-0": nil,
			"cluster-master-1": nil,
			"cluster-master-2": nil,
		},
	}, nil, nil)
	nodeGroup.immutable = true

	controller := NewMasterNodeGroupController(nodeGroup, false)

	require.NoError(t, controller.populateNodeToHost(convergeCtx))
	require.Equal(t,
		map[string]string{"cluster-master-0": "", "cluster-master-1": "", "cluster-master-2": ""},
		controller.nodeToHost)
}

// CheckSSHHosts asks to confirm the node-to-host mapping on every run, destructive plan
// or not. Answering "no" for a caller that has no terminal cost the whole hook, so the
// master was recreated with no guard; the answer is now yes plus a log line. Tests run
// without a TTY, which is exactly the case under test.
func TestHostsMappingConfirmedWithoutTerminal(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{})

	require.True(t, confirmOrProceed(convergeCtx)("master-0 -> 10.12.1.10"))
}

// The converge user reaches a master over sshd, so it belongs in the payload of a
// mutable master and nowhere else.
func TestMasterCloudConfig(t *testing.T) {
	const pub = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBjoNkgxOUgHOBR6kRCRXyO+XEcnsQ8+A6FHPExg4nMQ test@example"

	base := base64.StdEncoding.EncodeToString([]byte(`#cloud-config
write_files:
- path: '/var/lib/bashible/bootstrap.sh'
  content: |
    #!/bin/bash
runcmd:
- /var/lib/bashible/bootstrap.sh
`))

	meta := &config.MetaConfig{
		ProviderName:          "openstack",
		ProviderClusterConfig: map[string]json.RawMessage{"sshPublicKey": json.RawMessage(strconv.Quote(pub))},
	}

	convergeUser := func(t *testing.T, cloudConfigB64 string) map[string]any {
		t.Helper()

		raw, err := base64.StdEncoding.DecodeString(cloudConfigB64)
		require.NoError(t, err)

		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(raw, &doc))

		users, ok := doc["users"].([]any)
		require.True(t, ok)

		user, ok := users[len(users)-1].(map[string]any)
		require.True(t, ok)
		require.Equal(t, global.ConvergeUserName, user["name"])

		require.Equal(t, []any{"/var/lib/bashible/bootstrap.sh"}, doc["runcmd"],
			"the bashible payload must survive the render")

		return user
	}

	t.Run("mutable master gets the user", func(t *testing.T) {
		got, err := masterCloudConfig(t.Context(), meta, nil, base, false)
		require.NoError(t, err)

		require.Equal(t, []any{pub}, convergeUser(t, got)["ssh_authorized_keys"])
	})

	// The keys dhctl logs in with are authorized too: the operator may well reach the
	// cluster with a key that is not the one in the provider configuration.
	t.Run("the keys dhctl logs in with are authorized", func(t *testing.T) {
		keyPath := writeTestPrivateKey(t, "")

		got, err := masterCloudConfig(t.Context(), meta,
			[]sshconfig.AgentPrivateKey{{Key: keyPath, IsPath: true}}, base, false)
		require.NoError(t, err)

		require.Equal(t,
			[]any{pub, testPublicKey(t, keyPath, "")},
			convergeUser(t, got)["ssh_authorized_keys"])
	})

	// useradd -e disables the account at 00:00 on the date it is given, so one day would
	// leave a converge started at 23:50 ten minutes. Two days are at least 24 hours.
	t.Run("the user outlives a converge started at any hour", func(t *testing.T) {
		expiry := func() string { return time.Now().UTC().Add(48 * time.Hour).Format(time.DateOnly) }

		before := expiry()
		got, err := masterCloudConfig(t.Context(), meta, nil, base, false)
		require.NoError(t, err)

		expiredate, ok := convergeUser(t, got)["expiredate"].(string)
		require.True(t, ok)

		disabledAt, err := time.ParseInLocation(time.DateOnly, expiredate, time.UTC)
		require.NoError(t, err)
		require.GreaterOrEqual(t, time.Until(disabledAt), 24*time.Hour,
			"the account is disabled at 00:00 on %s, sooner than a master converge can finish", expiredate)

		require.Contains(t, []string{before, expiry()}, expiredate)
	})

	// An immutable master answers no sshd, and a commander converge has no SSH at
	// all: their payload must come back byte-identical.
	t.Run("skipped payload is untouched", func(t *testing.T) {
		got, err := masterCloudConfig(t.Context(), meta, nil, base, true)
		require.NoError(t, err)
		require.Equal(t, base, got)
	})
}

// The keys must be the operator's, exactly as dhctl was started with them. The live SSH
// client is not a source: converge switches it to a user of its own halfway through, and
// the public half of that generated key would then land in a new master's authorized_keys.
func TestOperatorPrivateKeys(t *testing.T) {
	t.Run("come from the connection config", func(t *testing.T) {
		convergeCtx := context.NewContext(t.Context(), context.Params{
			SSHProviderInitializer: providerinitializer.NewSSHProviderInitializer(
				settings.NewBaseProviders(settings.ProviderParams{}),
				&sshconfig.ConnectionConfig{Config: &sshconfig.Config{
					PrivateKeys: []sshconfig.AgentPrivateKey{
						{Key: "/tmp/id_ed25519", Passphrase: "s3cret", IsPath: true},
					},
				}},
			),
		})

		// Handed over as they are: the public half is taken later, and a key given
		// inline needs its IsPath to survive the trip.
		require.Equal(t,
			[]sshconfig.AgentPrivateKey{{Key: "/tmp/id_ed25519", Passphrase: "s3cret", IsPath: true}},
			operatorPrivateKeys(convergeCtx))
	})

	t.Run("a converge with no ssh configuration has none", func(t *testing.T) {
		require.Empty(t, operatorPrivateKeys(context.NewContext(t.Context(), context.Params{})))
	})
}

// Only a mutable master is reached over SSH by this converge, and only it may carry the
// user. A worker never is, an immutable master answers no sshd, and a commander converge
// holds Kubernetes credentials of its own and connects to no node at all.
func TestConvergeUserSkipped(t *testing.T) {
	newController := func(name string, immutable bool) *NodeGroupController {
		controller := NewNodeGroupController(name, state.NodeGroupInfrastructureState{}, nil, nil)
		controller.immutable = immutable
		return controller
	}

	convergeCtx := context.NewContext(t.Context(), context.Params{})
	commanderCtx := context.NewCommanderContext(t.Context(), context.Params{},
		commander.NewCommanderModeParams([]byte("{}"), []byte("{}")))

	// Own Kubernetes credentials and no SSH host known: nothing will ever log in, and a
	// standing NOPASSWD sudo account on a control-plane node is not worth an expiry date.
	sshlessCtx := context.NewContext(t.Context(), context.Params{KubeOwnCredentials: true})
	require.True(t, sshlessCtx.SSHless())

	require.False(t, newController("master", false).convergeUserSkipped(convergeCtx))
	require.True(t, newController("worker", false).convergeUserSkipped(convergeCtx))
	require.True(t, newController("master", true).convergeUserSkipped(convergeCtx))
	require.True(t, newController("master", false).convergeUserSkipped(commanderCtx))
	require.True(t, newController("master", false).convergeUserSkipped(sshlessCtx))
}

// A master that answers no sshd carries no converge user, so listing it would send the
// next switch looking for that user on a machine that has none.
func TestRememberConvergeUserNodeSkipsImmutableMaster(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{})

	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil)
	nodeGroup.immutable = true

	controller := NewMasterNodeGroupController(nodeGroup, false)
	controller.convergeState = &context.State{}

	// The context carries no kube client: saving the state would not even get that far.
	require.NoError(t, controller.rememberConvergeUserNode(convergeCtx, "cluster-master-0"))
	require.Empty(t, controller.convergeState.ConvergeUserNodes)
}

// convergeUserSkipped is answered where the payload is rendered, and the record follows
// that answer. SSHless() turns false the moment the hosts cache is written, so asking a
// second time at record time claims an account no payload ever carried.
func TestRememberConvergeUserNodeFollowsTheRenderedPayload(t *testing.T) {
	// The converge rendered its payload while it knew no host — sshless, no account went
	// in — and by record time a host is known, so SSHless() is false.
	convergeCtx := context.NewContext(t.Context(), context.Params{
		KubeOwnCredentials: true,
		KubeProvider:       unreachableKubeProvider{},
		SSHProviderInitializer: providerinitializer.NewSSHProviderInitializer(
			settings.NewBaseProviders(settings.ProviderParams{}),
			&sshconfig.ConnectionConfig{
				Config: &sshconfig.Config{},
				Hosts:  []sshconfig.Host{{Host: "10.12.1.10"}},
			},
		),
	})
	require.False(t, convergeCtx.SSHless())

	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil), false)
	controller.convergeState = &context.State{}

	require.NoError(t, controller.rememberConvergeUserNode(convergeCtx, "cluster-master-0"))
	require.Empty(t, controller.convergeState.ConvergeUserNodes,
		"a payload rendered without the converge user must not be recorded as carrying it")
}

// The same master is recorded once: addNodes and updateNode both report, and a converge
// that scales 1→3→1 walks the same node twice.
func TestRememberConvergeUserNodeIsIdempotent(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{})

	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil), false)
	controller.convergeState = &context.State{ConvergeUserNodes: []string{"cluster-master-0"}}
	controller.cloudConfigHasConvergeUser = true

	require.NoError(t, controller.rememberConvergeUserNode(convergeCtx, "cluster-master-0"))
	require.Equal(t, []string{"cluster-master-0"}, controller.convergeState.ConvergeUserNodes)
}

// A name recorded without the expiry of the account it stands for is dropped by the very
// next converge: DeleteConvergeStateIfUserGone reads a missing expiry as long past.
func TestRememberConvergeUserNodeRecordsTheAccountExpiry(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{KubeProvider: unreachableKubeProvider{}})

	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil), false)
	controller.convergeState = &context.State{}
	controller.cloudConfigHasConvergeUser = true

	// The cluster is unreachable, so the save fails; what it was about to save is the point.
	require.Error(t, controller.rememberConvergeUserNode(convergeCtx, "cluster-master-0"))

	require.Equal(t, []string{"cluster-master-0"}, controller.convergeState.ConvergeUserNodes)
	require.True(t, controller.convergeState.ConvergeUserExpiry.After(time.Now().Add(24*time.Hour)),
		"the recorded expiry outlives no account: %s", controller.convergeState.ConvergeUserExpiry)
}

// The scale dance of a destructive single-master plan creates two masters with the
// converge user and deletes them again. Left in the state, they send the cleanup to
// machines that no longer exist, and every later consumer has to re-derive liveness.
func TestForgetConvergeUserNodes(t *testing.T) {
	newController := func(recorded ...string) *MasterNodeGroupController {
		controller := NewMasterNodeGroupController(
			NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil), false)
		controller.convergeState = &context.State{ConvergeUserNodes: recorded}
		return controller
	}

	t.Run("the deleted masters are dropped and the state is saved", func(t *testing.T) {
		// The cluster is unreachable, so the save fails immediately. That failure is the
		// proof that it was attempted: a prune kept only in memory is lost on restart.
		convergeCtx := context.NewContext(t.Context(), context.Params{KubeProvider: unreachableKubeProvider{}})

		controller := newController("cluster-master-0", "cluster-master-1", "cluster-master-2")

		err := controller.forgetConvergeUserNodes(convergeCtx, []string{"cluster-master-1", "cluster-master-2"})
		require.ErrorContains(t, err, "save converge state without the deleted nodes")
		require.Equal(t, []string{"cluster-master-0"}, controller.convergeState.ConvergeUserNodes)
	})

	// Nothing to drop must not cost a write: deleteNodes runs on every converge that
	// scales down, and most of them never created a master of their own.
	t.Run("a node nobody recorded is not saved", func(t *testing.T) {
		convergeCtx := context.NewContext(t.Context(), context.Params{KubeProvider: unreachableKubeProvider{}})

		controller := newController("cluster-master-0")

		require.NoError(t, controller.forgetConvergeUserNodes(convergeCtx, []string{"cluster-master-7"}))
		require.Equal(t, []string{"cluster-master-0"}, controller.convergeState.ConvergeUserNodes)
	})
}

// CheckSSHHosts counts the hosts it is given against the replica count. An address of
// --ssh-host carries no node name, so it survives the merge beside the named entry it
// resolves to: three masters then look like four hosts, every converge reports "too many"
// and the exemption that covers the 1->3->1 scale dance never matches.
func TestMasterHostsToCheck(t *testing.T) {
	names := []string{"cluster-master-0", "cluster-master-1", "cluster-master-2"}

	t.Run("the raw --ssh-host entry is dropped", func(t *testing.T) {
		hosts := []session.Host{
			{Host: "10.12.1.33", Name: "10.12.1.33"},
			{Host: "10.12.0.174", Name: "cluster-master-0"},
			{Host: "10.12.0.230", Name: "cluster-master-1"},
			{Host: "10.12.1.33", Name: "cluster-master-2"},
		}

		require.Equal(t, []session.Host{
			{Host: "10.12.0.174", Name: "cluster-master-0"},
			{Host: "10.12.0.230", Name: "cluster-master-1"},
			{Host: "10.12.1.33", Name: "cluster-master-2"},
		}, masterHostsToCheck(hosts, names),
			"one host per master, or CheckSSHHosts warns about a count nobody passed")
	})

	// The scale dance reaches a single replica while three hosts are still configured, and
	// CheckSSHHosts has an exemption for exactly that shape. A fourth entry misses it.
	t.Run("three hosts stay three during the scale dance", func(t *testing.T) {
		hosts := []session.Host{
			{Host: "10.12.1.33", Name: "10.12.1.33"},
			{Host: "10.12.0.174", Name: "cluster-master-0"},
			{Host: "10.12.0.230", Name: "cluster-master-1"},
			{Host: "10.12.1.33", Name: "cluster-master-2"},
		}

		require.Len(t, masterHostsToCheck(hosts, names), 3)
	})

	// Nothing names a node on the first converge of a cluster whose state carries no
	// address. Dropping every host there would report "no hosts passed" instead.
	t.Run("hosts nobody claims are kept when they are all there is", func(t *testing.T) {
		hosts := []session.Host{
			{Host: "10.12.1.33", Name: "10.12.1.33"},
			{Host: "10.12.0.174", Name: "10.12.0.174"},
		}

		require.Equal(t, hosts, masterHostsToCheck(hosts, names))
	})
}

// deleteRedundantNodes skips an excluded node, so that master lives on with the converge
// user on it. Counting it as deleted drops it from the record CleanupConvergeUser reads,
// and the account with its passwordless sudo stays until its expiry.
func TestNodesActuallyDeleted(t *testing.T) {
	nodes := []nodeToDeleteInfo{
		{name: "cluster-master-1", index: 1},
		{name: "cluster-master-2", index: 2},
	}

	t.Run("an excluded master is not forgotten", func(t *testing.T) {
		require.Equal(t, []string{"cluster-master-2"},
			nodesActuallyDeleted(nodes, map[string]bool{"cluster-master-1": true}))
	})

	t.Run("without exclusions every deleted master is forgotten", func(t *testing.T) {
		require.Equal(t, []string{"cluster-master-1", "cluster-master-2"},
			nodesActuallyDeleted(nodes, nil))
	})
}
