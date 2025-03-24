/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/system-registry/playground",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "value",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-playground"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var cm v1core.ConfigMap

				err := sdk.FromUnstructured(obj, &cm)
				if err != nil {
					return "", fmt.Errorf("failed to convert state secret to struct: %v", err)
				}

				valueStr := cm.Data["value"]

				value, err := strconv.Atoi(valueStr)
				if err != nil || value < 1 {
					value = 3
				}

				return value, nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {
	valueSnaps := input.Snapshots["value"]

	value := 3
	if len(valueSnaps) == 1 {
		value = valueSnaps[0].(int)
	}

	model := make(map[string]int, value)

	for i := range value {
		k := fmt.Sprintf("field_%v", i+1)
		model[k] = i
	}

	input.Values.Set("systemRegistry.internal.playground.value", value)
	input.Values.Set("systemRegistry.internal.playground.data", value)

	return nil
})
