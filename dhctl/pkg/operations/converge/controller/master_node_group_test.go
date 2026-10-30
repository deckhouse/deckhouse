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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/utils"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

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

// newSSHContext gives the controller an SSH configuration: without one populateNodeToHost
// builds no readiness inventory at all. The switcher is what the refresh goes through.
func newSSHContext(t *testing.T, switcher context.MultiMasterClientSwitcher) *context.Context {
	initializer := providerinitializer.NewSSHProviderInitializer(
		settings.NewBaseProviders(settings.ProviderParams{}),
		&sshconfig.ConnectionConfig{
			Config: &sshconfig.Config{},
			Hosts:  []sshconfig.Host{{Host: "10.12.1.10"}},
		},
	)

	return context.NewContext(t.Context(), context.Params{
		ClientSwitcher:         switcher,
		SSHProviderInitializer: initializer,
		ChangeParams:           infrastructure.ChangeActionSettings{AutomaticSettings: infrastructure.AutomaticSettings{AutoApproveSettings: infrastructure.AutoApproveSettings{AutoApprove: true}}},
	})
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
	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{State: nodesState}, nil, nil),
		false,
	)
	controller.convergeState = &context.State{}

	err := controller.populateNodeToHost(newSSHContext(t, switcher))

	require.NoError(t, err)
	require.Equal(t, nodesState, switcher.refreshedStates)
	require.Equal(t, map[string]string{
		"cluster-master-0": "10.12.1.10",
		"cluster-master-1": "10.12.1.11",
		"cluster-master-2": "10.12.1.12",
	}, controller.nodeToHost)
}

// A refresh that fails leaves no readiness inventory to trust, so the converge stops.
func TestPopulateNodeToHostFailsWhenRefreshFails(t *testing.T) {
	switcher := &recordingMasterClientSwitcher{err: errors.New("cannot extract IPs")}
	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{
			State: map[string][]byte{"cluster-master-0": []byte("state-0")},
		}, nil, nil),
		false,
	)
	controller.convergeState = &context.State{}

	err := controller.populateNodeToHost(newSSHContext(t, switcher))

	require.ErrorIs(t, err, errMasterHostsRefreshFailed)
	require.Nil(t, controller.nodeToHost)
}

// A hook that cannot be built must stop the master, not turn into a nil hook: the runner
// replaces a nil hook with a DummyHook and would recreate the VM with no control-plane
// guard. The refresh error stays visible to the break in updateNodes.
func TestNewHookForUpdatePipelineFailsWhenRefreshFails(t *testing.T) {
	switcher := &recordingMasterClientSwitcher{err: fmt.Errorf("switch: %w", context.ErrSSHClientSwitchFailed)}
	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{
			State: map[string][]byte{
				"cluster-master-0": []byte("state-0"),
				"cluster-master-1": []byte("state-1"),
			},
		}, nil, nil),
		false,
	)
	controller.convergeState = &context.State{}

	hook, err := controller.newHookForUpdatePipeline(newSSHContext(t, switcher), "cluster-master-0", nil)

	require.Nil(t, hook)
	require.ErrorIs(t, err, errMasterHostsRefreshFailed)
	require.ErrorIs(t, err, context.ErrSSHClientSwitchFailed)
}

// A kube-flag run on a machine with ~/.ssh/id_rsa gets an SSH configuration with no
// hosts: converge-migration by hand, or converge with DHCTL_CLI_NO_SWITCH_TO_NODE_USER.
// It cannot check or switch masters, so it keeps the DummyHook 1.77 always gave it: a
// built hook would turn a destructive plan into a scale dance it cannot finish.
func TestNewHookForUpdatePipelineWithoutSSHHosts(t *testing.T) {
	switcher := &recordingMasterClientSwitcher{}
	controller := NewMasterNodeGroupController(
		NewNodeGroupController("master", state.NodeGroupInfrastructureState{
			State: map[string][]byte{
				"cluster-master-0": []byte("state-0"),
				"cluster-master-1": []byte("state-1"),
			},
		}, nil, nil),
		false,
	)
	controller.convergeState = &context.State{}

	ctx := context.NewContext(t.Context(), context.Params{
		ClientSwitcher: switcher,
		SSHProviderInitializer: providerinitializer.NewSSHProviderInitializer(
			settings.NewBaseProviders(settings.ProviderParams{}),
			&sshconfig.ConnectionConfig{Config: &sshconfig.Config{}},
		),
	})

	hook, err := controller.newHookForUpdatePipeline(ctx, "cluster-master-0", &config.MetaConfig{})

	require.NoError(t, err)
	require.Nil(t, hook)
	require.Nil(t, switcher.refreshedStates)
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
	nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{
		State: map[string][]byte{"cluster-master-0": []byte("state-0")},
	}, nil, nil)
	nodeGroup.nodesToDelete = []nodeToDeleteInfo{
		{name: "cluster-master-2", state: []byte("state-2")},
		{name: "cluster-master-1", state: []byte("state-1")},
	}
	controller := NewMasterNodeGroupController(nodeGroup, false)
	controller.convergeState = &context.State{}

	require.NoError(t, controller.populateNodeToHost(newSSHContext(t, switcher)))

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
		{name: "cluster-master-2", state: []byte("state-2")},
		{name: "cluster-master-1", state: []byte("state-1")},
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
		name    string
		outputs *infrastructure.PipelineOutputs
		want    [][]string
	}{
		{name: "outputs", outputs: outputs, want: [][]string{{"cluster-master-0", "state", "10.12.1.10", "192.168.0.10"}}},
		{name: "no outputs"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			switcher := &recordingMasterClientSwitcher{}
			nodeGroup := NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil)

			NewMasterNodeGroupController(nodeGroup, false).
				rememberMasterIPs(context.NewContext(t.Context(), context.Params{ClientSwitcher: switcher}), "cluster-master-0", tc.outputs)

			require.Equal(t, tc.want, switcher.remembered)
		})
	}
}

// The readiness checks reach the other masters by the addresses in nodeToHost. A master
// recreated with a new IP has to be checked at the new one, or the next master waits for
// a machine that is gone and fails the converge. A master the inventory does not list has
// no address right now and must stay in the checks.
func TestSyncNodeToHost(t *testing.T) {
	controller := NewMasterNodeGroupController(NewNodeGroupController("master", state.NodeGroupInfrastructureState{}, nil, nil), false)
	controller.nodeToHost = map[string]string{
		"cluster-master-0": "10.12.1.10",
		"cluster-master-1": "10.12.1.11",
		"cluster-master-2": "",
		"cluster-master-4": "10.12.1.14",
	}

	controller.syncNodeToHost([]session.Host{
		{Host: "10.12.1.10", Name: "cluster-master-0"},
		{Host: "10.12.1.21", Name: "cluster-master-1"},
		{Host: "10.12.1.22", Name: "cluster-master-2"},
		{Host: "10.12.1.23", Name: "cluster-master-3"},
	})

	require.Equal(t, map[string]string{
		"cluster-master-0": "10.12.1.10",
		"cluster-master-1": "10.12.1.21",
		"cluster-master-2": "10.12.1.22",
		"cluster-master-4": "10.12.1.14",
	}, controller.nodeToHost)
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

func (g *countingNodeGroup) updateNode(_ *context.Context, name string) error {
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
			require.Len(t, nodeGroup.updated, 1)
		})
	}
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
