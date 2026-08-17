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

	blockedMetric      = "d8_registry_migration_pending"
	blockedMetricGroup = "d8_registry_migration"
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
		},
	},
	handleSwitch,
)

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

	switch {
	case g.Legacy.Mode == "":
		return false, "the legacy implementation has not recorded a mode yet"

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

	state, err := helpers.SnapshotToSingle[legacyState](input, legacyStateSnapName)
	switch {
	case err == nil:
		result.Legacy = &state
	case errors.Is(err, helpers.ErrNoSnapshot):
		// Nothing recorded, which is not a failure to read.
	default:
		result.LegacyUnreadable = err
	}

	return result
}
