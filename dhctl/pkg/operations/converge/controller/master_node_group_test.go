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
	"errors"
	"fmt"
	"testing"
	"time"

	libcon "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/settings"
	sshconfig "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/config"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/session"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/testssh"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/utils"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/retry"
	klient "github.com/flant/kube-client/client"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
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

// A context with neither a switcher nor a provider initializer has nothing to find the
// master addresses with. The host map would stay empty, and an empty map is how a
// single-master cluster is recognised: a destructive plan would scale 1→3→1 and queue the
// two healthy masters for destruction.
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

	require.ErrorContains(t, err, "no ssh connection configuration")
	require.Nil(t, hook)
}

// converge-migration in the auto-converger pod runs with in-cluster credentials, no SSH
// keys and skipped checks. Its switcher has no hosts to refresh, yet every master is still
// listed, and the hook, which checks nothing over SSH, lets the migration go on.
func TestNewHookForUpdatePipelineInSSHlessMigration(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{KubeOwnCredentials: true})
	require.True(t, convergeCtx.SSHless())
	convergeCtx.SetClientSwitcher(context.NewKubeClientSwitcher(convergeCtx, nil, context.KubeClientSwitcherParams{DisableSwitch: true}))

	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{
			State: map[string][]byte{
				"cluster-master-0": []byte("state-0"),
				"cluster-master-1": []byte("state-1"),
				"cluster-master-2": []byte("state-2"),
			},
		}, nil, nil),
		true,
	)
	controller.convergeState = &context.State{}

	hook, err := controller.newHookForUpdatePipeline(convergeCtx, "cluster-master-0")

	require.NoError(t, err)
	require.NotNil(t, hook)
	require.Equal(t, map[string]string{"cluster-master-0": "", "cluster-master-1": "", "cluster-master-2": ""}, controller.nodeToHost)
}

// Commander reaches the cluster through its API server with no connection configuration
// and builds no SSH checker. A hosts-cache refresh that finds nothing only warns, and the
// master update goes on.
func TestNewHookForUpdatePipelineInCommanderNeedsNoSSHHosts(t *testing.T) {
	convergeCtx := context.NewCommanderContext(t.Context(), context.Params{
		ClientSwitcher: &recordingMasterClientSwitcher{},
	}, &commander.CommanderModeParams{})

	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{
			State: map[string][]byte{
				"cluster-master-0": []byte("state-0"),
				"cluster-master-1": []byte("state-1"),
				"cluster-master-2": []byte("state-2"),
			},
		}, nil, nil),
		false,
	)
	controller.convergeState = &context.State{}

	hook, err := controller.newHookForUpdatePipeline(convergeCtx, "cluster-master-0")

	require.NoError(t, err)
	require.NotNil(t, hook)
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

type recordingMasterClientSwitcher struct {
	refreshedStates map[string][]byte
	hosts           []session.Host
	err             error
	remembered      [][]string
}

func (s *recordingMasterClientSwitcher) SwitchToFirstMaster(gocontext.Context) error    { return nil }
func (s *recordingMasterClientSwitcher) SwitchToNotFirstMaster(gocontext.Context) error { return nil }
func (s *recordingMasterClientSwitcher) SwitchWhenDecreaseMastersIfNeed(gocontext.Context, string, []*context.NodeState) error {
	return nil
}
func (s *recordingMasterClientSwitcher) SwitchClientsToAnotherNodeIfNeed(gocontext.Context, string, string) error {
	return nil
}
func (s *recordingMasterClientSwitcher) RememberMasterIPs(nodeName string, state []byte, sshIP, internalIP string) {
	s.remembered = append(s.remembered, []string{nodeName, string(state), sshIP, internalIP})
}

func (s *recordingMasterClientSwitcher) RefreshMasterHostsFromStates(_ gocontext.Context, states map[string][]byte) ([]session.Host, error) {
	s.refreshedStates = states
	return s.hosts, s.err
}

// A restarted or resumed converge can have three master states but only the unnamed
// --ssh-host in its fresh session. Readiness mapping must refresh from the states first;
// filtering the unrefreshed session used to turn that into "SSH-hosts was not passed".
func TestPopulateNodeToHostRefreshesInventoryBeforeChecking(t *testing.T) {
	nodesState := map[string][]byte{
		"cluster-master-0": []byte("state-0"),
		"cluster-master-1": []byte("state-1"),
		"cluster-master-2": []byte("state-2"),
	}
	stateHosts := []session.Host{
		{Host: "10.12.1.10", Name: "cluster-master-0"},
		{Host: "10.12.1.11", Name: "cluster-master-1"},
		{Host: "10.12.1.12", Name: "cluster-master-2"},
	}
	switcher := &recordingMasterClientSwitcher{hosts: stateHosts}
	convergeCtx := context.NewContext(t.Context(), context.Params{ClientSwitcher: switcher})
	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{State: nodesState}, nil, nil),
		false,
	)
	controller.convergeState = &context.State{}

	err := controller.populateNodeToHost(convergeCtx)

	require.NoError(t, err)
	require.Equal(t, nodesState, switcher.refreshedStates)
	require.Equal(t, map[string]string{
		"cluster-master-0": "10.12.1.10",
		"cluster-master-1": "10.12.1.11",
		"cluster-master-2": "10.12.1.12",
	}, controller.nodeToHost)
}

// The masters to delete are out of the state from the start of a scale-down, but their
// machines run until deleteNodes destroys them, and the deletion can still be dismissed.
// The inventory keeps them, so the client is not moved off one of them early and the
// saved hosts still list them. The readiness checks cover only the masters that stay.
func TestPopulateNodeToHostKeepsMastersPendingDeletionInInventory(t *testing.T) {
	stateHosts := []session.Host{
		{Host: "10.12.1.10", Name: "cluster-master-0"},
		{Host: "10.12.1.11", Name: "cluster-master-1"},
		{Host: "10.12.1.12", Name: "cluster-master-2"},
	}
	switcher := &recordingMasterClientSwitcher{hosts: stateHosts}
	convergeCtx := context.NewContext(t.Context(), context.Params{ClientSwitcher: switcher})
	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
		State: map[string][]byte{"cluster-master-0": []byte("state-0")},
	}, nil, nil)
	nodeGroup.nodesToDelete = []nodeToDeleteInfo{
		{name: "cluster-master-2", index: 2, state: []byte("state-2")},
		{name: "cluster-master-1", index: 1, state: []byte("state-1")},
	}
	controller := NewMasterNodeGroupController(nodeGroup, false)
	controller.convergeState = &context.State{}

	require.NoError(t, controller.populateNodeToHost(convergeCtx))

	require.Equal(t, map[string][]byte{
		"cluster-master-0": []byte("state-0"),
		"cluster-master-1": []byte("state-1"),
		"cluster-master-2": []byte("state-2"),
	}, switcher.refreshedStates)
	require.Equal(t, map[string]string{"cluster-master-0": "10.12.1.10"}, controller.nodeToHost)
	require.Equal(t, map[string][]byte{"cluster-master-0": []byte("state-0")}, controller.state.State, "the state keeps only the masters that stay")

	// A master added again in the run, by the scale to three masters, has a new state.
	controller.state.State["cluster-master-1"] = []byte("state-1-new")
	require.Equal(t, []byte("state-1-new"), controller.inventoryStates()["cluster-master-1"])
}

// The masters to delete leave the inventory before any destroy starts: the destroy hook
// takes their control-plane role first, and a failover must not land on one of them. An
// excluded master is not destroyed and stays. With the inventory unchanged, nothing is
// destroyed.
func TestDeleteNodesDropsMastersFromInventoryBeforeDestroying(t *testing.T) {
	switcher := &recordingMasterClientSwitcher{err: errors.New("cannot extract IPs")}
	convergeCtx := context.NewContext(t.Context(), context.Params{ClientSwitcher: switcher})
	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
		State: map[string][]byte{"cluster-master-0": []byte("state-0")},
	}, nil, nil)
	nodeGroup.excludedNodes = map[string]bool{"cluster-master-1": true}
	nodesToDelete := []nodeToDeleteInfo{
		{name: "cluster-master-2", index: 2, state: []byte("state-2")},
		{name: "cluster-master-1", index: 1, state: []byte("state-1")},
	}
	nodeGroup.nodesToDelete = nodesToDelete
	controller := NewMasterNodeGroupController(nodeGroup, false)
	controller.desiredReplicas = 1

	require.ErrorIs(t, controller.deleteNodes(convergeCtx, nodesToDelete), errMasterHostsRefreshFailed)

	require.Equal(t, map[string][]byte{
		"cluster-master-0": []byte("state-0"),
		"cluster-master-1": []byte("state-1"),
	}, switcher.refreshedStates)
	require.Len(t, nodesToDelete, 2, "the list the destroys range over is left as it was")
	require.Equal(t, "cluster-master-2", nodesToDelete[0].name)
}

// Immutable masters answer no SSH, while bashible masters must refresh their
// inventory from the complete state set after infrastructure changes.
func TestRefreshMasterHostsSkipsImmutableNodes(t *testing.T) {
	nodesState := map[string][]byte{"cluster-master-0": []byte("state")}

	t.Run("immutable", func(t *testing.T) {
		switcher := &recordingMasterClientSwitcher{}
		convergeCtx := context.NewContext(t.Context(), context.Params{ClientSwitcher: switcher})

		nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{State: nodesState}, nil, nil)
		nodeGroup.immutable = true

		err := NewMasterNodeGroupController(nodeGroup, false).refreshMasterHostsFromStates(convergeCtx)

		require.NoError(t, err)
		require.Nil(t, switcher.refreshedStates)
	})

	t.Run("bashible", func(t *testing.T) {
		switcher := &recordingMasterClientSwitcher{
			hosts: []session.Host{{Host: "10.12.1.20", Name: "cluster-master-0"}},
		}
		convergeCtx := context.NewContext(t.Context(), context.Params{ClientSwitcher: switcher})

		nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{State: nodesState}, nil, nil)
		controller := NewMasterNodeGroupController(nodeGroup, false)
		controller.nodeToHost = map[string]string{"cluster-master-0": "10.12.1.10"}

		err := controller.refreshMasterHostsFromStates(convergeCtx)

		require.NoError(t, err)
		require.Equal(t, nodesState, switcher.refreshedStates)
		require.Equal(t, map[string]string{"cluster-master-0": "10.12.1.20"}, controller.nodeToHost)
	})
}

// The refresh after an apply finds the addresses the apply has just read, and runs no
// "output" for that master. Swapped addresses would put the internal IP into the SSH
// inventory of a cluster reached without a bastion.
func TestRememberMasterIPsFromApply(t *testing.T) {
	outputs := &infrastructure.PipelineOutputs{
		InfrastructureState: []byte("state"),
		MasterIPForSSH:      "10.12.1.10",
		NodeInternalIP:      "192.168.0.10",
	}

	cases := []struct {
		name      string
		immutable bool
		outputs   *infrastructure.PipelineOutputs
		want      [][]string
	}{
		{name: "bashible", outputs: outputs, want: [][]string{{"cluster-master-0", "state", "10.12.1.10", "192.168.0.10"}}},
		{name: "immutable", immutable: true, outputs: outputs},
		{name: "no outputs"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			switcher := &recordingMasterClientSwitcher{}
			nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil)
			nodeGroup.immutable = tc.immutable

			NewMasterNodeGroupController(nodeGroup, false).
				rememberMasterIPs(context.NewContext(t.Context(), context.Params{ClientSwitcher: switcher}), "cluster-master-0", tc.outputs)

			require.Equal(t, tc.want, switcher.remembered)
		})
	}
}

// The readiness checks reach the other masters by the addresses in nodeToHost. A master
// recreated with a new IP has to be checked at the new one, or the next master waits for
// a machine that is gone and fails the converge. The provider hands out the cached live
// readiness client whatever address it was opened to, so that client has to go as well.
// A master the inventory does not list has no address right now and must stay in the checks.
func TestSyncNodeToHost(t *testing.T) {
	newController := func(skipChecks bool) *MasterNodeGroupController {
		controller := NewMasterNodeGroupController(NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil), skipChecks)
		controller.nodeToHost = map[string]string{
			"cluster-master-0": "10.12.1.10",
			"cluster-master-1": "10.12.1.11",
			"cluster-master-2": "",
		}

		return controller
	}
	refreshed := []session.Host{
		{Host: "10.12.1.10", Name: "cluster-master-0"},
		{Host: "10.12.1.21", Name: "cluster-master-1"},
		{Host: "10.12.1.22", Name: "cluster-master-2"},
		{Host: "10.12.1.23", Name: "cluster-master-3"},
	}
	wantNodeToHost := map[string]string{
		"cluster-master-0": "10.12.1.10",
		"cluster-master-1": "10.12.1.21",
		"cluster-master-2": "10.12.1.22",
	}

	t.Run("stops readiness clients of changed masters", func(t *testing.T) {
		sshProvider := testssh.NewSSHProvider(session.NewSession(session.Input{}), true)
		openChecker := func(nodeName, address string) libcon.SSHClient {
			sess := session.NewSession(session.Input{AvailableHosts: []session.Host{{Host: address, Name: nodeName}}})
			cl, err := sshProvider.StandaloneClientFor(t.Context(), controlplane.SSHCheckerClientKey(nodeName), sess, nil)
			require.NoError(t, err)
			return cl
		}
		unchangedClient := openChecker("cluster-master-0", "10.12.1.10")
		staleClient := openChecker("cluster-master-1", "10.12.1.11")

		controller := newController(false)
		err := controller.syncNodeToHost(context.NewContext(t.Context(), context.Params{}), refreshed, func() (libcon.SSHProvider, error) {
			return sshProvider, nil
		})

		require.NoError(t, err)
		require.Equal(t, wantNodeToHost, controller.nodeToHost)
		require.Same(t, unchangedClient, openChecker("cluster-master-0", "10.12.1.10"))
		require.NotSame(t, staleClient, openChecker("cluster-master-1", "10.12.1.21"))
	})

	t.Run("a provider that cannot be had fails the sync", func(t *testing.T) {
		controller := newController(false)
		err := controller.syncNodeToHost(context.NewContext(t.Context(), context.Params{}), refreshed, func() (libcon.SSHProvider, error) {
			return nil, errors.New("no provider")
		})

		require.ErrorContains(t, err, "no provider")
		require.Equal(t, wantNodeToHost, controller.nodeToHost)
	})

	// No SSH checker is built when checks are skipped, nor in commander: nothing to stop.
	// Nothing changed asks for nothing either, which keeps sshless runs out of the provider.
	for name, tc := range map[string]struct {
		controller *MasterNodeGroupController
		ctx        *context.Context
		hosts      []session.Host
	}{
		"skipped checks":  {controller: newController(true), ctx: context.NewContext(t.Context(), context.Params{}), hosts: refreshed},
		"commander":       {controller: newController(false), ctx: context.NewCommanderContext(t.Context(), context.Params{}, &commander.CommanderModeParams{}), hosts: refreshed},
		"nothing changed": {controller: newController(false), ctx: context.NewContext(t.Context(), context.Params{}), hosts: []session.Host{{Host: "10.12.1.10", Name: "cluster-master-0"}}},
	} {
		t.Run(name, func(t *testing.T) {
			err := tc.controller.syncNodeToHost(tc.ctx, tc.hosts, func() (libcon.SSHProvider, error) {
				return nil, errors.New("the provider must not be asked for")
			})

			require.NoError(t, err)
		})
	}
}

// A failed apply can have recreated the VM. Its state is kept and the inventory refreshed
// from it, so the next master's readiness checks reach this one at the new address.
func TestAfterFailedApply(t *testing.T) {
	applyErr := errors.New("apply failed")
	lostClient := fmt.Errorf("hook: %w", context.ErrSSHClientSwitchFailed)

	cases := []struct {
		name       string
		applyErr   error
		getState   func() ([]byte, error)
		refreshErr error

		wantState   string
		wantRefresh bool
		wantLost    bool
		wantStop    bool
	}{
		{
			name:        "partial state is kept and refreshed",
			applyErr:    applyErr,
			getState:    func() ([]byte, error) { return []byte("partial"), nil },
			wantState:   "partial",
			wantRefresh: true,
		},
		{
			name:      "unreadable state stops the loop",
			applyErr:  applyErr,
			getState:  func() ([]byte, error) { return nil, errors.New("no state file") },
			wantState: "old",
			wantStop:  true,
		},
		{
			name:      "empty state stops the loop",
			applyErr:  applyErr,
			getState:  func() ([]byte, error) { return nil, nil },
			wantState: "old",
			wantStop:  true,
		},
		{
			name:      "lost client is not refreshed through",
			applyErr:  lostClient,
			getState:  func() ([]byte, error) { return []byte("partial"), nil },
			wantState: "partial",
			wantLost:  true,
		},
		{
			name:        "client lost by the refresh stops the loop",
			applyErr:    applyErr,
			getState:    func() ([]byte, error) { return []byte("partial"), nil },
			refreshErr:  lostClient,
			wantState:   "partial",
			wantRefresh: true,
			wantLost:    true,
			wantStop:    true,
		},
		{
			name:        "any refresh failure stops the loop",
			applyErr:    applyErr,
			getState:    func() ([]byte, error) { return []byte("partial"), nil },
			refreshErr:  errors.New("output failed"),
			wantState:   "partial",
			wantRefresh: true,
			wantStop:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			switcher := &recordingMasterClientSwitcher{err: tc.refreshErr}
			convergeCtx := context.NewContext(t.Context(), context.Params{ClientSwitcher: switcher})
			nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
				State: map[string][]byte{"cluster-master-1": []byte("old")},
			}, nil, nil)

			err := NewMasterNodeGroupController(nodeGroup, false).afterFailedApply(convergeCtx, "cluster-master-1", tc.getState, tc.applyErr)

			require.ErrorIs(t, err, tc.applyErr)
			require.Equal(t, tc.wantLost, errors.Is(err, context.ErrSSHClientSwitchFailed))
			require.Equal(t, tc.wantStop, errors.Is(err, errMasterHostsRefreshFailed))
			require.Equal(t, tc.wantState, string(nodeGroup.state.State["cluster-master-1"]))
			require.Equal(t, tc.wantRefresh, switcher.refreshedStates != nil)
		})
	}
}

type countingNodeGroup struct {
	nodeGroupController
	updateErr error
	updated   []string
}

func (g *countingNodeGroup) updateNode(_ *context.Context, name string, _ int) error {
	g.updated = append(g.updated, name)
	return g.updateErr
}

// A failure of a single node leaves the others to converge. A lost SSH client does not:
// the lock is not renewed any more, and the next master must not be changed without it.
func TestUpdateNodesStopsWhenSSHClientSwitchFails(t *testing.T) {
	newController := func(updateErr error) (*NodeGroupController, *countingNodeGroup) {
		controller := NewNodeGroupController("master", state.NodeGroupInfrastructureState{State: map[string][]byte{
			"cluster-master-0": nil,
			"cluster-master-1": nil,
			"cluster-master-2": nil,
		}}, nil, nil)
		controller.desiredReplicas = 3
		nodeGroup := &countingNodeGroup{updateErr: updateErr}
		controller.nodeGroup = nodeGroup

		return controller, nodeGroup
	}

	t.Run("other error", func(t *testing.T) {
		controller, nodeGroup := newController(errors.New("apply failed"))

		require.Error(t, controller.updateNodes(context.NewContext(t.Context(), context.Params{})))
		require.Len(t, nodeGroup.updated, 3)
	})

	for name, stopErr := range map[string]error{
		"lost SSH client":         context.ErrSSHClientSwitchFailed,
		"inventory not refreshed": errMasterHostsRefreshFailed,
	} {
		t.Run(name, func(t *testing.T) {
			controller, nodeGroup := newController(fmt.Errorf("update: %w", stopErr))

			err := controller.updateNodes(context.NewContext(t.Context(), context.Params{}))

			require.ErrorIs(t, err, stopErr)
			require.Equal(t, []string{"cluster-master-2"}, nodeGroup.updated)
		})
	}
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

// The live SSH session keeps masters pending deletion so the client can leave them
// safely. Readiness checks, however, describe only the masters left in state. Mixing
// those two inventories made a normal 3→1 scale-down look as if the user had passed
// too many --ssh-host flags and produced a misleading warning with two ignored hosts.
func TestHostsMatchingNodesExcludesMastersPendingDeletion(t *testing.T) {
	allHosts := []session.Host{
		{Host: "10.12.1.33", Name: "cluster-master-0"},
		{Host: "10.12.1.189", Name: "cluster-master-1"},
		{Host: "10.12.0.52", Name: "cluster-master-2"},
	}

	hostsForChecks := hostsMatchingNodes(allHosts, []string{"cluster-master-0"})

	require.Equal(t,
		[]session.Host{{Host: "10.12.1.33", Name: "cluster-master-0"}},
		hostsForChecks)

	var confirmation string
	nodeToHost, err := utils.CheckSSHHosts(hostsForChecks, []string{"cluster-master-0"}, "", func(message string) bool {
		confirmation = message
		return true
	})

	require.NoError(t, err)
	require.Equal(t, map[string]string{"cluster-master-0": "10.12.1.33"}, nodeToHost)
	require.NotContains(t, confirmation, "Too many master SSH-hosts")
	require.NotContains(t, confirmation, "(ignored)")
}

// CheckSSHHosts asks to confirm the node-to-host mapping on every run, destructive plan
// or not. Answering "no" for a caller that has no terminal cost the whole hook, so the
// master was recreated with no guard; the answer is now yes plus a log line. Tests run
// without a TTY, which is exactly the case under test.
func TestHostsMappingConfirmedWithoutTerminal(t *testing.T) {
	convergeCtx := context.NewContext(t.Context(), context.Params{})

	require.True(t, confirmOrProceed(convergeCtx)("master-0 -> 10.12.1.10"))
}

func TestSurvivingMasterNodeNames(t *testing.T) {
	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{
			State: map[string][]byte{
				"cluster-master-2": nil,
				"cluster-master-0": nil,
				"cluster-master-1": nil,
			},
		}, nil, nil),
		false,
	)

	require.Equal(t,
		[]string{"cluster-master-0", "cluster-master-2"},
		controller.survivingMasterNodeNames("cluster-master-1"))
}

// A single master has no surviving endpoint to put in refreshed cloud-init.
// Its destructive-plan hook must trigger the existing 1→3→1 workflow first.
func TestSingleMasterHasNoVariablesRefresher(t *testing.T) {
	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{
			State: map[string][]byte{"cluster-master-0": nil},
		}, nil, nil),
		false,
	)

	require.Nil(t, controller.makeMasterNodeVariablesRefresher(nil, nil, "cluster-master-0", 0, 1, 0))
}

func newAPIServerPod(nodeName, ip string, ready corev1.ConditionStatus) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-" + nodeName,
			Namespace: "kube-system",
			Labels:    map[string]string{"component": "kube-apiserver", "tier": "control-plane"},
		},
		Spec: corev1.PodSpec{NodeName: nodeName, HostNetwork: true},
		Status: corev1.PodStatus{
			PodIP:      ip,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: ready}},
		},
	}
}

// newRefresherTestController returns a three-master controller and a fake cluster
// with the masters' apiserver Pods and the manual-bootstrap-for-master Secret.
// cluster-master-0 is the one being replaced: its own apiserver is not required.
func newRefresherTestController(
	t *testing.T,
	master2Ready corev1.ConditionStatus,
	secretEndpoints string,
) (*MasterNodeGroupController, *context.Context, *k8sfake.Clientset) {
	t.Helper()

	kubeCl := client.NewFakeKubernetesClient()
	for _, pod := range []*corev1.Pod{
		newAPIServerPod("cluster-master-0", "10.0.0.10", corev1.ConditionFalse),
		newAPIServerPod("cluster-master-1", "10.0.0.11", corev1.ConditionTrue),
		newAPIServerPod("cluster-master-2", "10.0.0.12", master2Ready),
	} {
		_, err := kubeCl.CoreV1().Pods(pod.Namespace).Create(t.Context(), pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	_, err := kubeCl.CoreV1().Secrets("d8-cloud-instance-manager").Create(t.Context(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "manual-bootstrap-for-master", Namespace: "d8-cloud-instance-manager"},
		Data: map[string][]byte{
			"apiserverEndpoints": []byte(secretEndpoints),
			"cloud-config":       []byte("#cloud-config"),
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	clientset, ok := kubeCl.KubeClient.(*klient.Client).Interface.(*k8sfake.Clientset)
	require.True(t, ok, "fake kube client is not backed by a fake clientset")

	convergeCtx := context.NewContext(t.Context(), context.Params{
		KubeProvider: staticKubeProvider{client: execCapableClient{kubeCl.KubeClient}},
	})

	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
		State: map[string][]byte{"cluster-master-0": nil, "cluster-master-1": nil, "cluster-master-2": nil},
	}, nil, nil)
	nodeGroup.cloudConfig = "old cloud config"

	return NewMasterNodeGroupController(nodeGroup, false), convergeCtx, clientset
}

const bothSurvivorsEndpoints = "- 10.0.0.11:6443\n- 10.0.0.12:6443\n"

func TestMasterNodeVariablesRefresher(t *testing.T) {
	wasInTestEnvironment := retry.InTestEnvironment
	retry.InTestEnvironment = true
	t.Cleanup(func() { retry.InTestEnvironment = wasInTestEnvironment })

	tests := []struct {
		name            string
		master2Ready    corev1.ConditionStatus
		secretEndpoints string
		expectedErrs    []string
		expectedConfig  string
	}{
		{
			name:            "ready survivors",
			master2Ready:    corev1.ConditionTrue,
			secretEndpoints: bothSurvivorsEndpoints,
			expectedConfig:  base64.StdEncoding.EncodeToString([]byte("#cloud-config")),
		},
		{
			name:            "not ready survivor",
			master2Ready:    corev1.ConditionFalse,
			secretEndpoints: bothSurvivorsEndpoints,
			expectedErrs:    []string{`get ready API server hosts before replacing master node "cluster-master-0": `, "cluster-master-2"},
			expectedConfig:  "old cloud config",
		},
		{
			// The ready survivors are what the refresher waits for in the Secret.
			name:            "secret lacks a ready survivor",
			master2Ready:    corev1.ConditionTrue,
			secretEndpoints: "- 10.0.0.11:6443\n",
			expectedErrs:    []string{`get refreshed cloud config before replacing master node "cluster-master-0": `, "10.0.0.12"},
			expectedConfig:  "old cloud config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, convergeCtx, _ := newRefresherTestController(t, tt.master2Ready, tt.secretEndpoints)
			metaConfig := &config.MetaConfig{}

			// A non-zero index catches a refresher that ignores the index it is given.
			const nodeIndex = 2
			refresh := controller.makeMasterNodeVariablesRefresher(convergeCtx, metaConfig, "cluster-master-0", nodeIndex, 1, 0)
			require.NotNil(t, refresh)

			variables, err := refresh(t.Context())
			require.Equal(t, tt.expectedConfig, controller.cloudConfig)
			if len(tt.expectedErrs) > 0 {
				for _, expectedErr := range tt.expectedErrs {
					require.ErrorContains(t, err, expectedErr)
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, metaConfig.NodeGroupConfig("master", nodeIndex, tt.expectedConfig), variables)
		})
	}
}

func TestMasterNodeVariablesRefresherRetries(t *testing.T) {
	wasInTestEnvironment := retry.InTestEnvironment
	retry.InTestEnvironment = false
	t.Cleanup(func() { retry.InTestEnvironment = wasInTestEnvironment })

	t.Run("waits for a survivor to turn ready", func(t *testing.T) {
		controller, convergeCtx, clientset := newRefresherTestController(t, corev1.ConditionFalse, bothSurvivorsEndpoints)

		lists := 0
		clientset.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
			lists++
			if lists == 2 {
				pod := newAPIServerPod("cluster-master-2", "10.0.0.12", corev1.ConditionTrue)
				require.NoError(t, clientset.Tracker().Update(corev1.SchemeGroupVersion.WithResource("pods"), pod, pod.Namespace))
			}
			return false, nil, nil
		})

		refresh := controller.makeMasterNodeVariablesRefresher(convergeCtx, &config.MetaConfig{}, "cluster-master-0", 0, 3, time.Millisecond)
		_, err := refresh(t.Context())
		require.NoError(t, err)
		require.Equal(t, 2, lists)
		require.Equal(t, base64.StdEncoding.EncodeToString([]byte("#cloud-config")), controller.cloudConfig)
	})

	t.Run("stops on a permanent auth error", func(t *testing.T) {
		controller, convergeCtx, clientset := newRefresherTestController(t, corev1.ConditionTrue, bothSurvivorsEndpoints)

		lists := 0
		clientset.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
			lists++
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", nil)
		})

		refresh := controller.makeMasterNodeVariablesRefresher(convergeCtx, &config.MetaConfig{}, "cluster-master-0", 0, 3, time.Millisecond)
		_, err := refresh(kubeerrors.WithAuthMode(t.Context(), kubeerrors.AuthModeOwnCredentials))
		require.True(t, apierrors.IsForbidden(err), "unexpected error: %v", err)
		require.Equal(t, 1, lists)
		require.Equal(t, "old cloud config", controller.cloudConfig)
	})
}
