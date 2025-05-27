/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registryservice

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
)

func KubernetsConfig(name string) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:              name,
		ApiVersion:        "v1",
		Kind:              "Service",
		NamespaceSelector: helpers.NamespaceSelector,
		NameSelector: &types.NameSelector{
			MatchNames: []string{
				"registry",
			},
		},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var service v1core.Service

			err := sdk.FromUnstructured(obj, &service)
			if err != nil {
				return nil, fmt.Errorf("failed to convert secret \"%v\" to struct: %v", obj.GetName(), err)
			}

			var ret Inputs = true
			return ret, nil
		},
	}
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	return helpers.SnapshotToSingle[Inputs](input, name)
}
