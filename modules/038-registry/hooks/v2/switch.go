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

	// systemTypeImmutable is the NodeGroup whose nodes an on-node agent configures from a
	// NodeConfig. bashible never runs on one, which is the whole of what this gate needs to
	// know about it. Spelled out rather than imported from node-manager for the reason
	// `modeDirect` is: it is a string read out of another module's resource, and the field
	// names are the contract.
	systemTypeImmutable = "Immutable"

	blockedMetric      = "d8_registry_migration_pending"
	blockedMetricGroup = "d8_registry_migration"

	// modeDirect is spelled out rather than imported from the legacy package for the same reason
	// legacyState is a local copy: the two implementations share no code, and this is a string
	// read out of a Secret the other one writes.
	modeDirect = "Direct"
)

// legacyState is the part of the legacy state machine this gate reads.
//
// A local, two-field copy rather than an import of the legacy package. The two
// implementations deliberately share no code, so that removing the old one is a single
// reviewable commit; a gate that reached into its types would be the one thread left
// holding them together. The field names are the contract, and a change to them shows
// up here as a mode that reads empty — which fails the gate closed.
type legacyState struct {
	Mode       string `json:"mode,omitempty"`
	TargetMode string `json:"target_mode,omitempty"`
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

func filterLegacyState(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("converting the secret: %w", err)
	}

	raw := secret.Data["state"]
	if len(raw) == 0 {
		return nil, nil
	}

	var state legacyState
	if err := yaml.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("decoding the legacy registry state: %w", err)
	}
	return state, nil
}

func handleSwitch(_ context.Context, input *go_hook.HookInput) error {
	enabled, reason := readGate(input).decide()

	input.MetricsCollector.Expire(blockedMetricGroup)
	if reason != "" {
		input.Logger.Info("the cluster is still managed by the previous registry implementation",
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

	// ModuleConfigured reports that the operator has written this module's own configuration:
	// `mode: Managed` together with a source of images. It is what makes taking over from
	// `Direct` safe, and it is deliberately not consulted for any other mode.
	ModuleConfigured bool

	// EngineOnly reports that every NodeGroup in the cluster names `systemType: Immutable`,
	// so no node here is configured by bashible.
	//
	// It settles the handover on its own, because the legacy implementation has exactly one
	// way to reach a node and that is a bashible step. On such a cluster it has configured
	// nothing, owns no pull path and has nothing to let go of — whatever its state secret
	// says about itself describes objects in the cluster, not writers on the nodes.
	//
	// Without this the gate is not merely cautious here, it is stuck: every refusal below
	// tells the operator to bring the legacy implementation to `Unmanaged` first, and the
	// transitions that would do that are themselves bashible steps. So a `Direct` or `Proxy`
	// state recorded on a cluster whose nodes are all Immutable never clears, the switch
	// never happens, and with it the module never renders its controller — which is what
	// compiles a RegistryUpstream into anything at all.
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
// the previous implementation runs this one; a cluster that has runs the previous one
// until it has let go of the pull path, and then this one takes over on its own. What is
// deliberately impossible is for an operator to select the previous implementation on a
// new cluster, or for both to be active at once.
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
		return false, fmt.Sprintf("the state of the legacy implementation cannot be read: %s",
			g.LegacyUnreadable.Error())
	}

	if g.Legacy == nil {
		// The legacy implementation has never recorded a state, so it owns nothing and
		// there is nothing to hand over. This is a cluster installed with the
		// controller-based implementation from the start.
		return true, ""
	}

	// Asked before the modes below, and that order is load-bearing: a cluster on its way somewhere
	// is being reconfigured by the legacy implementation right now, and no amount of configuration
	// on this side makes it safe to take its nodes over mid-move. `Unmanaged` heading to
	// `Unmanaged` is not a move.
	if g.Legacy.TargetMode != "" && g.Legacy.TargetMode != g.Legacy.Mode &&
		g.Legacy.TargetMode != string(registry_const.ModeUnmanaged) {
		return false, fmt.Sprintf(
			"the legacy implementation is transitioning to %q; wait for it to settle",
			g.Legacy.TargetMode)
	}

	switch {
	case g.Legacy.Mode == "":
		return false, "the legacy implementation has not recorded a mode yet"

	// `Direct` is admitted when this module's configuration is already written, and refusing it
	// otherwise is not caution — it is the only safe answer. This release does not render the
	// legacy implementation's objects at all, so a `Direct` cluster arrives here having lost the
	// Service and the in-cluster proxy that served the address its nodes pull through. Staying
	// switched off would leave that address unserved; taking over serves it from the
	// configuration below, and the node agent takes the runtime configuration from the same
	// moment. With nothing configured there is nothing to serve it WITH, so the gate stays shut
	// and the operator is told which lever to pull.
	//
	// Widened for `Direct` only. `Proxy` and `Local` keep state this release cannot account for —
	// static pods with their own PKI on every node, and, for `Local`, the image store on the
	// master disks — and for them `Unmanaged` remains the way through.
	case g.Legacy.Mode == modeDirect && g.ModuleConfigured:

	case g.Legacy.Mode == modeDirect:
		return false, fmt.Sprintf(
			"the cluster is in the %q mode of the legacy implementation and this module has no "+
				"configuration of its own; write `mode: Managed` with `primary.upstream` in the "+
				"registry ModuleConfig — that is what this implementation will serve the "+
				"in-cluster address from — or bring `registry.mode` to %q instead",
			g.Legacy.Mode, registry_const.ModeUnmanaged)

	case g.Legacy.Mode != string(registry_const.ModeUnmanaged):
		return false, fmt.Sprintf(
			"the cluster is in the %q mode of the legacy implementation; bring it to %q first, "+
				"so that the two implementations never configure the same nodes at once",
			g.Legacy.Mode, registry_const.ModeUnmanaged)

	case g.Legacy.TargetMode != "" && g.Legacy.TargetMode != string(registry_const.ModeUnmanaged):
		// Mid-transition: the mode says Unmanaged, but the cluster is on its way
		// somewhere else and nodes are being reconfigured right now.
		return false, fmt.Sprintf(
			"the legacy implementation is transitioning to %q; wait for it to settle",
			g.Legacy.TargetMode)
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

	state, err := helpers.SnapshotToSingle[legacyState](input, legacyStateSnapName)
	switch {
	case err == nil:
		result.Legacy = &state
	case errors.Is(err, helpers.ErrNoSnapshot):
		// Nothing recorded, which is not a failure to read.
	default:
		result.LegacyUnreadable = err
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
		if systemType != systemTypeImmutable {
			return false
		}
	}
	return true
}
