/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type affinityRule struct {
	Polarity      string `json:"polarity"`
	Required      bool   `json:"required"`
	NodeGroupName string `json:"nodeGroupName"`
}

type vcdInstanceClass struct {
	Spec struct {
		AffinityRule *affinityRule `json:"affinityRule"`
	} `json:"spec"`
	Status struct {
		NodeGroupConsumers []string `json:"nodeGroupConsumers"`
	} `json:"status"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 25},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "affinity_rules_from_vcdinstanceclass",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "VCDInstanceClass",
			FilterFunc: applyInstanceClassFilter,
		},
	},
}, handleAffinityRules)

func applyInstanceClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var instanceClass vcdInstanceClass

	err := sdk.FromUnstructured(obj, &instanceClass)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal VCDInstanceClass: %w", err)
	}

	affinityRule := instanceClass.Spec.AffinityRule
	if affinityRule != nil && len(instanceClass.Status.NodeGroupConsumers) > 0 {
		return instanceClass, nil
	}

	return nil, nil
}

func handleAffinityRules(_ context.Context, input *go_hook.HookInput) error {
	affinityRules := make([]affinityRule, 0)

	if masterAffinityRule, ok := input.Values.GetOk("cloudProviderVcd.internal.providerClusterConfiguration.masterNodeGroup.instanceClass.affinityRule"); ok {
		affinityRules = append(affinityRules, affinityRule{
			Polarity:      masterAffinityRule.Map()["polarity"].String(),
			Required:      masterAffinityRule.Map()["required"].Bool(),
			NodeGroupName: "master",
		})
	}

	if nodeGroups, ok := input.Values.GetOk("cloudProviderVcd.internal.providerClusterConfiguration.nodeGroups"); ok {
		for _, ng := range nodeGroups.Array() {
			ngMap := ng.Map()
			ngName := ngMap["name"].String()
			ngInstanceClass := ngMap["instanceClass"].Map()

			if ngAffinityRule, ok := ngInstanceClass["affinityRule"]; ok {
				affinityRules = append(affinityRules, affinityRule{
					Polarity:      ngAffinityRule.Map()["polarity"].String(),
					Required:      ngAffinityRule.Map()["required"].Bool(),
					NodeGroupName: ngName,
				})
			}
		}
	}

	vcdInstanceClasses, err := sdkobjectpatch.UnmarshalToStruct[vcdInstanceClass](input.Snapshots, "affinity_rules_from_vcdinstanceclass")
	if err != nil {
		return fmt.Errorf("failed to unmarshal affinity rules from VCDInstanceClasses: %w", err)
	}

	for _, instanceClass := range vcdInstanceClasses {
		for _, nodeGroup := range instanceClass.Status.NodeGroupConsumers {
			affinityRules = append(affinityRules, affinityRule{
				Polarity:      instanceClass.Spec.AffinityRule.Polarity,
				Required:      instanceClass.Spec.AffinityRule.Required,
				NodeGroupName: nodeGroup,
			})
		}
	}

	input.Values.Set("cloudProviderVcd.internal.affinityRules", affinityRules)
	return nil
}
