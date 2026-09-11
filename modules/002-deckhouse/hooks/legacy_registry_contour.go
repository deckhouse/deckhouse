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

// Whether this cluster still runs the previous implementation of the registry module.
//
// One fact, published for the templates: the registry module records its handover in
// `d8-system/registry-v2-switch`, and that record is sticky, so its presence answers "is the legacy
// registry contour still read by anything" — which decides whether this module renders it.
//
// The contour is `d8-system/registry-config`, rendered here out of `deckhouse.registry.*` — the
// previous implementation's settings in `mc/deckhouse`. Design ADR decision 22 asks for it to go: the
// configuration moves to `mc/registry` and the address becomes a constant. On a cluster that has
// switched the secret is not merely redundant but WRONG, since nothing writes those settings any
// more, and dhctl reads it to find the registry reachable from outside. Stale registry data preferred
// by the tooling is worse than none: absence falls back to something that works, staleness dials the
// wrong registry with the wrong account.
//
// The record and not the mode, because a cluster can run this implementation and manage nothing
// (`Unmanaged`), where the contour is just as stale.
//
// Order this depends on: dhctl reads the upstream from the `RegistryConfig` resource before falling
// back to this secret, and that came first on purpose.
package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// v2SwitchSecretName is the registry module's record that the handover happened.
	//
	// Spelled out rather than imported: this module shares no code with that one, and a rename shows up
	// here as a cluster that keeps rendering the legacy contour — which is the safe direction.
	v2SwitchSecretName = "registry-v2-switch"

	v2SwitchSnapName   = "registry-v2-switch"
	v2ActiveValuesPath = "deckhouse.internal.registry.v2Active"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/legacy-registry-contour",
	// Before the render that reads what this publishes.
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       v2SwitchSnapName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{v2SwitchSecretName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				// Only that it is there. What is inside it is the registry module's business.
				return obj.GetName(), nil
			},
		},
	},
}, handleLegacyRegistryContour)

func handleLegacyRegistryContour(_ context.Context, input *go_hook.HookInput) error {
	switched := len(input.Snapshots.Get(v2SwitchSnapName)) > 0

	input.Values.Set(v2ActiveValuesPath, switched)

	if switched {
		input.Logger.Info("the registry module runs its current implementation, so the legacy registry contour is not rendered",
			"secret", "d8-system/registry-config")
	}

	return nil
}
