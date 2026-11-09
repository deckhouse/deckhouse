/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkpatchablevalues "github.com/deckhouse/module-sdk/pkg/patchable-values"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	bashible_model "github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	registry_requirements "github.com/deckhouse/deckhouse/modules/038-registry/requirements"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
)

func TestGateDecide(t *testing.T) {
	cases := []struct {
		name    string
		gate    gate
		enabled bool
		// blocked is a fragment of the reason, empty when nothing should be blocked.
		blocked string
	}{{
		// A cluster the previous implementation has never touched. There is no choice to
		// offer and nothing to hand over, so this one manages it from the start.
		name:    "a cluster with no previous state at all",
		gate:    gate{},
		enabled: true,
	}, {
		name:    "a cluster the previous implementation has let go of",
		gate:    gate{Legacy: &legacyState{Mode: "Unmanaged"}},
		enabled: true,
	}, {
		// The case the handover exists for. Both implementations would write
		// /etc/containerd/registry.d on the same nodes.
		name:    "a cluster the previous implementation still manages",
		gate:    gate{Legacy: &legacyState{Mode: "Proxy"}},
		enabled: false,
		blocked: `the cluster is in the "Proxy" mode`,
	}, {
		// `Direct`, and the case this gate was widened for.
		//
		// A `Direct` cluster pulls through the in-cluster address, and the objects that served
		// it — the previous implementation's Service and in-cluster proxy — are not rendered by
		// this release at all: kept by their `keep` policy after an update from 1.77 but updated by
		// nothing, and gone after a jump from an older release. So refusing for good does not keep
		// such a cluster safe: it leaves it pulling through objects nothing maintains.
		//
		// What makes taking over safe is that the operator has already written where images come
		// from. Then this implementation serves the same address from that configuration and the
		// node agent takes over the runtime configuration; the address never goes unserved.
		name:    "Direct, with the module's own configuration already written",
		gate:    gate{Legacy: &legacyState{Mode: "Direct"}, ModuleConfigured: true},
		enabled: true,
	}, {
		// Without that configuration there is nothing to serve the address WITH, and switching on
		// would mean this implementation managing nothing while the previous one's objects go away.
		name:    "Direct, with nothing configured",
		gate:    gate{Legacy: &legacyState{Mode: "Direct"}},
		enabled: false,
		blocked: `is in the "Direct" mode`,
	}, {
		// Configuration does not make a moving cluster safe to take over from: the nodes are
		// being reconfigured by the previous implementation right now.
		name:    "Direct, configured, but on its way somewhere else",
		gate:    gate{Legacy: &legacyState{Mode: "Direct", TargetMode: "Local"}, ModuleConfigured: true},
		enabled: false,
		blocked: `transitioning to "Local"`,
	}, {
		// `Proxy` and `Local` are not widened, and that is a decision rather than an omission.
		// Both keep state this release cannot account for: static pods with their own PKI on the
		// nodes, and for `Local` the image store on the master disks. `Direct` keeps neither.
		name:    "Proxy stays refused even when configured",
		gate:    gate{Legacy: &legacyState{Mode: "Proxy"}, ModuleConfigured: true},
		enabled: false,
		blocked: `is in the "Proxy" mode`,
	}, {
		name:    "Local stays refused even when configured",
		gate:    gate{Legacy: &legacyState{Mode: "Local"}, ModuleConfigured: true},
		enabled: false,
		blocked: `is in the "Local" mode`,
	}, {
		name:    "a cluster mid-transition inside the previous implementation",
		gate:    gate{Legacy: &legacyState{Mode: "Unmanaged", TargetMode: "Local"}},
		enabled: false,
		blocked: `transitioning to "Local"`,
	}, {
		// Settling into Unmanaged is not a reason to wait: the target is where the
		// handover wants the cluster anyway.
		name:    "a cluster settling into Unmanaged",
		gate:    gate{Legacy: &legacyState{Mode: "Unmanaged", TargetMode: "Unmanaged"}},
		enabled: true,
	}, {
		name:    "a previous state that records no mode",
		gate:    gate{Legacy: &legacyState{}},
		enabled: false,
		blocked: "has not recorded a mode",
	}, {
		// Fails closed. Guessing here would enable a second writer on every node.
		name: "a previous state that cannot be read",
		gate: gate{
			LegacyUnreadable: errors.New("decoding the legacy registry state: yaml: line 3: mapping values are not allowed"),
		},
		enabled: false,
		blocked: "cannot be read",
	}, {
		// Once handed over, the question has expired. A module restart must not re-ask it
		// and hand the cluster back — the previous state secret is still there and still
		// says whatever it said.
		name: "after the handover, with the previous state still saying Proxy",
		gate: gate{
			AlreadySwitched: true,
			Legacy:          &legacyState{Mode: "Proxy"},
		},
		enabled: true,
	}, {
		// The marker alone is enough. There is no setting that could contradict it,
		// which is the whole point of removing the choice.
		name:    "after the handover, with no previous state left",
		gate:    gate{AlreadySwitched: true},
		enabled: true,
	}, {
		// An Engine cluster carrying a state the legacy implementation recorded for itself.
		// Refusing over it is not caution here: the way out of `Direct` is a bashible
		// transition, and this cluster has no bashible, so the refusal would be permanent.
		name: "an Engine cluster the previous implementation recorded a mode on",
		gate: gate{
			Legacy:     &legacyState{Mode: modeDirect},
			EngineOnly: true,
		},
		enabled: true,
	}, {
		// The same, for the mode whose way out is not widened for anyone else.
		name: "an Engine cluster recorded in Proxy",
		gate: gate{
			Legacy:     &legacyState{Mode: "Proxy"},
			EngineOnly: true,
		},
		enabled: true,
	}, {
		// An unreadable state is the one case the gate otherwise refuses over, and it is
		// refused because guessing would put a second writer on every node. There are no
		// bashible writers here to be second to.
		name: "an Engine cluster whose previous state cannot be read",
		gate: gate{
			LegacyUnreadable: errors.New("decoding the legacy registry state: unexpected EOF"),
			EngineOnly:       true,
		},
		enabled: true,
	}, {
		// The line this is drawn at. One bashible group means nodes the legacy
		// implementation can still write to, so the handover is asked in full.
		name: "a cluster still holding one bashible group",
		gate: gate{
			Legacy:     &legacyState{Mode: modeDirect},
			EngineOnly: false,
		},
		enabled: false,
		blocked: `the cluster is in the "Direct" mode`,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			enabled, reason := test.gate.decide()

			assert.Equal(t, test.enabled, enabled)
			if test.blocked == "" {
				assert.Empty(t, reason)
				return
			}
			assert.Contains(t, reason, test.blocked)
		})
	}
}

// TestGateRefusalsSayTheStateIsUnsupported: every refusal is read by an operator through the alert.
//
// `Direct` is the one state this release takes over from, once told where images come from, so its
// reason names that step. Every other refusal describes a state nothing in this release can move:
// the v1 code that could is gone, and the platform does not go back to it. Those reasons say that the
// state is not supported, how a cluster got there, what not to touch and whom to ask — and nothing
// that reads as a way back.
func TestGateRefusalsSayTheStateIsUnsupported(t *testing.T) {
	_, reason := gate{Legacy: &legacyState{Mode: modeDirect}}.decide()
	assert.Contains(t, reason, "primary.upstream")
	assert.NotContains(t, reason, "Unmanaged")

	for name, g := range map[string]gate{
		"Proxy":         {Legacy: &legacyState{Mode: "Proxy"}},
		"Local":         {Legacy: &legacyState{Mode: "Local"}},
		"transitioning": {Legacy: &legacyState{Mode: modeDirect, TargetMode: "Local"}},
		"no mode":       {Legacy: &legacyState{}},
		"unreadable": {
			LegacyUnreadable: errors.New("decoding the state: unexpected EOF"),
		},
		"node configuration only":       {NodeConfig: &legacyNodeConfig{Mode: "Proxy", ProxyEndpoints: true}},
		"unreadable node configuration": {NodeConfig: &legacyNodeConfig{Unreadable: true}},
	} {
		t.Run(name, func(t *testing.T) {
			enabled, reason := g.decide()
			assert.False(t, enabled)
			assert.Contains(t, reason, unsupported)
			assert.Contains(t, reason, "v1 implementation")
			for _, wayBack := range []string{"previous release", "return", "roll", "downgrade", "1.77", "settle"} {
				assert.NotContains(t, reason, wayBack)
			}
		})
	}
}

// TestGateReadsTheNodeConfigurationTheV1ImplementationLeft: the second witness, for the pass after the
// update has deleted `registry-state`.
//
// That secret has no `keep` policy, so the release that refuses the handover deletes it, and a gate
// that read only the state found nothing on its next pass and opened — seconds later, on its own,
// with the v1 node configuration still on every node. `registry-bashible-config` carries `keep` on
// every release that wrote it, so it is still there to say what the v1 implementation was doing.
func TestGateReadsTheNodeConfigurationTheV1ImplementationLeft(t *testing.T) {
	cases := []struct {
		name    string
		gate    gate
		enabled bool
		blocked string
	}{{
		// The core case: a forced update from a `Proxy` cluster, one pass after Helm deleted the state.
		name:    "only a Proxy node configuration is left",
		gate:    gate{NodeConfig: &legacyNodeConfig{Mode: "Proxy", ProxyEndpoints: true}},
		enabled: false,
		blocked: `the cluster is in the "Proxy" mode`,
	}, {
		name:    "only a Local node configuration is left",
		gate:    gate{NodeConfig: &legacyNodeConfig{Mode: "Local", ProxyEndpoints: true}},
		enabled: false,
		blocked: `the cluster is in the "Local" mode`,
	}, {
		// The document names the target mode, so a cluster moving from `Proxy` to `Direct` reads as
		// `Direct` there. The proxy endpoints are what give the move away, and a move is refused even
		// when `Direct` on its own would be admitted.
		name: "proxy endpoints under Direct are a move out of Proxy",
		gate: gate{
			NodeConfig:       &legacyNodeConfig{Mode: modeDirect, ProxyEndpoints: true},
			ModuleConfigured: true,
		},
		enabled: false,
		blocked: `transitioning to "Direct"`,
	}, {
		name:    "proxy endpoints under Unmanaged are a move out of Proxy that has not finished",
		gate:    gate{NodeConfig: &legacyNodeConfig{Mode: "Unmanaged", ProxyEndpoints: true}},
		enabled: false,
		blocked: `the cluster is in the "Proxy" mode`,
	}, {
		// `Direct` asks the same question of this witness as of the state.
		name:    "a v1 Direct node configuration with the module configured",
		gate:    gate{NodeConfig: &legacyNodeConfig{Mode: modeDirect}, ModuleConfigured: true},
		enabled: true,
	}, {
		name:    "a v1 Direct node configuration with nothing configured",
		gate:    gate{NodeConfig: &legacyNodeConfig{Mode: modeDirect}},
		enabled: false,
		blocked: `is in the "Direct" mode`,
	}, {
		name:    "a v1 Unmanaged node configuration",
		gate:    gate{NodeConfig: &legacyNodeConfig{Mode: "Unmanaged"}},
		enabled: true,
	}, {
		name:    "a node configuration that records no mode",
		gate:    gate{NodeConfig: &legacyNodeConfig{}},
		enabled: false,
		blocked: "has not recorded a mode",
	}, {
		name:    "a node configuration that cannot be read",
		gate:    gate{NodeConfig: &legacyNodeConfig{Unreadable: true}},
		enabled: false,
		blocked: "registry-bashible-config cannot be read",
	}, {
		// Both witnesses have to say the v1 implementation let go, not the first one asked.
		name: "a state that let go, with node services still in the node configuration",
		gate: gate{
			Legacy:     &legacyState{Mode: "Unmanaged"},
			NodeConfig: &legacyNodeConfig{Mode: "Unmanaged", ProxyEndpoints: true},
		},
		enabled: false,
		blocked: `the cluster is in the "Proxy" mode`,
	}, {
		// While both are there the state answers first, so the reason, which is the alert's label,
		// does not change when the update deletes the state: the alert's five minutes keep counting.
		name: "both witnesses of a Proxy cluster give the state's reason",
		gate: gate{
			Legacy:     &legacyState{Mode: "Proxy"},
			NodeConfig: &legacyNodeConfig{Mode: "Proxy", ProxyEndpoints: true},
		},
		enabled: false,
		blocked: `the cluster is in the "Proxy" mode of the v1 implementation; `,
	}, {
		name: "the marker outranks a node configuration left behind",
		gate: gate{
			AlreadySwitched: true,
			NodeConfig:      &legacyNodeConfig{Mode: "Proxy", ProxyEndpoints: true},
		},
		enabled: true,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			enabled, reason := test.gate.decide()

			assert.Equal(t, test.enabled, enabled)
			if test.blocked == "" {
				assert.Empty(t, reason)
				return
			}
			assert.Contains(t, reason, test.blocked)
		})
	}

	// The reason is the same whichever witness gives it, which is what keeps the alert's series.
	_, fromState := gate{Legacy: &legacyState{Mode: "Proxy"}}.decide()
	_, fromNodeConfig := gate{NodeConfig: &legacyNodeConfig{Mode: "Proxy", ProxyEndpoints: true}}.decide()
	assert.Equal(t, fromState, fromNodeConfig)
}

// TestFilterLegacyNodeConfig: what the gate is told about `registry-bashible-config`, and what it is
// never told.
func TestFilterLegacyNodeConfig(t *testing.T) {
	cases := []struct {
		name string
		data map[string][]byte

		// marked puts the installer's mark for the document on the secret, markedWith the mark of
		// another document.
		marked     bool
		markedWith string

		want   *legacyNodeConfig
		absent bool
	}{{
		name: "a v1 Proxy configuration",
		data: map[string][]byte{"config": []byte(v1ProxyNodeConfig)},
		want: &legacyNodeConfig{Mode: "Proxy", ProxyEndpoints: true},
	}, {
		name: "a v1 Direct configuration",
		data: map[string][]byte{"config": []byte("mode: Direct\nimagesBase: registry.d8-system.svc:5001/system/deckhouse\n")},
		want: &legacyNodeConfig{Mode: modeDirect},
	}, {
		// This implementation's own document, and the installer's: they name the node agent, and no
		// v1 writer ever did.
		name:   "a configuration that names the node agent",
		data:   map[string][]byte{"config": []byte("agent:\n  endpoint: 127.0.0.1:4281\nmode: Managed\nproxyEndpoints: []\n")},
		absent: true,
	}, {
		// The installer's document of an installation from a bundle, as it wrote it. By its contents a
		// v1 `Local` cluster; read as one it would keep the gate, and with it the store the installation
		// waits for, shut for good.
		name:   "the installer's document of an installation from a bundle",
		data:   map[string][]byte{"config": []byte(installersBundleNodeConfig)},
		marked: true,
		absent: true,
	}, {
		// The installer's document of an installation with no registry settings: by its contents a v1
		// `Direct` cluster with nothing configured, which the gate refuses and records as v1.
		name:   "the installer's document of an installation with no registry settings",
		data:   map[string][]byte{"config": []byte(installersDirectNodeConfig)},
		marked: true,
		absent: true,
	}, {
		// The same document with no mark is what an installer of 1.77 left on a v1 `Local` cluster
		// whose v1 implementation has not rewritten it yet. It is the v1 implementation's.
		name: "the bundle document with no mark",
		data: map[string][]byte{"config": []byte(installersBundleNodeConfig)},
		want: &legacyNodeConfig{Mode: "Local", ProxyEndpoints: true},
	}, {
		name: "the no-settings document with no mark",
		data: map[string][]byte{"config": []byte(installersDirectNodeConfig)},
		want: &legacyNodeConfig{Mode: modeDirect},
	}, {
		// A mark speaks only for the document it was written with: once anything has rewritten the
		// document, it is read for what it says.
		name:       "a marked document rewritten since",
		data:       map[string][]byte{"config": []byte(v1ProxyNodeConfig)},
		markedWith: installersDirectNodeConfig,
		want:       &legacyNodeConfig{Mode: "Proxy", ProxyEndpoints: true},
	}, {
		name: "a v1 Local configuration",
		data: map[string][]byte{"config": []byte(
			"mode: Local\nproxyEndpoints:\n- 192.168.199.10:5001\n")},
		want: &legacyNodeConfig{Mode: "Local", ProxyEndpoints: true},
	}, {
		name: "a configuration that cannot be decoded",
		data: map[string][]byte{"config": []byte("mode: [Proxy")},
		want: &legacyNodeConfig{Unreadable: true},
	}, {
		name: "a secret with no configuration in it",
		data: map[string][]byte{},
		want: &legacyNodeConfig{Unreadable: true},
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			object := secretObject(t, BashibleConfigSecretName, test.data)
			switch {
			case test.marked:
				object.SetAnnotations(map[string]string{
					bashible_model.InstallerMarkAnnotation: bashible_model.InstallerMark(test.data["config"]),
				})
			case test.markedWith != "":
				object.SetAnnotations(map[string]string{
					bashible_model.InstallerMarkAnnotation: bashible_model.InstallerMark([]byte(test.markedWith)),
				})
			}

			result, err := filterLegacyNodeConfig(object)
			require.NoError(t, err, "a filter error is not a refusal: it blocks the module start or drops the event")

			if test.absent {
				assert.Nil(t, result)
				return
			}
			assert.Equal(t, *test.want, result)
		})
	}

	// The document carries the credentials of every host the nodes pull from. None of it may reach the
	// snapshot, from which it would be one log line away.
	result, err := filterLegacyNodeConfig(secretObject(t, BashibleConfigSecretName,
		map[string][]byte{"config": []byte(v1ProxyNodeConfig)}))
	require.NoError(t, err)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), testPassword)
	assert.NotContains(t, string(encoded), "192.168.199.10")
}

// TestAnUnreadableStateIsARefusal: `registry-state` that cannot be decoded is refused, and refused by
// the gate, with a reason — not by a filter error.
//
// A filter error looks like the safe answer and is neither. On the first list it fails the informer,
// so the module, which is critical, does not start; on a later event the event is dropped and the
// snapshot keeps whatever it said before. And a filter that returned nothing for an empty state handed
// the gate the one answer that opens it: no state at all.
func TestAnUnreadableStateIsARefusal(t *testing.T) {
	cases := []struct {
		name    string
		data    map[string][]byte
		blocked string
	}{{
		name:    "a state that cannot be decoded",
		data:    map[string][]byte{"state": []byte("mode: [Proxy")},
		blocked: "the state of the v1 implementation cannot be read: decoding the state:",
	}, {
		name:    "a secret with no state in it",
		data:    map[string][]byte{},
		blocked: "has not recorded a mode",
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result, err := filterLegacyState(secretObject(t, LegacyStateSecretName, test.data))
			require.NoError(t, err)
			require.NotNil(t, result, "a nil result is dropped from the snapshot, which reads as no state and opens the gate")

			input, _ := newSwitchInput(t, map[string]any{legacyStateSnapName: result})
			require.NoError(t, handleSwitch(context.Background(), input))

			current := valuesAfter(t, input)
			assert.False(t, current.Enabled)
			assert.Contains(t, current.SwitchBlockedReason, test.blocked)
		})
	}
}

// TestARefusalOutlivesTheStateItWasGivenFor plays the update of a `Proxy` cluster to this release
// pass by pass, with what each pass actually sees.
//
// Pass 1 sees both witnesses and refuses. Helm then deletes `registry-state`, which has no `keep`
// policy, and the deletion triggers pass 2 within seconds; `registry-bashible-config` is kept. Pass 2
// has to refuse with the same reason and set the alert's series again, or the alert, which waits five
// minutes, never fires and the gate deploys this implementation next to what the v1 one left. The
// node configuration hook then runs on the refused pass and must not withdraw that configuration: it
// is the one thing still pointing the nodes at a registry.
func TestARefusalOutlivesTheStateItWasGivenFor(t *testing.T) {
	nodeConfig, err := filterLegacyNodeConfig(secretObject(t, BashibleConfigSecretName,
		map[string][]byte{"config": []byte(v1ProxyNodeConfig)}))
	require.NoError(t, err)
	state, err := filterLegacyState(secretObject(t, LegacyStateSecretName,
		map[string][]byte{"state": []byte("mode: Proxy\n")}))
	require.NoError(t, err)

	var reasons []string
	for pass, snapshots := range []map[string]any{
		{legacyStateSnapName: state, legacyNodeConfigSnapName: nodeConfig},
		{legacyNodeConfigSnapName: nodeConfig},
	} {
		requirements.RemoveValue(registry_requirements.ImplementationKey)

		input, collector := newSwitchInput(t, snapshots)
		require.NoError(t, handleSwitch(context.Background(), input))

		current := valuesAfter(t, input)
		assert.Falsef(t, current.Enabled, "pass %d enabled this implementation", pass+1)
		reasons = append(reasons, current.SwitchBlockedReason)

		recorded, _ := requirements.GetValue(registry_requirements.ImplementationKey)
		assert.Equal(t, ImplementationLegacy, recorded)

		// Expired and set again in the same pass: the series is there after every pass that refuses.
		ops := collector.CollectedMetrics()
		require.Len(t, ops, 2)
		assert.Equal(t, operation.ActionExpireMetrics, ops[0].Action)
		assert.Equal(t, blockedMetricGroup, ops[0].Group)
		assert.Equal(t, operation.ActionGaugeSet, ops[1].Action)
		assert.Equal(t, blockedMetric, ops[1].Name)
		assert.Equal(t, current.SwitchBlockedReason, ops[1].Labels["reason"])

		// The refused values, handed to the hook that would otherwise withdraw the node configuration.
		nodeInput, _ := newHookInput(t, map[string]any{bashibleConfigSnapName: BashibleConfigSecretName},
			map[string]any{"internal": map[string]any{"v2": current}})
		require.NoError(t, handle(context.Background(), nodeInput))
		assert.Emptyf(t, nodeInput.PatchCollector.(*object_patch.PatchCollector).Operations(),
			"pass %d touched the cluster while the handover was refused", pass+1)
	}
	assert.Equal(t, reasons[0], reasons[1], "the alert's label changed, which restarts its five minutes")

	// The same hook does withdraw it once this implementation is active and manages nothing, which
	// is what the assertion above guards against reaching.
	nodeInput, _ := newHookInput(t, map[string]any{bashibleConfigSnapName: BashibleConfigSecretName},
		map[string]any{"mode": "Unmanaged", "internal": map[string]any{"v2": Values{Enabled: true}}})
	require.NoError(t, handle(context.Background(), nodeInput))
	ops := nodeInput.PatchCollector.(*object_patch.PatchCollector).Operations()
	require.Len(t, ops, 1)
	assert.Contains(t, ops[0].Description(), BashibleConfigSecretName)
}

// TestRecordImplementation: the cluster has to say which implementation it is running, or the
// release requirement cannot compare anything.
//
// Worth its own test because of how this one fails. An absent value is treated as "no information"
// and passes every release — deliberately, so that a cluster whose module has not reported yet is
// not stranded — which means a recording that stopped happening would leave the gate wide open and
// nothing anywhere would complain. The check on the other side of this value is covered by
// modules/038-registry/requirements/check_test.go; what is covered here is that it gets anything to
// check at all.
//
// Verified on a cluster as well, which is the only place it can be: a requirement is evaluated by
// the version already installed, so this gate is only ever exercised by a cluster running this code.
func TestRecordImplementation(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		want    string
	}{{
		name:    "a cluster this implementation manages",
		enabled: true,
		want:    ImplementationV2,
	}, {
		// The case the requirement exists for: while the previous implementation still manages the
		// cluster, a release that drops it must not be installable.
		name:    "a cluster the previous implementation still manages",
		enabled: false,
		want:    ImplementationLegacy,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			requirements.RemoveValue(registry_requirements.ImplementationKey)

			recordImplementation(test.enabled)

			got, recorded := requirements.GetValue(registry_requirements.ImplementationKey)
			require.True(t, recorded,
				"nothing was recorded, so the release check has no information and lets every release through")
			assert.Equal(t, test.want, got)
		})
	}
}

// TestEngineOnly: which clusters have no bashible writer left, phrased as the question the
// gate asks of the NodeGroups.
//
// The empty case is the one that matters. It is not "a cluster with no node groups" — every
// cluster has at least the master group — it is "the groups have not been read yet", which
// happens on every module start before the informer syncs. Answering it `true` would open the
// handover on a bashible cluster for the first seconds of every restart.
func TestEngineOnly(t *testing.T) {
	cases := []struct {
		name        string
		systemTypes []string
		want        bool
	}{{
		name:        "no groups read yet",
		systemTypes: nil,
		want:        false,
	}, {
		name:        "every group is DeckhouseEngine",
		systemTypes: []string{"DeckhouseEngine", "DeckhouseEngine"},
		want:        true,
	}, {
		name:        "every group is Engine, in either spelling",
		systemTypes: []string{"DeckhouseEngine", "Immutable"},
		want:        true,
	}, {
		// A group predating the field carries no systemType, and that absence means bashible.
		name:        "one group predating the field",
		systemTypes: []string{"DeckhouseEngine", ""},
		want:        false,
	}, {
		name:        "a mixed cluster mid-migration",
		systemTypes: []string{"DeckhouseEngine", "GenericOS"},
		want:        false,
	}, {
		name:        "a mixed cluster mid-migration, in the legacy spelling",
		systemTypes: []string{"Immutable", "Mutable"},
		want:        false,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, engineOnly(test.systemTypes))
		})
	}
}

// testPassword is the registry password in the fixtures below, from the allowlisted test values.
const testPassword = "dh-test-secret-0123456789abcdef"

// v1ProxyNodeConfig is `registry-bashible-config` as the v1 implementation wrote it in `Proxy` mode:
// rendered by Helm's toYaml, so with sorted keys, and with the credentials of the node services in it.
const v1ProxyNodeConfig = `hosts:
  registry.d8-system.svc:5001:
    mirrors:
    - auth:
        password: ` + testPassword + `
        username: username
      host: 127.0.0.1:5001
      scheme: https
imagesBase: registry.d8-system.svc:5001/system/deckhouse
mode: Proxy
proxyEndpoints:
- 192.168.199.10:5001
version: 0123456789abcdef
`

// installersBundleNodeConfig and installersDirectNodeConfig are the shape of what the installer writes
// for an installation from a bundle and for one with no registry settings. TestTheGateAdmitsEveryNewInstallation
// builds the real ones; these keep the filter's own cases readable.
const (
	installersBundleNodeConfig = `hosts:
  registry.d8-system.svc:5001:
    mirrors:
    - host: ${discovered_node_ip}:5001
      scheme: https
imagesBase: registry.d8-system.svc:5001/system/deckhouse
mode: Local
proxyEndpoints:
- ${discovered_node_ip}:5001
version: 0123456789abcdef
`
	installersDirectNodeConfig = `hosts:
  registry.d8-system.svc:5001:
    mirrors:
    - host: registry.deckhouse.io
      scheme: https
imagesBase: registry.d8-system.svc:5001/system/deckhouse
mode: Direct
version: 0123456789abcdef
`
)

func secretObject(t *testing.T, name string, data map[string][]byte) *unstructured.Unstructured {
	t.Helper()

	secret := &v1core.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "d8-system"},
		Data:       data,
	}
	object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: object}
}

// newSwitchInput is a hook input carrying the given snapshots and empty values.
func newSwitchInput(t *testing.T, snapshots map[string]any) (*go_hook.HookInput, *metrics.MemoryMetricsCollector) {
	t.Helper()
	return newHookInput(t, snapshots, map[string]any{"internal": map[string]any{}})
}

// newHookInput builds what addon-operator hands a Go hook: one snapshot per binding, as a filter
// result, and the module's values under `registry`.
func newHookInput(
	t *testing.T,
	snapshots map[string]any,
	registry map[string]any,
) (*go_hook.HookInput, *metrics.MemoryMetricsCollector) {
	t.Helper()

	encoded, err := json.Marshal(map[string]any{"registry": registry})
	require.NoError(t, err)
	var values map[string]any
	require.NoError(t, json.Unmarshal(encoded, &values))

	patchable, err := sdkpatchablevalues.NewPatchableValues(values)
	require.NoError(t, err)

	wrapped := go_hook.Snapshots{}
	for name, result := range snapshots {
		wrapped[name] = []sdkpkg.Snapshot{&go_hook.Wrapped{Wrapped: result}}
	}

	collector := metrics.NewCollector("")
	return &go_hook.HookInput{
		Snapshots:        wrapped,
		Values:           patchable,
		MetricsCollector: collector,
		PatchCollector:   object_patch.NewPatchCollector(),
		Logger:           log.NewNop(),
	}, collector
}

// valuesAfter is what the hook wrote to its values: the last write wins, as it does when addon-operator
// applies the patches.
func valuesAfter(t *testing.T, input *go_hook.HookInput) Values {
	t.Helper()

	path := "/" + strings.ReplaceAll(valuesPath, ".", "/")
	var current Values
	found := false
	for _, op := range input.Values.GetPatches() {
		if op.Path != path {
			continue
		}
		current = Values{}
		require.NoError(t, json.Unmarshal(op.Value, &current))
		found = true
	}
	require.True(t, found, "the hook wrote no values")
	return current
}
