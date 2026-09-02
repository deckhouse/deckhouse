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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"

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
// master sends the next converge looking for a NodeUser and bashible on a machine
// that runs neither.
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
