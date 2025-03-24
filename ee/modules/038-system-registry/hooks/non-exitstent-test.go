/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"gopkg.in/yaml.v3"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/nonexistent-test",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "value",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-nonexitent"},
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

				value := cm.Data["value"]
				return value, nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {
	valueSnaps := input.Snapshots["value"]

	if len(valueSnaps) == 1 {
		value := valueSnaps[0].(string)
		input.Values.Set("systemRegistry.internal.state.nonexistent_value", value)
	} else if len(valueSnaps) > 1 {
		buf, err := yaml.Marshal(valueSnaps)
		if err != nil {
			return fmt.Errorf("cannot marshal YAML value snaps: %w", err)
		}

		input.Values.Set("systemRegistry.internal.state.nonexistent_value", fmt.Sprintf("# [MULTIPLE]\n%s", buf))
	} else {
		input.Values.Remove("systemRegistry.internal.state.nonexistent_value")
	}

	return nil
})
