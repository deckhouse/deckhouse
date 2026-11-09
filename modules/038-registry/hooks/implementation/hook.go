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

package implementation

import (
	"context"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
	registry_requirements "github.com/deckhouse/deckhouse/modules/038-registry/requirements"
)

const (
	legacyStateSnapName  = "legacy-state"
	moduleConfigSnapName = "module-config"
	desiredModeSnapName  = "desired-mode"

	// desiredModeUnreadable stands for a `registry.mode` in the `deckhouse` ModuleConfig that is not
	// a string. Any value `decide` does not know refuses, so this one does too.
	desiredModeUnreadable = "<unreadable>"
)

// The value starts as Legacy, before this hook has run even once, so that it is never absent while
// the module runs.
//
// Every start of the controller process — a restart, a new leader — begins with nothing recorded,
// and a release requiring V2 must not be judged on a cluster nobody has looked at yet. `check`
// refuses an absent value as well; the seed makes that state visible as what it means
// (`deckhouse-controller requirements` shows Legacy) instead of resting on a fallback.
//
// A cluster that is V2 does not notice it. A minor release is checked only once the controller
// reports ready, which is after the first converge of every enabled module, and this hook runs in
// that converge (OnBeforeHelm) and replaces the seed. What can still read the seed is a manual
// approval of such a release made during that first converge: it is refused and can be repeated
// once the controller is ready. With the module disabled the seed stays and nothing reads it, since
// the check of a disabled module is skipped.
func init() {
	recordUndecided()
}

func recordUndecided() {
	requirements.SaveValue(registry_requirements.ImplementationKey, ImplementationLegacy)
}

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue: "/modules/registry/implementation",
		// On every reconciliation rather than on a schedule: the answer changes the moment an
		// operator writes the module's configuration.
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
		Kubernetes: []go_hook.KubernetesConfig{
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
				Name:       moduleConfigSnapName,
				ApiVersion: "deckhouse.io/v1alpha1",
				Kind:       "ModuleConfig",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"registry"},
				},
				FilterFunc: filterModuleConfig,
			},
			{
				// The mode the operator has asked the previous implementation for. It changes before
				// the recorded state does, so a cluster asked to move to `Proxy` or `Local` is
				// refused from that moment rather than from the next render of module `deckhouse`.
				Name:       desiredModeSnapName,
				ApiVersion: "deckhouse.io/v1alpha1",
				Kind:       "ModuleConfig",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"deckhouse"},
				},
				FilterFunc: filterDesiredMode,
			},
		},
	},
	handleImplementation,
)

// filterLegacyState reads the previous implementation's recorded state, and answers every failure
// with a state that records no mode rather than with an error.
//
// The direction is the reason. A filter error never reaches this hook: on the watch path the
// platform logs it and drops the event (shell-operator, resource_informer.go `handleWatchEvent`), so
// the object simply never enters the snapshot — and an object missing from the snapshot is exactly
// what a cluster with no state at all looks like, which `decide` ADMITS. A Secret that exists and
// cannot be read has to be refused instead, and the only way to say that is to hand `decide` a state
// with no mode.
//
// `filterModuleConfig` below does return its error, because there the same dropping degrades the
// other way: a ModuleConfig missing from the snapshot reads as "nothing configured", which refuses.
func filterLegacyState(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	raw, found, err := unstructured.NestedString(obj.Object, "data", "state")
	if err != nil || !found {
		return legacyState{}, nil
	}

	decoded, err := decodeBase64(raw)
	if err != nil {
		return legacyState{}, nil
	}

	var state legacyState
	if err := yaml.Unmarshal(decoded, &state); err != nil {
		return legacyState{}, nil
	}
	return state, nil
}

// filterModuleConfig answers the one question the decision needs: has the operator written a
// configuration the next release can act on. Both halves are required — `mode: Managed` with no
// source of images is unservable, and `primary` under the default `Unmanaged` mode is settings
// nothing applies.
func filterModuleConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	settings, _, err := unstructured.NestedMap(obj.Object, "spec", "settings")
	if err != nil {
		return false, fmt.Errorf("reading the registry ModuleConfig settings: %w", err)
	}

	mode, _ := settings["mode"].(string)
	_, hasPrimary := settings["primary"]

	return mode == "Managed" && hasPrimary, nil
}

// filterDesiredMode reads `spec.settings.registry.mode` of the `deckhouse` ModuleConfig. A mode that
// is not set is `Unmanaged`, the schema default that module `deckhouse` renders for it: removing the
// setting from a `Direct` cluster starts the same switch as writing `Unmanaged`.
//
// It never returns an error, for the reason `filterLegacyState` gives: an object dropped from the
// snapshot reads as "nothing asked for", which admits. A mode that is not a string cannot pass the
// ModuleConfig schema, and is reported as a mode `decide` does not know, which refuses.
func filterDesiredMode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mode, found, err := unstructured.NestedString(obj.Object, "spec", "settings", "registry", "mode")
	switch {
	case err != nil:
		return desiredModeUnreadable, nil
	case !found || mode == "":
		return "Unmanaged", nil
	}
	return mode, nil
}

func handleImplementation(_ context.Context, input *go_hook.HookInput) error {
	// "The previous implementation never recorded a state" and "it recorded one this code cannot
	// read" are the difference between admitting a cluster and refusing it, so the two are told
	// apart here rather than collapsed into a zero value.
	var legacy *legacyState
	state, err := helpers.SnapshotToSingle[legacyState](input, legacyStateSnapName)
	switch {
	case err == nil:
		legacy = &state

	case errors.Is(err, helpers.ErrNoSnapshot):
		// No such object. The previous implementation never took this cluster, so an upgrade can
		// take nothing away from it — which is what `decide` reads a nil state as. Refusing here
		// instead would block every cluster that has never used the legacy modes, since the state
		// Secret is not rendered on those at all.
		legacy = nil

	default:
		// A snapshot the helper could not convert, treated as a state with no mode, which `decide`
		// refuses. A backstop rather than the usual path: `filterLegacyState` already normalises a
		// Secret it cannot read into exactly that, and reports no error for it.
		legacy = &legacyState{}
	}

	configured := false
	if value, err := helpers.SnapshotToSingle[bool](input, moduleConfigSnapName); err == nil {
		configured = value
	}

	// No ModuleConfig is nothing asked of the previous implementation. A snapshot the helper cannot
	// convert is a mode nobody can tell, which refuses.
	desiredMode := ""
	mode, err := helpers.SnapshotToSingle[string](input, desiredModeSnapName)
	switch {
	case err == nil:
		desiredMode = mode
	case !errors.Is(err, helpers.ErrNoSnapshot):
		desiredMode = desiredModeUnreadable
	}

	value := decide(legacy, desiredMode, configured)
	requirements.SaveValue(registry_requirements.ImplementationKey, value)

	input.Logger.Info("recorded whether this cluster may take a release without the previous registry implementation",
		"value", value, "desiredMode", desiredMode, "moduleConfigured", configured)

	return nil
}
