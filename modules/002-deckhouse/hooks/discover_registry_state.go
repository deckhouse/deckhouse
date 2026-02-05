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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	registryStateSnapName   = "registry-state"
	registryStateValuesPath = "deckhouse.internal.registry"
)

type registryState struct {
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty"`
}

func (state *registryState) fromSecret(secret corev1.Secret) {
	*state = registryState{
		Mode: string(secret.Data["mode"]),
	}
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/deckhouse/discover-registry-state",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       registryStateSnapName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-state"},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret corev1.Secret
				if err := sdk.FromUnstructured(obj, &secret); err != nil {
					return nil, fmt.Errorf("failed to convert secret %q: %w", obj.GetName(), err)
				}

				state := registryState{}
				state.fromSecret(secret)
				return state, nil
			},
		},
	},
}, func(ctx context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get(registryStateSnapName)
	if len(snaps) == 0 {
		input.Values.Set(registryStateValuesPath, registryState{})
		return nil
	}

	var state registryState
	if err := snaps[0].UnmarshalTo(&state); err != nil {
		return fmt.Errorf("failed to convert registry state snapshot: %w", err)
	}

	input.Values.Set(registryStateValuesPath, state)
	return nil
})
