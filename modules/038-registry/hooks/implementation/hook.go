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
)

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
		},
	},
	handleImplementation,
)

func filterLegacyState(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	raw, found, err := unstructured.NestedString(obj.Object, "data", "state")
	if err != nil || !found {
		// An unreadable or absent state is reported as "present but empty", which `decide`
		// refuses: guessing "probably fine" would let a cluster upgrade into an unserved address.
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

func handleImplementation(_ context.Context, input *go_hook.HookInput) error {
	// Read through the module's own helper, which reports "no snapshot" as an error rather than a
	// zero value: "never recorded a state" and "recorded one this code could not read" are the
	// difference between admitting a cluster and refusing it.
	var legacy *legacyState
	if state, err := helpers.SnapshotToSingle[legacyState](input, legacyStateSnapName); err == nil {
		legacy = &state
	} else if !errors.Is(err, helpers.ErrNoSnapshot) {
		// Present but unreadable: treated as a state with no mode, which `decide` refuses.
		legacy = &legacyState{}
	}

	configured := false
	if value, err := helpers.SnapshotToSingle[bool](input, moduleConfigSnapName); err == nil {
		configured = value
	}

	value := decide(legacy, configured)
	requirements.SaveValue(registry_requirements.ImplementationKey, value)

	input.Logger.Info("recorded whether this cluster may take a release without the previous registry implementation",
		"value", value, "moduleConfigured", configured)

	return nil
}
