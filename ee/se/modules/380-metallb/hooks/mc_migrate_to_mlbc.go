/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/metallb/node-labels-update",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "metallb_module_config",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"metallb"},
			},
			FilterFunc: applyModuleConfigFilter,
		},
	},
}, migrateMCtoMLBC)

func applyModuleConfigFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &v1alpha1.ModuleConfig{}
	err := sdk.FromUnstructured(obj, mc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert metallb moduleconfig: %v", err)
	}

	if mc.Spec.Version == 1 {
		return mc, nil
	}
	return nil, nil
}

func migrateMCtoMLBC(input *go_hook.HookInput) error {
	snapsMC := input.Snapshots["metallb_module_config"]
	if len(snapsMC) == 1 && snapsMC[0] != nil {
		mc, ok := snapsMC[0].(*v1alpha1.ModuleConfig)
		if !ok {
			return nil
		}

		var addressPools []interface{}
		if addressPoolsRaw, ok := mc.Spec.Settings["addressPools"]; ok {
			addressPools = addressPoolsRaw.([]interface{})
		}

		addressesSlice := make([]string, 0, 8)
		for _, addressPool := range addressPools {
			protocolRaw, ok := addressPool.(map[string]interface{})["protocol"]
			if !ok || protocolRaw.(string) != "layer2" {
				continue
			}
			addressesRaw, ok := addressPool.(map[string]interface{})["addresses"]
			if !ok {
				continue
			}
			for _, addr := range addressesRaw.([]interface{}) {
				addressesSlice = append(addressesSlice, addr.(string))
			}
		}

		nodeSelector := make((map[string]interface{}), 8)
		tolerations := make([]interface{}, 0, 8)
		if speakerRaw, ok := mc.Spec.Settings["speaker"]; ok {
			speaker := speakerRaw.(map[string]interface{})
			if nodeSelectorRaw, ok := speaker["nodeSelector"]; ok {
				nodeSelector = nodeSelectorRaw.(map[string]interface{})
			}
			if tolerationsRaw, ok := speaker["tolerations"]; ok {
				tolerations = tolerationsRaw.([]interface{})
			}
		}

		mlbc := map[string]interface{}{
			"apiVersion": "network.deckhouse.io/v1alpha1",
			"kind":       "MetalLoadBalancerClass",
			"metadata": map[string]interface{}{
				"name": "default",
			},
			"spec": map[string]interface{}{
				"isDefault":    true,
				"type":         "L2",
				"addressPool":  addressesSlice,
				"nodeSelector": nodeSelector,
				"tolerations":  tolerations,
			},
		}
		mlbcUnstructured, err := sdk.ToUnstructured(&mlbc)
		if err != nil {
			return nil
		}
		input.PatchCollector.Create(mlbcUnstructured, object_patch.IgnoreIfExists())
	}
	return nil
}
