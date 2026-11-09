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
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	bashible_model "github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
	registry_requirements "github.com/deckhouse/deckhouse/modules/038-registry/requirements"
)

const (
	// ImplementationLegacy and ImplementationV2 name the two implementations for the
	// release requirement and for the log. Not a user-facing choice: a cluster runs the
	// current implementation unless the previous one is still managing it.
	ImplementationLegacy = "Legacy"
	ImplementationV2     = "V2"

	// SwitchSecretName records that the switch to the controller-based implementation
	// has already happened.
	//
	// Needed because the handover below asks a question about the state of the cluster
	// at one moment, and that state does not stay true afterwards: the legacy state
	// secret is left behind, and the answer it gives has no bearing on a cluster that
	// has already moved. Without this marker, a module restart would re-ask a question
	// whose answer has expired and could switch the cluster back under itself.
	SwitchSecretName = "registry-v2-switch"

	// LegacyStateSecretName is where the legacy implementation persists its state
	// machine.
	LegacyStateSecretName = "registry-state"

	switchSnapName      = "v2-switch"
	legacyStateSnapName = "legacy-state"
	nodeGroupSnapName   = "node-groups"

	// legacyNodeConfigSnapName is `registry-bashible-config` as the gate reads it: for what the v1
	// implementation left in it, not for its existence. Its own name rather than the one hook.go
	// watches the same secret under, because snapshots belong to the hook that asked for them.
	legacyNodeConfigSnapName = "legacy-node-config"

	// systemTypeDeckhouseEngine is the NodeGroup whose nodes an on-node agent configures from a
	// NodeConfig. bashible never runs on one, which is the whole of what this gate needs to
	// know about it. Spelled out rather than imported from node-manager for the reason
	// `modeDirect` is: it is a string read out of another module's resource, and the field
	// names are the contract.
	systemTypeDeckhouseEngine = "DeckhouseEngine"
	legacySystemTypeImmutable = "Immutable"

	blockedMetric      = "d8_registry_migration_pending"
	blockedMetricGroup = "d8_registry_migration"

	// modeDirect is spelled out rather than imported from the legacy package for the same reason
	// legacyState is a local copy: the two implementations share no code, and this is a string
	// read out of a Secret the other one writes.
	modeDirect = "Direct"

	// modeProxy and modeLocal are the v1 modes that run node services of their own, spelled out for
	// the reason modeDirect is.
	modeProxy = "Proxy"
	modeLocal = "Local"

	// unsupported ends every reason this release cannot act on.
	//
	// It names no way forward because there is none in this release: nothing here writes the v1
	// state any more, so a mode or a transition it recorded stays exactly as recorded, and the only
	// code that could move it is a release this cluster has already left. Such a state is reachable
	// only by an update that bypassed the `registryImplementation` requirement of the release before
	// this one, which is why the reason says so.
	//
	// What it does name is what must not be touched: the v1 objects that remain are what keeps this
	// gate shut, and opening it would deploy this implementation next to whatever the v1 one left.
	unsupported = "this state is not supported and is reachable only by an update that bypassed the " +
		"registryImplementation release requirement; do not change the registry and deckhouse " +
		"ModuleConfigs, do not delete the registry-* secrets in d8-system, and contact Deckhouse " +
		"Platform technical support"
)

// legacyState is the part of the legacy state machine this gate reads.
//
// A local, two-field copy, because the legacy implementation that owned the type is no
// longer in this release. The secret it wrote outlives it, and the field names are the
// contract: a change to them shows up here as a mode that reads empty — which fails the
// gate closed.
type legacyState struct {
	Mode       string `json:"mode,omitempty"`
	TargetMode string `json:"target_mode,omitempty"`
}

// legacyStateSnapshot is what the filter of `registry-state` hands the gate: the state, or why it
// could not be read.
//
// A value in both cases rather than a filter error, because a filter error is not a refusal. On the
// first list it fails the informer, and with it the start of this module, which is critical; on a
// later event the event is dropped and the snapshot keeps the previous answer. Neither is the answer
// the gate exists to give, which is "refused, and here is why". Likewise never nil: a nil result is
// dropped from the snapshot, and an absent state is the one answer that opens the gate.
type legacyStateSnapshot struct {
	State legacyState `json:"state"`

	// Unreadable is why the state could not be decoded, empty when it could.
	Unreadable string `json:"unreadable,omitempty"`
}

// legacyNodeConfig is what the v1 implementation left in `registry-bashible-config`, as far as the
// gate needs it.
//
// The gate's second witness, for when `registry-state` is gone. That secret has no `keep` policy,
// so the very release that refuses the handover deletes it, and the gate's next pass would find no
// state at all and open. `registry-bashible-config` does carry `keep` on every release that wrote it
// (1.75 to 1.77), and this release renders it only once the gate is open, so it outlives the update
// with what the v1 implementation last told the nodes.
//
// Three facts, never the document itself: the same document holds the registry credentials of every
// host the nodes pull from, and none of that may reach a snapshot, the log or a metric label.
type legacyNodeConfig struct {
	// Mode is the mode the v1 implementation was configuring the nodes for. Its target, not where it
	// stood: during a transition the document already describes the destination.
	Mode string `json:"mode,omitempty"`

	// ProxyEndpoints reports that the nodes were told about node services of a `Proxy` or `Local`
	// mode. The v1 implementation lists them while either end of a transition is one of those modes,
	// so this is what tells a move out of `Proxy` from a cluster that never had it.
	ProxyEndpoints bool `json:"proxyEndpoints,omitempty"`

	// Unreadable reports that the document could not be decoded.
	Unreadable bool `json:"unreadable,omitempty"`
}

// legacy turns the node configuration into the state the v1 implementation would have recorded,
// so that the gate asks one set of questions of both witnesses.
//
// A proxy endpoint under any mode other than `Proxy` and `Local` is a cluster moving out of one of
// those, and reads as that transition. It is not admitted on its target: the node services of the
// mode it is leaving may still be on the masters.
func (c legacyNodeConfig) legacy() legacyState {
	if c.ProxyEndpoints && c.Mode != modeProxy && c.Mode != modeLocal {
		return legacyState{Mode: modeProxy, TargetMode: c.Mode}
	}
	return legacyState{Mode: c.Mode}
}

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		// Ahead of the hook that builds the rest of the internal values, because
		// everything it produces is conditional on the answer here.
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
		Queue:        "/modules/registry/v2",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       switchSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{SwitchSecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: filterSwitchSecret,
			},
			{
				Name:       legacyStateSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{LegacyStateSecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: filterLegacyState,
			},
			{
				Name:       legacyNodeConfigSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{BashibleConfigSecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: filterLegacyNodeConfig,
			},
			{
				// This module's own ModuleConfig, for the one question the `Direct` case asks:
				// has the operator said where images come from. Watched rather than read once,
				// because writing that configuration is exactly the event that makes a `Direct`
				// cluster takeable — it should not have to wait for anything else to happen.
				Name:       moduleConfigSnapName,
				ApiVersion: "deckhouse.io/v1alpha1",
				Kind:       "ModuleConfig",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"registry"},
				},
				FilterFunc: filterModuleConfig,
			},
			{
				// Every NodeGroup, for the one question that can retire the legacy state
				// entirely: is there a node in this cluster bashible could still be
				// configuring. See gate.EngineOnly.
				Name:       nodeGroupSnapName,
				ApiVersion: "deckhouse.io/v1",
				Kind:       "NodeGroup",
				FilterFunc: filterNodeGroupSystemType,
			},
		},
	},
	handleSwitch,
)

// filterNodeGroupSystemType keeps one field of a NodeGroup: what configures its nodes.
// Absent on every group predating the field, and that absence is the answer "bashible" —
// which is why it is carried as the empty string rather than dropped.
func filterNodeGroupSystemType(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	systemType, _, err := unstructured.NestedString(obj.Object, "spec", "systemType")
	if err != nil {
		return nil, fmt.Errorf("reading spec.systemType of NodeGroup %q: %w", obj.GetName(), err)
	}
	return systemType, nil
}

func filterSwitchSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// Only its existence matters. Nothing reads what is inside it, and giving it
	// contents would invite something to start depending on them.
	return obj.GetName(), nil
}

// filterLegacyState reads `registry-state`, and never fails: see legacyStateSnapshot.
//
// A secret with no state in it reads as a state that recorded no mode, which the gate refuses. Only
// the v1 implementation ever wrote this secret, so its presence alone says that implementation was
// here; what is missing from it is no reason to assume it has let go.
func filterLegacyState(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return legacyStateSnapshot{Unreadable: fmt.Sprintf("converting the secret: %s", err)}, nil
	}

	raw := secret.Data["state"]
	if len(raw) == 0 {
		return legacyStateSnapshot{}, nil
	}

	var state legacyState
	if err := yaml.Unmarshal(raw, &state); err != nil {
		return legacyStateSnapshot{Unreadable: fmt.Sprintf("decoding the state: %s", err)}, nil
	}
	return legacyStateSnapshot{State: state}, nil
}

// filterLegacyNodeConfig reads what the v1 implementation left in `registry-bashible-config`.
//
// Nothing when the document is not the v1 implementation's, and two things say so. It names the
// node agent: that is this implementation's own document, or the installer's for a cluster this
// module manages, and no v1 writer ever set it. Or it is still exactly the document the installer of
// this release wrote and marked, see bashible_model.InstallerMarkAnnotation. The installer writes
// `mode: Direct` with no agent for an installation with no registry settings, and `mode: Local` with
// the node services' endpoints for an installation from a bundle, and by their contents those are a
// v1 `Direct` and a v1 `Local` cluster. Without the mark the gate refused both: the first recorded as
// running the v1 implementation, the second never given the store it waits for.
//
// Everything else is the v1 implementation's, and a document that cannot be decoded is reported as
// such rather than dropped, for the same reason as in filterLegacyState.
//
// Decoded into a local three-field struct, so the credentials in the same document are never held:
// see legacyNodeConfig.
func filterLegacyNodeConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return legacyNodeConfig{Unreadable: true}, nil
	}

	raw := secret.Data["config"]
	if len(raw) == 0 {
		return legacyNodeConfig{Unreadable: true}, nil
	}

	if bashible_model.WrittenByInstaller(secret.Annotations, raw) {
		return nil, nil
	}

	var document struct {
		Agent          *struct{} `json:"agent,omitempty"`
		Mode           string    `json:"mode,omitempty"`
		ProxyEndpoints []string  `json:"proxyEndpoints,omitempty"`
	}
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return legacyNodeConfig{Unreadable: true}, nil
	}

	if document.Agent != nil {
		return nil, nil
	}

	return legacyNodeConfig{
		Mode:           document.Mode,
		ProxyEndpoints: len(document.ProxyEndpoints) > 0,
	}, nil
}

func handleSwitch(_ context.Context, input *go_hook.HookInput) error {
	enabled, reason := readGate(input).decide()

	// Expired and set again on every pass, so the series lives exactly as long as the refusal:
	// D8RegistryMigrationPending waits five minutes before firing, and it can only fire on a reason
	// that is still given by then. That is what the second witness is for as much as the gate is —
	// without it the refusal, and the series, ended on the pass after the update deleted
	// `registry-state`, seconds after it began.
	input.MetricsCollector.Expire(blockedMetricGroup)
	if reason != "" {
		input.Logger.Info("the cluster is still managed by the v1 registry implementation",
			"reason", reason)
		input.MetricsCollector.Set(blockedMetric, 1,
			map[string]string{"reason": reason},
			metrics.WithGroup(blockedMetricGroup))
	}

	recordImplementation(enabled)

	values := accessor(input)
	current := values.Get()
	current.Enabled = enabled
	current.SwitchBlockedReason = reason
	values.Set(current)

	return nil
}

// recordImplementation publishes which implementation the cluster is running, for the
// release requirement in modules/038-registry/requirements to compare a release against.
// Without it, the release that finally removes the previous implementation could install
// on a cluster still running it, and that cluster would lose the code configuring the
// container runtime on its nodes.
//
// Its own function so that it can be tested, which matters here more than it looks: the
// failure mode of not recording is silence. An absent value is deliberately treated as "no
// information" and lets every release through, so a recording that quietly stopped
// happening would leave the gate open and nothing would say so.
func recordImplementation(enabled bool) {
	if enabled {
		requirements.SaveValue(registry_requirements.ImplementationKey, ImplementationV2)
		return
	}

	requirements.SaveValue(registry_requirements.ImplementationKey, ImplementationLegacy)
}

// gate is what the decision is made from: whether the handover has already happened,
// and what the previous implementation says about itself.
type gate struct {
	// AlreadySwitched reports that the marker secret exists.
	AlreadySwitched bool

	// Legacy is the legacy state, or nil when it has never been recorded.
	Legacy *legacyState

	// LegacyUnreadable is why the legacy state could not be read, when it exists but
	// could not be decoded.
	LegacyUnreadable error

	// NodeConfig is what the v1 implementation left in `registry-bashible-config`, or nil when
	// that secret is absent, is this implementation's own, or is the installer's as it wrote it.
	// Asked in addition to Legacy, not instead of it: either one refusing refuses. See
	// legacyNodeConfig for why it outlives the state.
	NodeConfig *legacyNodeConfig

	// ModuleConfigured reports that the operator has written this module's own configuration:
	// `mode: Managed` together with a source of images. It is what makes taking over from
	// `Direct` safe, and it is deliberately not consulted for any other mode.
	ModuleConfigured bool

	// EngineOnly reports that every NodeGroup in the cluster names `systemType: DeckhouseEngine`,
	// so no node here is configured by bashible.
	//
	// It settles the handover on its own, because the legacy implementation has exactly one
	// way to reach a node and that is a bashible step. On such a cluster it has configured
	// nothing, owns no pull path and has nothing to let go of — whatever its state secret
	// says about itself describes objects in the cluster, not writers on the nodes.
	//
	// Without this the gate is not merely cautious here, it is stuck: a state recorded by the
	// legacy implementation could only ever be moved by its own transitions, and those are
	// bashible steps. So a `Direct` or `Proxy` state recorded on a cluster whose nodes are
	// all DeckhouseEngine never clears, the switch never happens, and with it the module never
	// renders its controller — which is what compiles a RegistryUpstream into anything at all.
	//
	// Deliberately all rather than any. A cluster holding one bashible group still has nodes
	// the legacy implementation can write to, and there the handover is the real thing.
	//
	// Asked once in practice: the switch it allows writes the marker secret AlreadySwitched
	// reads, so a bashible group added later finds the question already answered — which is
	// the wanted answer, since by then this implementation is the one configuring it.
	EngineOnly bool
}

// decide answers whether the current implementation is active, and if not, why.
//
// There is no choice to make here, and that is the point. A cluster that has never run
// the previous implementation runs this one; a cluster that has is taken over only from a
// state that leaves nothing to race — `Unmanaged`, or `Direct` once this module has its
// own configuration — and every other state is refused, with the reason. What is
// deliberately impossible is for an operator to select the previous implementation on a
// new cluster, or for both to be active at once.
//
// Fails closed on every v1 trace it has: the state the v1 implementation recorded and the
// node configuration it left. Both have to say it has let go. The update to this release
// deletes the state, so a gate that read only the state refused once and then opened on its
// own a few seconds later, deploying this implementation next to whatever the v1 one left.
//
// A pure function of its inputs, because it is the decision that could put two writers on
// every node in the cluster. The two implementations configure the same thing — which
// registry the container runtime asks, and with which credentials — and enabling both
// would not merge those answers, it would race them.
func (g gate) decide() (bool, string) {
	if g.AlreadySwitched {
		// The question this gate asks is about a moment that has passed.
		return true, ""
	}

	if g.EngineOnly {
		// No node here has a bashible writer, so there is no second writer to wait for —
		// including when the state below cannot be read at all, which on any other cluster
		// is the one case worth refusing over.
		return true, ""
	}

	if g.LegacyUnreadable != nil {
		// Refused rather than assumed. An unreadable state is exactly the case where
		// guessing "it is probably fine" would enable a second writer on every node.
		return false, fmt.Sprintf("the state of the v1 implementation cannot be read: %s; %s",
			g.LegacyUnreadable.Error(), unsupported)
	}

	if g.Legacy != nil {
		if enabled, reason := g.decideState(*g.Legacy); !enabled {
			return false, reason
		}
	}

	if g.NodeConfig != nil {
		if g.NodeConfig.Unreadable {
			return false, "the node configuration the v1 implementation left in registry-bashible-config " +
				"cannot be read; " + unsupported
		}
		if enabled, reason := g.decideState(g.NodeConfig.legacy()); !enabled {
			return false, reason
		}
	}

	// Either the v1 implementation has never recorded anything, so it owns nothing and there is
	// nothing to hand over — a cluster installed with the controller-based implementation from the
	// start — or everything it left says it has let go.
	return true, ""
}

// decideState answers for one account of what the v1 implementation is doing: the state it
// recorded, or the one its node configuration implies.
func (g gate) decideState(state legacyState) (bool, string) {
	// Asked before the modes below, and that order is load-bearing: a cluster on its way somewhere
	// is being reconfigured by the legacy implementation right now, and no amount of configuration
	// on this side makes it safe to take its nodes over mid-move. `Unmanaged` heading to
	// `Unmanaged` is not a move.
	if state.TargetMode != "" && state.TargetMode != state.Mode &&
		state.TargetMode != string(registry_const.ModeUnmanaged) {
		return false, fmt.Sprintf("the v1 implementation is transitioning to %q; %s",
			state.TargetMode, unsupported)
	}

	switch {
	case state.Mode == "":
		return false, "the v1 implementation has not recorded a mode; " + unsupported

	// `Direct` is admitted when this module's configuration is already written, and refusing it
	// otherwise is the only safe answer. This release renders none of the v1 implementation's
	// objects. After an update from 1.77 its Service and in-cluster proxy are still there, kept by
	// their `keep` policy, but nothing in this release updates them; after a jump from an older
	// release they are gone. Taking over serves the in-cluster address from the configuration
	// below, the node agent takes the runtime configuration from the same moment, and the
	// controller removes the v1 objects once every node's agent answers. With nothing configured
	// there is nothing to serve the address WITH, so the gate stays shut — which also keeps the
	// node configuration the v1 implementation left, the one thing still pointing the nodes at the
	// registry — and the operator is told which lever to pull.
	//
	// Widened for `Direct` only. `Proxy` and `Local` keep state this release cannot account for —
	// static pods with their own PKI on the masters, and, for `Local`, the image store on their
	// disks — and running this implementation next to what is left of them is not supported.
	case state.Mode == modeDirect && g.ModuleConfigured:

	case state.Mode == modeDirect:
		return false, fmt.Sprintf(
			"the cluster is in the %q mode of the v1 implementation and this module has no "+
				"configuration of its own; write `mode: Managed` with `primary.upstream` in the "+
				"registry ModuleConfig — that is what this implementation will serve the "+
				"in-cluster address from",
			state.Mode)

	case state.Mode != string(registry_const.ModeUnmanaged):
		return false, fmt.Sprintf("the cluster is in the %q mode of the v1 implementation; %s",
			state.Mode, unsupported)
	}

	return true, ""
}

// readGate collects what the decision needs from the snapshots.
func readGate(input *go_hook.HookInput) gate {
	result := gate{}

	if _, err := helpers.SnapshotToSingle[string](input, switchSnapName); err == nil {
		result.AlreadySwitched = true
		return result
	}

	if groups, err := helpers.SnapshotToList[string](input, nodeGroupSnapName); err == nil {
		result.EngineOnly = engineOnly(groups)
	} else {
		// A group that could not be converted leaves the field false, which asks the handover
		// in full — the same answer a bashible cluster gets, and the safe one to be wrong with.
		input.Logger.Warn("cannot read the node groups; the registry handover is decided without them",
			"error", err.Error())
	}

	state, err := helpers.SnapshotToSingle[legacyStateSnapshot](input, legacyStateSnapName)
	switch {
	case err == nil && state.Unreadable != "":
		result.LegacyUnreadable = errors.New(state.Unreadable)
	case err == nil:
		result.Legacy = &state.State
	case errors.Is(err, helpers.ErrNoSnapshot):
		// Nothing recorded, which is not a failure to read.
	default:
		result.LegacyUnreadable = err
	}

	nodeConfig, err := helpers.SnapshotToSingle[legacyNodeConfig](input, legacyNodeConfigSnapName)
	switch {
	case err == nil:
		result.NodeConfig = &nodeConfig
	case errors.Is(err, helpers.ErrNoSnapshot):
		// Absent, or this implementation's or the installer's document, which the filter drops.
	default:
		result.NodeConfig = &legacyNodeConfig{Unreadable: true}
	}

	// Absent configuration reads as false, which is the answer that keeps the gate shut. There is
	// no third state to distinguish here: a ModuleConfig that cannot be read is a ModuleConfig
	// nothing can be served from either way.
	if facts, err := helpers.SnapshotToSingle[moduleConfigFacts](input, moduleConfigSnapName); err == nil {
		result.ModuleConfigured = facts.Actionable()
	}

	return result
}

// engineOnly answers whether bashible has a node to configure anywhere in this cluster.
//
// An empty list is not an answer: a cluster always has at least the master group, so having
// none means the groups have not been read rather than that there are none.
func engineOnly(systemTypes []string) bool {
	if len(systemTypes) == 0 {
		return false
	}
	for _, systemType := range systemTypes {
		if !isDeckhouseEngine(systemType) {
			return false
		}
	}
	return true
}

// isDeckhouseEngine mirrors IsDeckhouseEngine in
// modules/040-node-manager/images/node-controller/src/api/deckhouse.io/v1/nodegroup_types.go.
func isDeckhouseEngine(systemType string) bool {
	return systemType == systemTypeDeckhouseEngine || systemType == legacySystemTypeImmutable
}
